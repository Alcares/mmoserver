using System.Collections.Generic;
using Game.Networking;
using Game.V1;
using UnityEngine;

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
        // Mirrors internal/game/world.go WorldMaxX/WorldMaxY.
        private const float WorldSize = 500f;
        private const float CrewmateSize = 1.3f;
        private const float StationRadius = 0.9f;
        private const int BackgroundOrder = -100;
        private const int StationOrder = -50;
        private const int LabelOrder = 10000;

        [SerializeField] private float cameraSize = 12f;
        [Tooltip("Higher = snappier follow of the 20Hz server snapshots.")]
        [SerializeField] private float smoothing = 20f;
        [Tooltip("Optional 8-direction player art. Leave empty to use the built-in procedural crewmate.")]
        [SerializeField] private DirectionalAnimationSet playerArt;
        [Tooltip("Optional station icons. A commodity without an icon is drawn as a plain disc.")]
        [SerializeField] private CommodityIcons commodityIcons;

        private sealed class PlayerView
        {
            public GameObject Go;
            public PlayerAvatar Avatar;
            public Vector2 Target;
            public Vector2 Velocity;
            public bool IsMe;
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

        private readonly Dictionary<uint, PlayerView> _players = new();
        private readonly HashSet<uint> _seen = new();
        private readonly List<uint> _stale = new();
        private readonly List<StationView> _stations = new();

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
            _state.PricesChanged += RefreshStationLabels;
        }

        private void OnDisable()
        {
            _client.OnWorldSnapshot -= HandleSnapshot;
            _client.OnInitialState -= HandleInitialState;
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
                // Input is in server axes (+y down); avatars take a world direction (+y up).
                bool moving = view.IsMe ? _input.Move != Vector2.zero : view.Velocity.magnitude > 0.01f;
                Vector2 direction = view.IsMe ? new Vector2(_input.Move.x, -_input.Move.y) : view.Velocity;

                view.Avatar.Animate(direction, moving);
                view.Avatar.SetSortingOrder(-Mathf.RoundToInt(pos.y * 10f));

                if (view.IsMe) _cameraFollow = view.Go.transform;
            }
        }

        private void LateUpdate()
        {
            if (_cameraFollow == null) return;
            var p = _cameraFollow.position;
            _camera.transform.position = new Vector3(p.x, p.y, -10f);
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
            _camera.orthographicSize = cameraSize;
            _camera.clearFlags = CameraClearFlags.SolidColor;
            _camera.backgroundColor = new Color32(0xcb, 0xd5, 0xe1, 0xff);
            _camera.transform.position = new Vector3(WorldSize / 2f, -WorldSize / 2f, -10f);
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
            go.transform.position = new Vector3(WorldSize / 2f, -WorldSize / 2f, 0f);

            var sr = go.AddComponent<SpriteRenderer>();
            sr.sprite = sprite;
            sr.drawMode = SpriteDrawMode.Tiled;
            sr.tileMode = SpriteTileMode.Continuous;
            sr.size = new Vector2(WorldSize, WorldSize);
            sr.sortingOrder = BackgroundOrder;
        }

        private void HandleInitialState(InitialGameState state)
        {
            foreach (var s in _stations) Destroy(s.Go);
            _stations.Clear();

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

            PlayerAvatar avatar;
            if (playerArt != null)
            {
                var sprites = go.AddComponent<DirectionalSpriteView>();
                sprites.Build(playerArt);
                avatar = sprites;
            }
            else
            {
                var crew = go.AddComponent<CrewmateView>();
                crew.Build(isMe ? CrewmateView.Emerald : CrewmateView.ForPlayer(state.Id), CrewmateSize);
                avatar = crew;
            }

            var label = OutlinedLabel.Create(go.transform, new Vector2(0f, avatar.LabelHeight), Color.white, LabelOrder);
            label.Text = isMe ? "You" : string.IsNullOrEmpty(state.Name) ? $"P{state.Id}" : state.Name;

            return new PlayerView { Go = go, Avatar = avatar, IsMe = isMe };
        }
    }
}
