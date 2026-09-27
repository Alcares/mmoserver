using System;
using System.Collections.Generic;
using System.Globalization;
using Game.Networking;
using Game.V1;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// The latest server state the local player needs: position, balance, holdings,
    /// market prices and station layout. Single source of truth for the HUD, the
    /// world view and buying, so they can't disagree about the price on screen.
    /// </summary>
    [RequireComponent(typeof(GameClient))]
    public class GameState : MonoBehaviour
    {
        /// <summary>How close to a station the server lets a player trade, from InitialGameState.</summary>
        public float TradeRange { get; private set; }

        private GameClient _client;
        private readonly Dictionary<CommodityType, PriceQuote> _prices = new();
        private readonly List<OwnedCommodity> _holdings = new();
        private readonly List<TradingStation> _stations = new();
        private readonly List<uint> _orderSizes = new();

        /// <summary>Local player position in server coordinates (tiles, +y down); null before the first snapshot.</summary>
        public Vector2? Position { get; private set; }
        /// <summary>Cash in cents.</summary>
        public ulong? Balance { get; private set; }
        public IReadOnlyList<OwnedCommodity> Holdings => _holdings;
        public IReadOnlyDictionary<CommodityType, PriceQuote> Prices => _prices;
        /// <summary>Order sizes (whole units) the server quotes, smallest first; empty before the first MarketState.</summary>
        public IReadOnlyList<uint> OrderSizes => _orderSizes;

        /// <summary>True once the server has put us in a game; false while we are in the lobby.</summary>
        public bool Joined { get; private set; }
        /// <summary>The join code of the game we are in, for sharing with other players.</summary>
        public string GameId { get; private set; }
        /// <summary>Where the round is: waiting for players, counting down, running or finished.</summary>
        public GamePhase Phase { get; private set; } = GamePhase.Unspecified;
        public int PlayerCount { get; private set; }
        public int MinPlayers { get; private set; }
        /// <summary>Final standings once the round is over, null before that.</summary>
        public GameOver Standings { get; private set; }
        /// <summary>Spectator playback speed as a multiple of real time; null when connected to a
        /// game server, so it doubles as "this is a spectator".</summary>
        public float? PlaybackSpeed { get; private set; }
        /// <summary>The spectated episode: its goal and which training snapshot plays it; null
        /// when connected to a game server.</summary>
        public Episode Episode { get; private set; }
        /// <summary>The spectator has played every snapshot and closed the connection.</summary>
        public bool SpectatingOver { get; private set; }
        /// <summary>Why the last create or join attempt failed, null if none has.</summary>
        public JoinRejection? LastRejection { get; private set; }

        // Counted down locally from the remaining_ms the server sent with the last phase change,
        // so the label ticks smoothly instead of once per status message. Anchored to
        // realtimeSinceStartup, not Time.time: Time.time only advances by deltaTime, which Unity
        // clamps to maximumDeltaTime, so it falls permanently behind whenever frames are slow or
        // the window is unfocused, and the server only resends the status on a phase change.
        // Game time runs at the spectator's playback speed, so the countdown does too; a speed
        // change re-anchors it at the time left then.
        private float? _phaseLeft;
        private float _phaseAnchor;

        /// <summary>Seconds of game time left in the current phase, null when the phase is open-ended.</summary>
        public float? SecondsLeft => _phaseLeft.HasValue
            ? Mathf.Max(0f, _phaseLeft.Value - (Time.realtimeSinceStartup - _phaseAnchor) * (PlaybackSpeed ?? 1f))
            : (float?)null;

        /// <summary>A random event the server has started and not yet ended. The server never runs
        /// two with the same type and commodity at once, so that pair identifies it.</summary>
        public sealed class ActiveEvent
        {
            public RandomEventOccurred Event;
            // The remaining_ms it arrived with, in seconds, counted down locally from when it
            // arrived. realtimeSinceStartup for the same reason as the phase timer.
            public float Total;
            public float StartedAt;

            /// <summary>Share still to run, 1 when it starts and 0 when it is due to end.</summary>
            public float LeftFraction => Total > 0f
                ? Mathf.Clamp01(1f - (Time.realtimeSinceStartup - StartedAt) / Total)
                : 0f;

            public float SecondsLeft => LeftFraction * Total;
        }

        private readonly List<ActiveEvent> _events = new();

        /// <summary>The random events running now, oldest first. Only an event's own RandomEventEnded
        /// removes it; the countdowns are for display.</summary>
        public IReadOnlyList<ActiveEvent> ActiveEvents => _events;

        /// <summary>Whether an event of type t is running on commodity c.</summary>
        public bool EventOn(CommodityType c, RandomCommodityEventType t) =>
            _events.Exists(a => a.Event.EventType == t && a.Event.Commodity == c);

        public event Action PricesChanged;
        /// <summary>Raised when the phase, the player count or the join result changes.</summary>
        public event Action SessionChanged;
        /// <summary>Raised when a random event starts or ends.</summary>
        public event Action EventChanged;

        private void Awake()
        {
            _client = GetComponent<GameClient>();
        }

        private void OnEnable()
        {
            _client.OnWorldSnapshot += OnSnapshot;
            _client.OnPlayerInventory += OnInventory;
            _client.OnMarketState += OnMarket;
            _client.OnInitialState += OnInitial;
            _client.OnGameStatus += OnGameStatus;
            _client.OnJoinRejected += OnJoinRejected;
            _client.OnGameOver += OnGameOver;
            _client.OnPlaybackSpeed += OnPlaybackSpeed;
            _client.OnEpisode += OnEpisode;
            _client.OnSpectatingOver += OnSpectatingOver;
            _client.OnRandomEventOccurred += OnRandomEventOccurred;
            _client.OnRandomEventEnded += OnRandomEventEnded;
        }

        private void OnDisable()
        {
            _client.OnWorldSnapshot -= OnSnapshot;
            _client.OnPlayerInventory -= OnInventory;
            _client.OnMarketState -= OnMarket;
            _client.OnInitialState -= OnInitial;
            _client.OnGameStatus -= OnGameStatus;
            _client.OnJoinRejected -= OnJoinRejected;
            _client.OnGameOver -= OnGameOver;
            _client.OnPlaybackSpeed -= OnPlaybackSpeed;
            _client.OnEpisode -= OnEpisode;
            _client.OnSpectatingOver -= OnSpectatingOver;
            _client.OnRandomEventOccurred -= OnRandomEventOccurred;
            _client.OnRandomEventEnded -= OnRandomEventEnded;
        }

        // The server lists the receiving player first in its own snapshot.
        private void OnSnapshot(WorldSnapshot s)
        {
            if (s.Players.Count > 0) Position = new Vector2(s.Players[0].X, s.Players[0].Y);
        }

        private void OnInventory(PlayerInventory inv)
        {
            Balance = inv.Balance;
            _holdings.Clear();
            _holdings.AddRange(inv.Commodities);
            _holdings.Sort((a, b) => string.CompareOrdinal(a.Type.ToString(), b.Type.ToString()));
        }

        private void OnMarket(MarketState m)
        {
            foreach (var q in m.Quotes) _prices[q.Commodity] = q;

            // Every quote carries the same sizes; the server owns the list.
            if (m.Quotes.Count > 0 && !SameSizes(m.Quotes[0]))
            {
                _orderSizes.Clear();
                foreach (var o in m.Quotes[0].Orders) _orderSizes.Add(o.Units);
            }
            PricesChanged?.Invoke();
        }

        // InitialGameState is the server's confirmation that we are in a game: it arrives
        // once per join and carries the code and this game's station layout.
        private void OnInitial(InitialGameState s)
        {
            _stations.Clear();
            _stations.AddRange(s.StationLayout);
            TradeRange = s.TradeRange;
            ClearEvents();

            Joined = true;
            GameId = s.GameId;
            Standings = null;
            LastRejection = null;
            SessionChanged?.Invoke();
        }

        private void OnGameStatus(GameStatus s)
        {
            Phase = s.Phase;
            PlayerCount = s.PlayerCount;
            MinPlayers = s.MinPlayers;
            _phaseLeft = s.RemainingMs > 0 ? s.RemainingMs / 1000f : (float?)null;
            _phaseAnchor = Time.realtimeSinceStartup;
            SessionChanged?.Invoke();
        }

        private void OnJoinRejected(JoinRejected r)
        {
            LastRejection = r.Reason;
            SessionChanged?.Invoke();
        }

        private void OnGameOver(GameOver o)
        {
            Standings = o;
            SessionChanged?.Invoke();
        }

        private void OnPlaybackSpeed(PlaybackSpeed s)
        {
            // At the old speed, before it changes.
            _phaseLeft = SecondsLeft;
            _phaseAnchor = Time.realtimeSinceStartup;
            PlaybackSpeed = s.Speed;
            SessionChanged?.Invoke();
        }

        private void OnEpisode(Episode e)
        {
            Episode = e;
            SessionChanged?.Invoke();
        }

        private void OnSpectatingOver(SpectatingOver _)
        {
            SpectatingOver = true;
            SessionChanged?.Invoke();
        }

        private void OnRandomEventOccurred(RandomEventOccurred e)
        {
            _events.RemoveAll(a => a.Event.EventType == e.EventType && a.Event.Commodity == e.Commodity);
            _events.Add(new ActiveEvent { Event = e, Total = e.RemainingMs / 1000f, StartedAt = Time.realtimeSinceStartup });
            EventChanged?.Invoke();
        }

        private void OnRandomEventEnded(RandomEventEnded e)
        {
            if (_events.RemoveAll(a => a.Event.EventType == e.EventType && a.Event.Commodity == e.Commodity) > 0)
                EventChanged?.Invoke();
        }

        private void ClearEvents()
        {
            _events.Clear();
            EventChanged?.Invoke();
        }

        /// <summary>Forgets everything tied to one game, which drops the HUD back to the lobby
        /// panel. The socket is separate and has to go too; see GameClient.Reconnect.</summary>
        public void LeaveGame()
        {
            Joined = false;
            GameId = null;
            Phase = GamePhase.Unspecified;
            PlayerCount = 0;
            MinPlayers = 0;
            Standings = null;
            LastRejection = null;
            PlaybackSpeed = null;
            Episode = null;
            SpectatingOver = false;
            Position = null;
            Balance = null;
            _phaseLeft = null;
            _stations.Clear();
            _holdings.Clear();
            _prices.Clear();
            _orderSizes.Clear();

            ClearEvents();
            SessionChanged?.Invoke();
            PricesChanged?.Invoke();
        }

        private bool SameSizes(PriceQuote q)
        {
            if (q.Orders.Count != _orderSizes.Count) return false;
            for (int i = 0; i < q.Orders.Count; i++)
            {
                if (q.Orders[i].Units != _orderSizes[i]) return false;
            }
            return true;
        }

        /// <summary>The per-unit buy/sell prices for an order of this many units of a commodity.</summary>
        public bool TryGetOrderQuote(CommodityType type, uint units, out OrderQuote quote)
        {
            quote = null;
            if (!_prices.TryGetValue(type, out var q)) return false;
            foreach (var o in q.Orders)
            {
                if (o.Units == units)
                {
                    quote = o;
                    return true;
                }
            }
            return false;
        }

        /// <summary>The nearest station within trading range of the local player, as the server will see it.</summary>
        public bool TryGetNearbyStation(out TradingStation station)
        {
            station = null;
            if (Position == null) return false;

            float nearest = TradeRange;
            foreach (var s in _stations)
            {
                float d = Vector2.Distance(Position.Value, new Vector2(s.X, s.Y));
                if (d <= nearest)
                {
                    nearest = d;
                    station = s;
                }
            }
            return station != null;
        }

        /// <summary>Held whole units of a commodity, 0 if none.</summary>
        public ulong AmountOf(CommodityType type)
        {
            foreach (var c in _holdings)
            {
                if (c.Type == type) return c.Amount;
            }
            return 0;
        }

        /// <summary>Value of a holding in dollars at the one-unit sell price.</summary>
        public double ValueOf(OwnedCommodity c)
        {
            return TryGetOrderQuote(c.Type, 1, out var q) ? c.Amount * q.SellPriceCents / 100d : 0d;
        }

        public static string Money(double dollars) => "$" + dollars.ToString("F2", CultureInfo.InvariantCulture);

        public static string MoneyCents(ulong cents) => Money(cents / 100d);

        public static string Label(CommodityType t)
        {
            const string prefix = "Commodity";
            var name = t.ToString();
            return (name.StartsWith(prefix, StringComparison.Ordinal) ? name.Substring(prefix.Length) : name).ToUpperInvariant();
        }
    }
}
