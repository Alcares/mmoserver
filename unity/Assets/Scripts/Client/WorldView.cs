using System.Collections.Generic;
using Game.Networking;
using Game.V1;
using UnityEngine;
using UnityEngine.InputSystem;

namespace Game.Client
{
    /// <summary>
    /// Renders the server's world: checkerboard floor, trading stations, and every
    /// player in the latest snapshot, with the camera following the local player.
    /// Server coordinates are in tiles with +y down; Unity world is (x, -y).
    /// </summary>
    [RequireComponent(typeof(GameClient), typeof(GameInput), typeof(GameState))]
    public class WorldView : MonoBehaviour
    {
        private const float CrewmateSize = 1.3f;
        private const float StationRadius = 0.9f;
        private const int BackgroundOrder = -100;
        private const int StationOrder = -50;
        private const int GoalOrder = -49;
        private const int LabelOrder = 10000;

        [Tooltip("Orthographic half-height the camera starts at, in tiles.")]
        [SerializeField] private float cameraSize = 12f;
        [Tooltip("Zoom limits, in tiles of orthographic half-height.")]
        [SerializeField] private float minZoom = 4f;
        [SerializeField] private float maxZoom = 60f;
        [Tooltip("Multiplier applied per wheel notch, so a notch covers the same proportion at any zoom.")]
        [SerializeField] private float zoomFactor = 1.15f;
        [Tooltip("Higher = snappier zoom. 0 snaps instantly.")]
        [SerializeField] private float zoomSmoothing = 12f;
        [Tooltip("Higher = snappier follow of the 20Hz server snapshots.")]
        [SerializeField] private float smoothing = 20f;
        [Tooltip("Optional 8-direction player art. Leave empty to use the built-in procedural crewmate.")]
        [SerializeField] private DirectionalAnimationSet playerArt;
        [Tooltip("Optional 8-direction art for server-run bots (PlayerState.is_bot). Falls back to playerArt.")]
        [SerializeField] private DirectionalAnimationSet botArt;
        [Tooltip("Optional station icons. A commodity without an icon is drawn as a plain disc.")]
        [SerializeField] private CommodityIcons commodityIcons;

        private sealed class PlayerView
        {
            public GameObject Go;
            public PlayerAvatar Avatar;
            public Vector2 Target;
            public Vector2 Velocity;
            public bool IsMe;
            /// <summary>Server-run. Animates from velocity and wears the bot art even as players[0].</summary>
            public bool IsBot;
        }

        private sealed class StationView
        {
            public CommodityType Commodity;
            public string Name;
            public OutlinedLabel Label;
            public GameObject Go;
        }

        private GameClient _client;
        private GameInput _input;
        private GameState _state;
        private Camera _camera;
        private Transform _cameraFollow;
        private float _zoomTarget;
        private SpriteRenderer _floor;

        private readonly Dictionary<uint, PlayerView> _players = new();
        private readonly HashSet<uint> _seen = new();
        private readonly List<uint> _stale = new();
        private readonly List<StationView> _stations = new();
        private GameObject _goal;

        private void Awake()
        {
            _client = GetComponent<GameClient>();
            _input = GetComponent<GameInput>();
            _state = GetComponent<GameState>();
            Application.runInBackground = true;

            SetupCamera();
            BuildBackground();
        }

        private void OnEnable()
        {
            _client.OnWorldSnapshot += HandleSnapshot;
            _client.OnInitialState += HandleInitialState;
            _client.OnGoalMarker += HandleGoalMarker;
            _state.PricesChanged += RefreshStationLabels;
        }

        private void OnDisable()
        {
            _client.OnWorldSnapshot -= HandleSnapshot;
            _client.OnInitialState -= HandleInitialState;
            _client.OnGoalMarker -= HandleGoalMarker;
            _state.PricesChanged -= RefreshStationLabels;
        }

        private void Update()
        {
            float follow = 1f - Mathf.Exp(-smoothing * Time.deltaTime);

            foreach (var view in _players.Values)
            {
                var pos = Vector2.Lerp(view.Go.transform.position, view.Target, follow);
                view.Go.transform.position = pos;

                // The local player animates from raw input so it reacts before the server echoes.
                // A bot never does, even as players[0] - which is what a spectator stream makes
                // it - because this keyboard is not what moves it.
                // Input is in server axes (+y down); avatars take a world direction (+y up).
                bool driven = view.IsMe && !view.IsBot;
                bool moving = driven ? _input.Move != Vector2.zero : view.Velocity.magnitude > 0.01f;
                Vector2 direction = driven ? new Vector2(_input.Move.x, -_input.Move.y) : view.Velocity;

                view.Avatar.Animate(direction, moving);
                view.Avatar.SetSortingOrder(-Mathf.RoundToInt(pos.y * 10f));

                if (view.IsMe) _cameraFollow = view.Go.transform;
            }
        }

        private void LateUpdate()
        {
            // Before the follow guard: zooming works in the lobby too, where the camera is
            // framed on the whole map and there is nobody to follow yet.
            ApplyZoom();

            if (_cameraFollow == null) return;
            var p = _cameraFollow.position;
            _camera.transform.position = new Vector3(p.x, p.y, -10f);
        }

        /// <summary>
        /// Wheel zoom. Purely local, so it lives here rather than in GameInput, which exists to
        /// turn keys into messages for the server.
        ///
        /// One step per frame off the sign rather than the raw delta: a notch reads as 120 on
        /// Windows and as 1 elsewhere, and nothing normalizes that for us.
        ///
        /// Note the server culls snapshots to DefaultFOVRadius around each player, so zooming
        /// past that shows empty floor where players actually are until that radius grows.
        /// </summary>
        private void ApplyZoom()
        {
            if (_camera == null) return;

            var mouse = Mouse.current;
            float scroll = mouse != null ? mouse.scroll.ReadValue().y : 0f;
            if (scroll != 0f)
            {
                float step = Mathf.Pow(zoomFactor, -Mathf.Sign(scroll));
                _zoomTarget = Mathf.Clamp(_zoomTarget * step, minZoom, maxZoom);
            }

            _camera.orthographicSize = zoomSmoothing > 0f
                ? Mathf.Lerp(_camera.orthographicSize, _zoomTarget, 1f - Mathf.Exp(-zoomSmoothing * Time.deltaTime))
                : _zoomTarget;
        }

        private void SetupCamera()
        {
            _camera = Camera.main;
            if (_camera == null)
            {
                var go = new GameObject("Main Camera") { tag = "MainCamera" };
                _camera = go.AddComponent<Camera>();
            }

            _camera.orthographic = true;
            _zoomTarget = Mathf.Clamp(cameraSize, minZoom, maxZoom);
            _camera.orthographicSize = _zoomTarget;
            _camera.clearFlags = CameraClearFlags.SolidColor;
            _camera.backgroundColor = new Color32(0xcb, 0xd5, 0xe1, 0xff);
            // Framed on the world by ApplyWorldSize once the server says how big it is
            _camera.transform.position = new Vector3(0f, 0f, -10f);
        }

        // 8x8 cells of 2x2 tiles (marble squares), tiled across the world.
        private void BuildBackground()
        {
            const int cellPx = 64;
            const int cells = 8;
            const int size = cellPx * cells;
            const float tileWorld = 2f * cells;

            var tex = new Texture2D(size, size, TextureFormat.RGBA32, true)
            {
                filterMode = FilterMode.Bilinear,
                wrapMode = TextureWrapMode.Repeat,
            };

            var white = new Color32(255, 255, 255, 255);
            var marble = new Color32(0xf1, 0xf5, 0xf9, 255);
            var grout = new Color32(0xcb, 0xd5, 0xe1, 255);
            var px = new Color32[size * size];
            for (int y = 0; y < size; y++)
            {
                for (int x = 0; x < size; x++)
                {
                    bool edge = x % cellPx == 0 || y % cellPx == 0;
                    bool even = (x / cellPx + y / cellPx) % 2 == 0;
                    px[y * size + x] = edge ? grout : even ? white : marble;
                }
            }
            tex.SetPixels32(px);
            tex.Apply(true);

            var sprite = Sprite.Create(tex, new Rect(0, 0, size, size), new Vector2(0.5f, 0.5f),
                size / tileWorld, 0, SpriteMeshType.FullRect);

            var go = new GameObject("Floor");
            go.transform.SetParent(transform, false);

            // The tile sprite is built once here; it stays zero-sized until ApplyWorldSize
            _floor = go.AddComponent<SpriteRenderer>();
            _floor.sprite = sprite;
            _floor.drawMode = SpriteDrawMode.Tiled;
            _floor.tileMode = SpriteTileMode.Continuous;
            _floor.sortingOrder = BackgroundOrder;
        }

        /// <summary>
        /// Sizes the floor to the world the server described, instead of hardcoding the
        /// backend's bounds. Runs again on every InitialGameState, so a resend can resize it.
        /// </summary>
        private void ApplyWorldSize(float size)
        {
            _floor.transform.position = new Vector3(size / 2f, -size / 2f, 0f);
            _floor.size = new Vector2(size, size);

            // Frame the whole map until a snapshot gives LateUpdate someone to follow
            if (_cameraFollow == null)
                _camera.transform.position = new Vector3(size / 2f, -size / 2f, -10f);
        }

        private void HandleInitialState(InitialGameState state)
        {
            if (state.WorldSize > 0f) ApplyWorldSize(state.WorldSize);

            foreach (var s in _stations) Destroy(s.Go);
            _stations.Clear();

            // The old world's goal; the spectator sends the new one right after.
            if (_goal != null) _goal.SetActive(false);

            // A new initial state means a new world whose IDs restart at 1; keeping the old views
            // would glide them from where the last world ended to the new spawn.
            foreach (var p in _players.Values) Destroy(p.Go);
            _players.Clear();

            foreach (var st in state.StationLayout)
            {
                var go = new GameObject($"Station {st.Label}");
                go.transform.SetParent(transform, false);
                go.transform.position = new Vector3(st.X, -st.Y, 0f);

                var icon = commodityIcons != null ? commodityIcons.Get(st.Commodity) : null;
                var art = new GameObject(icon != null ? "Icon" : "Disc");
                art.transform.SetParent(go.transform, false);
                var body = art.AddComponent<SpriteRenderer>();
                body.sortingOrder = StationOrder;

                float labelHeight;
                if (icon != null)
                {
                    body.sprite = icon;
                    labelHeight = icon.bounds.extents.y * 0.8f + 0.15f;
                }
                else
                {
                    art.transform.localScale = Vector3.one * StationRadius * 2f;
                    body.sprite = Shapes.Circle;
                    body.color = new Color32(0xf5, 0x9e, 0x0b, 0xff);
                    labelHeight = 1.1f;
                }

                var label = OutlinedLabel.Create(go.transform, new Vector2(0f, labelHeight), Color.white, LabelOrder);
                _stations.Add(new StationView { Commodity = st.Commodity, Name = st.Label, Label = label, Go = go });
            }

            RefreshStationLabels();
        }

        // The spectated bot's goal: a translucent disc the size of the arrival radius, so you can
        // see how close counts, with a solid dot on the exact point.
        private void HandleGoalMarker(GoalMarker goal)
        {
            if (_goal == null)
            {
                _goal = new GameObject("Goal");
                _goal.transform.SetParent(transform, false);

                var zone = new GameObject("Zone").AddComponent<SpriteRenderer>();
                zone.transform.SetParent(_goal.transform, false);
                zone.transform.localScale = Vector3.one * GameState.TradeRange * 2f;
                zone.sprite = Shapes.Circle;
                zone.color = new Color32(0x22, 0xc5, 0x5e, 0x40);
                zone.sortingOrder = GoalOrder;

                var dot = new GameObject("Dot").AddComponent<SpriteRenderer>();
                dot.transform.SetParent(_goal.transform, false);
                dot.transform.localScale = Vector3.one * 0.5f;
                dot.sprite = Shapes.Circle;
                dot.color = new Color32(0x16, 0xa3, 0x4a, 0xff);
                dot.sortingOrder = GoalOrder;

                OutlinedLabel.Create(_goal.transform, new Vector2(0f, GameState.TradeRange + 0.4f), Color.white, LabelOrder).Text = "GOAL";
            }

            _goal.transform.position = new Vector3(goal.X, -goal.Y, 0f);
            _goal.SetActive(true);
        }

        private void RefreshStationLabels()
        {
            uint units = _input.OrderUnits;
            foreach (var s in _stations)
            {
                // Per-unit prices for the selected order size: E buys that many, Q sells that many.
                s.Label.Text = _state.TryGetOrderQuote(s.Commodity, units, out var q) && q.BuyPriceCents > 0
                    ? $"{s.Name} x{units} (E {GameState.MoneyCents(q.BuyPriceCents)} / Q {GameState.MoneyCents(q.SellPriceCents)})"
                    : s.Name;
            }
        }

        private void HandleSnapshot(WorldSnapshot snapshot)
        {
            _seen.Clear();

            for (int i = 0; i < snapshot.Players.Count; i++)
            {
                var p = snapshot.Players[i];
                _seen.Add(p.Id);

                var target = new Vector2(p.X, -p.Y);
                if (!_players.TryGetValue(p.Id, out var view))
                {
                    view = CreatePlayer(p, i == 0);
                    view.Go.transform.position = target;
                    view.Target = target;
                    _players[p.Id] = view;
                }
                else
                {
                    view.Velocity = target - view.Target;
                    view.Target = target;
                }
            }

            // Players that left our field of view (or the game) get removed rather than
            // frozen, so they don't reappear with a stale position and a huge velocity.
            _stale.Clear();
            foreach (var id in _players.Keys)
            {
                if (!_seen.Contains(id)) _stale.Add(id);
            }
            foreach (var id in _stale)
            {
                Destroy(_players[id].Go);
                _players.Remove(id);
            }
        }

        // The server always lists the receiving player first in its own snapshot.
        private PlayerView CreatePlayer(PlayerState state, bool isMe)
        {
            var go = new GameObject($"Player {state.Id}");
            go.transform.SetParent(transform, false);

            // Bots get their own sheets so they read as server-run at a glance; the animation
            // and direction handling is identical, only the art differs.
            var art = state.IsBot && botArt != null ? botArt : playerArt;

            PlayerAvatar avatar;
            if (art != null)
            {
                var sprites = go.AddComponent<DirectionalSpriteView>();
                sprites.Build(art);
                avatar = sprites;
            }
            else
            {
                var crew = go.AddComponent<CrewmateView>();
                crew.Build(isMe ? CrewmateView.Emerald : CrewmateView.ForPlayer(state.Id), CrewmateSize);
                avatar = crew;
            }

            var label = OutlinedLabel.Create(go.transform, new Vector2(0f, avatar.LabelHeight), Color.white, LabelOrder);
            label.Text = isMe && !state.IsBot ? "You"
                : string.IsNullOrEmpty(state.Name) ? $"P{state.Id}" : state.Name;

            return new PlayerView { Go = go, Avatar = avatar, IsMe = isMe, IsBot = state.IsBot };
        }
    }
}
