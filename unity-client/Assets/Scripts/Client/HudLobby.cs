using System.Globalization;
using Game.V1;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// The lobby half of the HUD: the create/join panel shown before we are in a game, the
    /// phase banner while a round waits or counts down, and the final standings.
    /// Part of Hud rather than its own MonoBehaviour so the scene needs no extra component.
    /// </summary>
    public partial class Hud
    {
        private const float LobbyWidth = 320f;
        private const float LobbyRowHeight = 30f;
        private const int CodeLength = 6;

        private string _codeInput = "";
        private GUIStyle _title, _lobbyText, _codeField, _button;

        private static readonly Color Dim = new Color32(0xa1, 0xa1, 0xaa, 0xff);
        private static readonly Color Bad = new Color32(0xf8, 0x71, 0x71, 0xff);

        // Create a game, or join one by its code. Drawn until the server confirms a join,
        // and a rejection leaves the panel up so the code can be retyped on the same connection.
        private void DrawLobby(float screenW, float screenH)
        {
            if (_title == null) CreateLobbyStyles();

            float height = LobbyRowHeight * 5f + 46f;
            var rect = new Rect((screenW - LobbyWidth) / 2f, (screenH - height) / 2f, LobbyWidth, height);

            GUI.color = Color.white;
            GUI.DrawTexture(rect, _slotBorder);
            GUI.DrawTexture(new Rect(rect.x + 2f, rect.y + 2f, rect.width - 4f, rect.height - 4f), _slotFill);

            float x = rect.x + 20f;
            float width = rect.width - 40f;
            float y = rect.y + 16f;

            _title.normal.textColor = LabelGold;
            GUI.Label(new Rect(x, y, width, LobbyRowHeight), "CREWMATE MARKETS", _title);
            y += LobbyRowHeight + 8f;

            if (!_client.IsConnected)
            {
                _lobbyText.normal.textColor = Dim;
                GUI.Label(new Rect(x, y, width, LobbyRowHeight), "Connecting to the server...", _lobbyText);
                return;
            }

            if (GUI.Button(new Rect(x, y, width, LobbyRowHeight), "Create a new game", _button))
            {
                _client.SendCreateGame();
            }
            y += LobbyRowHeight + 12f;

            _lobbyText.normal.textColor = Dim;
            GUI.Label(new Rect(x, y, width, LobbyRowHeight), "or join with a code", _lobbyText);
            y += LobbyRowHeight;

            float codeWidth = width * 0.55f;
            GUI.SetNextControlName("joinCode");
            _codeInput = GUI.TextField(new Rect(x, y, codeWidth, LobbyRowHeight), _codeInput, CodeLength, _codeField)
                .ToUpperInvariant();

            bool submit = Event.current.type == EventType.KeyDown
                          && Event.current.keyCode == KeyCode.Return
                          && GUI.GetNameOfFocusedControl() == "joinCode";

            if (GUI.Button(new Rect(x + codeWidth + 8f, y, width - codeWidth - 8f, LobbyRowHeight), "Join", _button) || submit)
            {
                if (_codeInput.Length > 0) _client.SendJoinGame(_codeInput);
            }
            y += LobbyRowHeight + 10f;

            if (_state.LastRejection.HasValue)
            {
                _lobbyText.normal.textColor = Bad;
                GUI.Label(new Rect(x, y, width, LobbyRowHeight), RejectionText(_state.LastRejection.Value), _lobbyText);
            }
        }

        private static string RejectionText(JoinRejection reason) => reason switch
        {
            JoinRejection.GameNotFound => "No game with that code",
            JoinRejection.GameFull => "That game is full",
            JoinRejection.GameInProgress => "That game has already started",
            JoinRejection.ServerFull => "The server is hosting too many games",
            _ => "Could not join that game",
        };

        // While waiting or counting down: the code to share, how many players are in, and
        // the countdown. During the round it becomes the remaining time.
        private void DrawPhaseBanner(float screenW)
        {
            if (_title == null) CreateLobbyStyles();

            string line = _state.Phase switch
            {
                GamePhase.Waiting => $"WAITING FOR PLAYERS  {_state.PlayerCount}/{_state.MinPlayers}",
                GamePhase.Countdown => $"STARTING IN {Seconds(_state.SecondsLeft)}",
                GamePhase.Running => $"TIME LEFT {Clock(_state.SecondsLeft)}",
                _ => null,
            };
            if (line == null) return;

            var rect = new Rect((screenW - 260f) / 2f, 44f, 260f, 30f);
            GUI.color = Color.white;
            GUI.DrawTexture(rect, _panel);
            _lobbyText.normal.textColor = _state.Phase == GamePhase.Running ? Color.white : LabelGold;
            GUI.Label(rect, line, _lobbyText);

            if (!string.IsNullOrEmpty(_state.GameId) && _state.Phase != GamePhase.Running)
            {
                var codeRect = new Rect(rect.x, rect.yMax + 4f, rect.width, 26f);
                GUI.DrawTexture(codeRect, _panel);
                _lobbyText.normal.textColor = Color.white;
                GUI.Label(codeRect, $"CODE  {_state.GameId}", _lobbyText);
            }
        }

        // Final standings, highest net worth first.
        private void DrawStandings(float screenW, float screenH)
        {
            if (_title == null) CreateLobbyStyles();

            var standings = _state.Standings.Standings;
            float height = 60f + standings.Count * 26f;
            var rect = new Rect((screenW - LobbyWidth) / 2f, (screenH - height) / 2f, LobbyWidth, height);

            GUI.color = Color.white;
            GUI.DrawTexture(rect, _slotBorder);
            GUI.DrawTexture(new Rect(rect.x + 2f, rect.y + 2f, rect.width - 4f, rect.height - 4f), _slotFill);

            float x = rect.x + 20f;
            float width = rect.width - 40f;

            _title.normal.textColor = LabelGold;
            GUI.Label(new Rect(x, rect.y + 14f, width, LobbyRowHeight), "GAME OVER", _title);

            for (int i = 0; i < standings.Count; i++)
            {
                var row = new Rect(x, rect.y + 52f + i * 26f, width, 24f);
                var place = (i + 1).ToString(CultureInfo.InvariantCulture) + ".";

                _slotText.alignment = TextAnchor.MiddleLeft;
                DrawOutlined(row, $"{place} {standings[i].Name}", i == 0 ? LabelGold : Color.white);
                _slotText.alignment = TextAnchor.MiddleRight;
                DrawOutlined(row, GameState.MoneyCents(standings[i].NetWorth), MoneyGreen);
            }
        }

        private static string Seconds(float? left) =>
            Mathf.CeilToInt(left ?? 0f).ToString(CultureInfo.InvariantCulture);

        private static string Clock(float? left)
        {
            int total = Mathf.CeilToInt(left ?? 0f);
            return (total / 60).ToString(CultureInfo.InvariantCulture) + ":" + (total % 60).ToString("D2", CultureInfo.InvariantCulture);
        }

        private void CreateLobbyStyles()
        {
            _title = new GUIStyle(GUI.skin.label) { fontSize = 17, fontStyle = FontStyle.Bold, alignment = TextAnchor.MiddleCenter };
            _lobbyText = new GUIStyle(GUI.skin.label) { fontSize = 13, alignment = TextAnchor.MiddleCenter };
            _codeField = new GUIStyle(GUI.skin.textField) { fontSize = 16, fontStyle = FontStyle.Bold, alignment = TextAnchor.MiddleCenter };
            _button = new GUIStyle(GUI.skin.button) { fontSize = 14, fontStyle = FontStyle.Bold };
        }
    }
}
