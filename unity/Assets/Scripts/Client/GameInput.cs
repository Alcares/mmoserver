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
    /// order size, B asks the server for a bot, Ctrl+R leaves for the create/join screen.
    /// Watching a spectator, comma and period step the playback speed down and up.
    /// Movement is only sent when the vector changes (including back to zero on release), since
    /// the server keeps moving the player along the last direction it received.
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

        /// <summary>Playback speeds comma and period step through, as multiples of real time;
        /// the spectator clamps to the same range.</summary>
        private static readonly float[] PlaybackSpeeds = { 0.25f, 0.5f, 1f, 2f, 4f, 8f, 16f, 32f };

        private GameClient _client;
        private GameState _state;
        private Vector2 _sent;
        private uint _tradeSequenceId;
        private uint _botsRequested;

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

            if (kb.rKey.wasPressedThisFrame && (kb.leftCtrlKey.isPressed || kb.rightCtrlKey.isPressed))
            {
                Restart();
                return;
            }

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
            if (kb.bKey.wasPressedThisFrame) _client.SendSpawnBot($"BOT {++_botsRequested}");

            if (_state.PlaybackSpeed is { } speed)
            {
                if (kb.commaKey.wasPressedThisFrame) StepPlaybackSpeed(speed, -1);
                if (kb.periodKey.wasPressedThisFrame) StepPlaybackSpeed(speed, +1);
            }
        }

        // Steps from the speed the spectator last confirmed, not one we asked for, so a request
        // it clamped or another viewer overrode is not built on.
        private void StepPlaybackSpeed(float current, int direction)
        {
            int i = 0;
            while (i < PlaybackSpeeds.Length - 1 && PlaybackSpeeds[i] < current) i++;
            if (PlaybackSpeeds[i] > current && direction > 0) i--; // current sits between two steps
            int next = Mathf.Clamp(i + direction, 0, PlaybackSpeeds.Length - 1);
            if (!Mathf.Approximately(PlaybackSpeeds[next], current)) _client.SendPlaybackSpeed(PlaybackSpeeds[next]);
        }

        /// <summary>Leaves the current game for the create/join screen. The socket goes too:
        /// GameClient.Reconnect has the reason the server cannot put this connection back in the
        /// lobby. Shared with the Play again button on the standings panel.</summary>
        public void Restart()
        {
            _state.LeaveGame();
            _client.Reconnect();

            // Forget what was last sent, so a direction still held when the new player spawns is
            // sent again rather than mistaken for a vector the server already has.
            Move = Vector2.zero;
            _sent = Vector2.zero;
            MultiplierIndex = 0;
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
