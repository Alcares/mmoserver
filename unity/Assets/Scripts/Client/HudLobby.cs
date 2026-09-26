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

        // The standings table is wider than the lobby panel: it carries three numeric columns.
        private const float StandingsWidth = 440f;
        private const float MoneyColumn = 104f;
        private const float UnitsColumn = 56f;

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
        // Returns the lowest box drawn, or above if none, so more boxes can stack under it.
        private Rect DrawPhaseBanner(Rect above)
        {
            if (_title == null) CreateLobbyStyles();

            string line = _state.Phase switch
            {
                GamePhase.Waiting => $"WAITING FOR PLAYERS  {_state.PlayerCount}/{_state.MinPlayers}",
                GamePhase.Countdown => $"STARTING IN {Seconds(_state.SecondsLeft)}",
                GamePhase.Running => $"TIME LEFT {Clock(_state.SecondsLeft)}",
                _ => null,
            };
            if (line == null) return above;

            // Stacked under the status box, at its width.
            var rect = new Rect(above.x, above.yMax + TopGap, above.width, TopBoxHeight);
            DrawTopBox(rect, line, _state.Phase == GamePhase.Running ? Color.white : LabelGold);

            if (!string.IsNullOrEmpty(_state.GameId) && _state.Phase != GamePhase.Running)
            {
                rect.y += TopBoxHeight + TopGap;
                DrawTopBox(rect, $"CODE  {_state.GameId}", Color.white);
            }

            if (_state.Phase == GamePhase.Waiting)
            {
                rect.y += TopBoxHeight + TopGap;
                DrawTopBox(rect, "B ADDS A BOT", Dim);
            }
            return rect;
        }

        // Only a spectator reports a playback speed. Returns the box, or above if none.
        private Rect DrawPlaybackSpeed(Rect above)
        {
            if (_state.PlaybackSpeed is not { } speed) return above;

            var rect = new Rect(above.x, above.yMax + TopGap, above.width, TopBoxHeight);
            string text = $"PLAYBACK {speed.ToString(CultureInfo.InvariantCulture)}x   [,] SLOWER  [.] FASTER";
            DrawTopBox(rect, text, Mathf.Approximately(speed, 1f) ? Color.white : LabelGold);
            return rect;
        }

        // How far into training the spectated snapshot was taken, against the newest one.
        private void DrawTrainingProgress(Rect above)
        {
            var episode = _state.Episode;
            if (episode == null) return;

            var rect = new Rect(above.x, above.yMax + TopGap, above.width, TopBoxHeight);
            if (episode.SnapshotTimesteps < 0)
            {
                DrawTopBox(rect, "NO SNAPSHOTS, SCRIPTED BOT", Dim);
                return;
            }

            string text = $"SNAPSHOT {Millions(episode.SnapshotTimesteps)}";
            if (episode.FinalTimesteps > 0)
            {
                long percent = episode.SnapshotTimesteps * 100 / episode.FinalTimesteps;
                text = $"SNAPSHOT {Millions(episode.SnapshotTimesteps)} / {Millions(episode.FinalTimesteps)} ({percent}%)";
            }
            DrawTopBox(rect, text, Color.white);
        }

        // 1,998,848 -> "1.99M". Truncated rather than rounded, so a snapshot short of the last
        // never reads the same as it, the way the floored percentage never reads 100% early.
        private static string Millions(long steps) =>
            (steps / 10_000 / 100.0).ToString("0.00", CultureInfo.InvariantCulture) + "M";

        // The spectator played its last snapshot and hung up; the final frame stays behind this.
        private void DrawSpectatingOver(float screenW, float screenH)
        {
            if (_title == null) CreateLobbyStyles();

            float height = LobbyRowHeight * 2f + 40f;
            var rect = new Rect((screenW - LobbyWidth) / 2f, (screenH - height) / 2f, LobbyWidth, height);

            GUI.color = Color.white;
            GUI.DrawTexture(rect, _slotBorder);
            GUI.DrawTexture(new Rect(rect.x + 2f, rect.y + 2f, rect.width - 4f, rect.height - 4f), _slotFill);

            float x = rect.x + 20f;
            float width = rect.width - 40f;

            _title.normal.textColor = LabelGold;
            GUI.Label(new Rect(x, rect.y + 14f, width, LobbyRowHeight), "RECAP OVER", _title);

            var episode = _state.Episode;
            string line = episode != null && episode.SnapshotTimesteps >= 0
                ? $"Watched every snapshot up to {Millions(episode.SnapshotTimesteps)} steps"
                : "Watched every snapshot";
            _lobbyText.normal.textColor = Dim;
            GUI.Label(new Rect(x, rect.y + 14f + LobbyRowHeight, width, LobbyRowHeight), line, _lobbyText);
        }

        // Final standings, highest net worth first, with how much each player traded to get there.
        private void DrawStandings(float screenW, float screenH)
        {
            if (_title == null) CreateLobbyStyles();

            var standings = _state.Standings.Standings;
            float height = 80f + standings.Count * 26f + LobbyRowHeight + 12f;
            var rect = new Rect((screenW - StandingsWidth) / 2f, (screenH - height) / 2f, StandingsWidth, height);

            GUI.color = Color.white;
            GUI.DrawTexture(rect, _slotBorder);
            GUI.DrawTexture(new Rect(rect.x + 2f, rect.y + 2f, rect.width - 4f, rect.height - 4f), _slotFill);

            float x = rect.x + 20f;
            float width = rect.width - 40f;

            _title.normal.textColor = LabelGold;
            GUI.Label(new Rect(x, rect.y + 14f, width, LobbyRowHeight), "GAME OVER", _title);

            DrawStandingsRow(new Rect(x, rect.y + 50f, width, 20f), "PLAYER", "NET WORTH", "VOLUME", "UNITS", Dim, Dim);

            for (int i = 0; i < standings.Count; i++)
            {
                var s = standings[i];
                var row = new Rect(x, rect.y + 72f + i * 26f, width, 24f);
                var place = (i + 1).ToString(CultureInfo.InvariantCulture) + ".";

                DrawStandingsRow(row,
                    $"{place} {s.Name}",
                    GameState.MoneyCents(s.NetWorth),
                    GameState.MoneyCents(s.TradeVolumeCents),
                    s.UnitsTraded.ToString(CultureInfo.InvariantCulture),
                    i == 0 ? LabelGold : Color.white,
                    MoneyGreen);
            }

            var again = new Rect(x, rect.y + 78f + standings.Count * 26f, width, LobbyRowHeight);
            if (GUI.Button(again, "Play again", _button))
            {
                _input.Restart();
            }
        }

        // One standings line: the name fills what the three right-aligned numeric columns leave.
        private void DrawStandingsRow(Rect row, string name, string worth, string volume, string units, Color nameColor, Color worthColor)
        {
            var unitsRect = new Rect(row.xMax - UnitsColumn, row.y, UnitsColumn, row.height);
            var volumeRect = new Rect(unitsRect.x - MoneyColumn, row.y, MoneyColumn, row.height);
            var worthRect = new Rect(volumeRect.x - MoneyColumn, row.y, MoneyColumn, row.height);

            _slotText.alignment = TextAnchor.MiddleLeft;
            DrawOutlined(new Rect(row.x, row.y, worthRect.x - row.x, row.height), name, nameColor);

            _slotText.alignment = TextAnchor.MiddleRight;
            DrawOutlined(worthRect, worth, worthColor);
            DrawOutlined(volumeRect, volume, Dim);
            DrawOutlined(unitsRect, units, Dim);
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
