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

            // Every quote carries the same sizes; the server owns the list.
            if (m.Quotes.Count > 0 && !SameSizes(m.Quotes[0]))
            {
                _orderSizes.Clear();
                foreach (var o in m.Quotes[0].Orders) _orderSizes.Add(o.Units);
            }
            PricesChanged?.Invoke();
        }

        private void OnInitial(InitialGameState s)
        {
            _stations.Clear();
            _stations.AddRange(s.StationLayout);
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
