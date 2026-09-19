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
        /// <summary>Mirrors internal/game/world.go TradeRange; the server is authoritative.</summary>
        public const float TradeRange = 5f;

        /// <summary>Micro-units per whole commodity unit (internal/game/commodity.go UnitScale).</summary>
        public const double UnitScale = 1_000_000d;

        private GameClient _client;
        private readonly Dictionary<CommodityType, PriceQuote> _prices = new();
        private readonly List<OwnedCommodity> _holdings = new();
        private readonly List<TradingStation> _stations = new();

        /// <summary>Local player position in server coordinates (tiles, +y down); null before the first snapshot.</summary>
        public Vector2? Position { get; private set; }
        /// <summary>Cash in cents.</summary>
        public ulong? Balance { get; private set; }
        public IReadOnlyList<OwnedCommodity> Holdings => _holdings;
        public IReadOnlyDictionary<CommodityType, PriceQuote> Prices => _prices;

        public event Action PricesChanged;

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
        }

        private void OnDisable()
        {
            _client.OnWorldSnapshot -= OnSnapshot;
            _client.OnPlayerInventory -= OnInventory;
            _client.OnMarketState -= OnMarket;
            _client.OnInitialState -= OnInitial;
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
            PricesChanged?.Invoke();
        }

        private void OnInitial(InitialGameState s)
        {
            _stations.Clear();
            _stations.AddRange(s.StationLayout);
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

        /// <summary>Quantity in whole units (holdings are stored as micro-units).</summary>
        public static double Units(OwnedCommodity c) => c.Amount / UnitScale;

        /// <summary>Held quantity of a commodity in micro-units, 0 if none.</summary>
        public ulong AmountOf(CommodityType type)
        {
            foreach (var c in _holdings)
            {
                if (c.Type == type) return c.Amount;
            }
            return 0;
        }

        /// <summary>Value of a holding in dollars at its sell price, i.e. what selling it would fetch.</summary>
        public double ValueOf(OwnedCommodity c)
        {
            return _prices.TryGetValue(c.Type, out var q) ? Units(c) * q.SellPriceCents / 100d : 0d;
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
