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
    /// order size. Movement is only sent when the vector changes (including back to zero on
    /// release), since the server keeps moving the player along the last direction it received.
    /// </summary>
    [RequireComponent(typeof(GameClient), typeof(GameState))]
    public class GameInput : MonoBehaviour
    {
        /// <summary>Current input in server axes (+y is down), normalized.</summary>
        public Vector2 Move { get; private set; }

        /// <summary>Order sizes T cycles through, in whole units; the server's quoted sizes.</summary>
        public IReadOnlyList<uint> Multipliers => _state.OrderSizes;
        public int MultiplierIndex { get; private set; }

        /// <summary>Units in the selected order; 0 before the first MarketState.</summary>
        public uint OrderUnits => Multipliers.Count == 0 ? 0 : Multipliers[Math.Min(MultiplierIndex, Multipliers.Count - 1)];

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

            // While the lobby panel is up the keyboard belongs to it: typing a join code
            // must not walk the player around or fire trades.
            if (!_state.Joined) return;

            var move = new Vector2(
                Axis(kb.dKey, kb.rightArrowKey) - Axis(kb.aKey, kb.leftArrowKey),
                Axis(kb.sKey, kb.downArrowKey) - Axis(kb.wKey, kb.upArrowKey));
            if (move.sqrMagnitude > 1f) move.Normalize();
            Move = move;

            if (kb.tKey.wasPressedThisFrame && Multipliers.Count > 0)
            {
                MultiplierIndex = (MultiplierIndex + 1) % Multipliers.Count;
            }

            if (!_client.IsConnected) return;

            if (move != _sent)
            {
                _client.SendMovement(move.x, move.y);
                _sent = move;
            }

            if (kb.eKey.wasPressedThisFrame) TryTrade(OrderIntent.IntentBuy);
            if (kb.qKey.wasPressedThisFrame) TryTrade(OrderIntent.IntentSell);
        }

        // Buys or sells exactly the selected number of units, sending the per-unit price shown at
        // the station for that size. The server fills the whole order at one price or rejects it
        // (not enough cash or units, or the price moved against us) and answers with a TradeReceipt.
        private void TryTrade(OrderIntent intent)
        {
            if (!_state.TryGetNearbyStation(out var station)) return;
            if (!_state.TryGetOrderQuote(station.Commodity, OrderUnits, out var quote)) return;

            ulong price = intent == OrderIntent.IntentBuy ? quote.BuyPriceCents : quote.SellPriceCents;
            if (price == 0) return;

            _client.SendTrade(new TradeRequest
            {
                SequenceId = ++_tradeSequenceId,
                Intent = intent,
                Units = OrderUnits,
                PriceCents = price,
            });
        }

        private static float Axis(UnityEngine.InputSystem.Controls.KeyControl a, UnityEngine.InputSystem.Controls.KeyControl b)
        {
            return a.isPressed || b.isPressed ? 1f : 0f;
        }
    }
}
