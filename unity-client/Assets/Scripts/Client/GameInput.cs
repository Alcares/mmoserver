using System;
using System.Collections.Generic;
using Game.Networking;
using Game.V1;
using UnityEngine;
using UnityEngine.InputSystem;

namespace Game.Client
{
    /// <summary>
    /// Keyboard input: WASD/arrows move, E buys and Q sells at the nearby station, T cycles the
    /// order-size multiplier. Movement is only sent when the vector changes (including back to
    /// zero on release), since the server keeps moving the player along the last direction it received.
    /// </summary>
    [RequireComponent(typeof(GameClient), typeof(GameState))]
    public class GameInput : MonoBehaviour
    {
        [Tooltip("Order sizes T cycles through. A buy offers the displayed price times this in cash; a sell offers this many units.")]
        [SerializeField] private float[] multipliers = { 1f, 2f, 5f, 10f };

        /// <summary>Current input in server axes (+y is down), normalized.</summary>
        public Vector2 Move { get; private set; }

        public IReadOnlyList<float> Multipliers => multipliers;
        public int MultiplierIndex { get; private set; }

        /// <summary>The selected order-size multiplier; 1 if none are configured.</summary>
        public double Multiplier => multipliers.Length == 0 ? 1d : multipliers[MultiplierIndex];

        private GameClient _client;
        private GameState _state;
        private Vector2 _sent;
        private uint _tradeSequenceId;

        private void Awake()
        {
            _client = GetComponent<GameClient>();
            _state = GetComponent<GameState>();
        }

        private void Update()
        {
            var kb = Keyboard.current;
            if (kb == null) return;

            var move = new Vector2(
                Axis(kb.dKey, kb.rightArrowKey) - Axis(kb.aKey, kb.leftArrowKey),
                Axis(kb.sKey, kb.downArrowKey) - Axis(kb.wKey, kb.upArrowKey));
            if (move.sqrMagnitude > 1f) move.Normalize();
            Move = move;

            if (kb.tKey.wasPressedThisFrame && multipliers.Length > 0)
            {
                MultiplierIndex = (MultiplierIndex + 1) % multipliers.Length;
            }

            if (!_client.IsConnected) return;

            if (move != _sent)
            {
                _client.SendMovement(move.x, move.y);
                _sent = move;
            }

            if (kb.eKey.wasPressedThisFrame) TryBuy();
            if (kb.qKey.wasPressedThisFrame) TrySell();
        }

        // Offers the price shown at the station (cents, the cost of one unit) times the selected
        // multiplier. The server converts it at its price when the order executes: a hair over
        // that many units if the price held, and nothing (order rejected, no charge) if it rose so
        // far that the cash no longer buys a whole unit. Bigger orders move the price while they
        // fill, so they get slightly fewer units than multiplier x 1.
        private void TryBuy()
        {
            if (!_state.TryGetNearbyStation(out var station)) return;
            if (!_state.Prices.TryGetValue(station.Commodity, out var quote)) return;

            ulong cash = (ulong)Math.Ceiling(quote.BuyPriceCents * Multiplier);
            if (quote.BuyPriceCents == 0 || _state.Balance == null || _state.Balance.Value < cash) return;

            _client.SendTrade(new TradeRequest
            {
                SequenceId = ++_tradeSequenceId,
                Intent = OrderIntent.IntentAllocateFixed,
                CashAmount = cash,
            });
        }

        // Sells as many units as the selected multiplier says, or whatever is held if that is less,
        // so small remainders can be cleared. Nothing is sent when the player holds none, mirroring
        // how buying needs enough cash.
        private void TrySell()
        {
            if (!_state.TryGetNearbyStation(out var station)) return;
            if (!_state.Prices.TryGetValue(station.Commodity, out var quote) || quote.SellPriceCents == 0) return;

            ulong held = _state.AmountOf(station.Commodity);
            if (held == 0) return;

            _client.SendTrade(new TradeRequest
            {
                SequenceId = ++_tradeSequenceId,
                Intent = OrderIntent.IntentSellFixed,
                UnitAmount = Math.Min(held, (ulong)Math.Round(Multiplier * GameState.UnitScale)),
            });
        }

        private static float Axis(UnityEngine.InputSystem.Controls.KeyControl a, UnityEngine.InputSystem.Controls.KeyControl b)
        {
            return a.isPressed || b.isPressed ? 1f : 0f;
        }
    }
}
