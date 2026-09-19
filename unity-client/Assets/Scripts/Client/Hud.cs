using System.Collections.Generic;
using System.Globalization;
using Game.Networking;
using Game.V1;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// Connection status, the local player's position/balance/portfolio, and a bottom hotbar
    /// with one slot per commodity: icon, dollar value (top right), units (bottom right).
    /// Commodities you don't hold are drawn grayed out and translucent.
    /// Value comes from the latest MarketState quotes rather than the server, so price has
    /// one live source of truth.
    /// </summary>
    [RequireComponent(typeof(GameClient), typeof(GameState), typeof(GameInput))]
    public class Hud : MonoBehaviour
    {
        private const float MultiplierWidth = 56f;
        private const float MultiplierHeight = 26f;
        private const float MultiplierGap = 6f;
        private const float HintWidth = 30f;

        private const float SlotSize = 64f;
        private const float SlotGap = 6f;
        private const float BottomMargin = 14f;
        private const float EmptyAlpha = 0.5f; // slot and icon for a commodity you don't hold
        private const float EmptyTextAlpha = 0.9f;

        [Tooltip("Optional. Icon drawn in each hotbar slot.")]
        [SerializeField] private CommodityIcons commodityIcons;

        private GameClient _client;
        private GameState _state;
        private GameInput _input;
        private readonly List<(string label, string value, Color valueColor)> _rows = new();
        private readonly Dictionary<Sprite, Texture2D> _grayIcons = new();

        private static readonly Color MoneyGreen = new Color32(0x86, 0xef, 0xac, 0xff);
        private static readonly Color LabelGold = new Color32(0xd9, 0xc4, 0x8a, 0xff);

        private GUIStyle _status, _slotText, _multiplierText;
        private Texture2D _panel, _slotBorder, _slotFill, _selectedBorder, _selectedFill;

        private void Awake()
        {
            _client = GetComponent<GameClient>();
            _state = GetComponent<GameState>();
            _input = GetComponent<GameInput>();
        }

        private string StatusText()
        {
            var conn = _client.Connection;
            if (conn == null || conn.CurrentState == GameConnection.State.Connecting) return "Status: Connecting...";
            return conn.CurrentState == GameConnection.State.Connected
                ? "Status: Connected. Tracking local player."
                : "Status: Disconnected. Server stopped.";
        }

        private void OnGUI()
        {
            if (_slotText == null) CreateStyles();

            float scale = Mathf.Clamp(Screen.height / 720f, 1f, 3f);
            GUI.matrix = Matrix4x4.Scale(new Vector3(scale, scale, 1f));
            float screenW = Screen.width / scale;
            float screenH = Screen.height / scale;

            var status = StatusText();
            var size = _status.CalcSize(new GUIContent(status));
            var statusRect = new Rect((screenW - size.x - 28f) / 2f, 12f, size.x + 28f, size.y + 12f);
            GUI.DrawTexture(statusRect, _panel);
            GUI.Label(statusRect, status, _status);

            if (_state.Position == null) return;

            DrawStats();
            DrawHotbar(screenW, screenH);
            DrawMultiplier(screenW, screenH);
        }

        // Every multiplier in a row above the hotbar, the selected one highlighted; T cycles.
        private void DrawMultiplier(float screenW, float screenH)
        {
            var options = _input.Multipliers;
            if (options.Count == 0) return;

            float total = HintWidth + options.Count * MultiplierWidth + (options.Count - 1) * MultiplierGap;
            float x = (screenW - total) / 2f;
            float y = screenH - SlotSize - BottomMargin - MultiplierHeight - 10f;

            GUI.color = Color.white;
            _multiplierText.normal.textColor = new Color32(0xa1, 0xa1, 0xaa, 0xff);
            GUI.Label(new Rect(x, y, HintWidth - 4f, MultiplierHeight), "[T]", _multiplierText);
            x += HintWidth;

            for (int i = 0; i < options.Count; i++)
            {
                bool selected = i == _input.MultiplierIndex;
                var rect = new Rect(x, y, MultiplierWidth, MultiplierHeight);

                GUI.color = new Color(1f, 1f, 1f, selected ? 1f : 0.6f);
                GUI.DrawTexture(rect, selected ? _selectedBorder : _slotBorder);
                GUI.DrawTexture(new Rect(rect.x + 2f, rect.y + 2f, rect.width - 4f, rect.height - 4f), selected ? _selectedFill : _slotFill);

                _multiplierText.normal.textColor = selected ? new Color32(0xfd, 0xe6, 0x8a, 0xff) : new Color32(0xd4, 0xd4, 0xd8, 0xff);
                GUI.Label(rect, options[i].ToString("0.##", CultureInfo.InvariantCulture) + "x", _multiplierText);

                x += MultiplierWidth + MultiplierGap;
            }
            GUI.color = Color.white;
        }

        // Position, balance and portfolio in the same bordered-slot style as the hotbar:
        // muted labels on the left, bright outlined values on the right.
        private void DrawStats()
        {
            double portfolio = 0;
            foreach (var c in _state.Holdings) portfolio += _state.ValueOf(c);

            var pos = _state.Position.Value;
            _rows.Clear();
            _rows.Add(("POS", $"{pos.x.ToString("F1", CultureInfo.InvariantCulture)}, {pos.y.ToString("F1", CultureInfo.InvariantCulture)}", Color.white));
            _rows.Add(("CASH", _state.Balance.HasValue ? GameState.MoneyCents(_state.Balance.Value) : "...", MoneyGreen));
            _rows.Add(("PORTFOLIO", GameState.Money(portfolio), MoneyGreen));

            const float rowHeight = 22f, padding = 9f, width = 214f;
            var rect = new Rect(16f, 16f, width, padding * 2f + _rows.Count * rowHeight);

            GUI.color = Color.white;
            GUI.DrawTexture(rect, _slotBorder);
            GUI.DrawTexture(new Rect(rect.x + 2f, rect.y + 2f, rect.width - 4f, rect.height - 4f), _slotFill);

            for (int i = 0; i < _rows.Count; i++)
            {
                var (label, value, valueColor) = _rows[i];
                var row = new Rect(rect.x + 12f, rect.y + padding + i * rowHeight, rect.width - 24f, rowHeight);

                _slotText.alignment = TextAnchor.MiddleLeft;
                DrawOutlined(row, label, LabelGold);
                _slotText.alignment = TextAnchor.MiddleRight;
                DrawOutlined(row, value, valueColor);
            }
        }

        private void DrawHotbar(float screenW, float screenH)
        {
            var slots = _state.Holdings;
            if (slots.Count == 0) return;

            float total = slots.Count * SlotSize + (slots.Count - 1) * SlotGap;
            float x = (screenW - total) / 2f;
            float y = screenH - SlotSize - BottomMargin;

            foreach (var c in slots)
            {
                DrawSlot(new Rect(x, y, SlotSize, SlotSize), c);
                x += SlotSize + SlotGap;
            }
            GUI.color = Color.white;
        }

        private void DrawSlot(Rect slot, OwnedCommodity c)
        {
            bool held = c.Amount > 0;
            GUI.color = new Color(1f, 1f, 1f, held ? 1f : EmptyAlpha);

            GUI.DrawTexture(slot, _slotBorder);
            var inner = new Rect(slot.x + 2f, slot.y + 2f, slot.width - 4f, slot.height - 4f);
            GUI.DrawTexture(inner, _slotFill);

            var icon = commodityIcons != null ? commodityIcons.Get(c.Type) : null;
            if (icon != null)
            {
                var texture = held ? icon.texture : GrayOf(icon);
                GUI.DrawTexture(new Rect(slot.x + 4f, slot.y + 4f, slot.width - 8f, slot.height - 8f), texture);
            }
            else
            {
                GUI.Label(inner, GameState.Label(c.Type), _slotText);
            }

            // The slot and icon are dimmed when empty, but the numbers stay readable.
            GUI.color = new Color(1f, 1f, 1f, held ? 1f : EmptyTextAlpha);

            // Value hangs off the top-right corner; units sit in the bottom-right.
            Color valueColor = held ? MoneyGreen : new Color32(0xa1, 0xa1, 0xaa, 0xff);
            Color unitsColor = held ? Color.white : new Color32(0xa1, 0xa1, 0xaa, 0xff);
            _slotText.alignment = TextAnchor.UpperRight;
            DrawOutlined(new Rect(slot.x, slot.y - 1f, slot.width - 3f, 18f), GameState.Money(_state.ValueOf(c)), valueColor);
            _slotText.alignment = TextAnchor.LowerRight;
            DrawOutlined(new Rect(slot.x, slot.yMax - 18f, slot.width - 3f, 17f), GameState.Units(c).ToString("F2", CultureInfo.InvariantCulture), unitsColor);
        }

        // Desaturated copy of an icon. Falls back to the colour icon if the texture isn't readable.
        private Texture2D GrayOf(Sprite sprite)
        {
            if (_grayIcons.TryGetValue(sprite, out var cached) && cached != null) return cached;

            var source = sprite.texture;
            if (!source.isReadable) return source;

            var pixels = source.GetPixels32();
            for (int i = 0; i < pixels.Length; i++)
            {
                var p = pixels[i];
                byte l = (byte)(0.3f * p.r + 0.59f * p.g + 0.11f * p.b);
                pixels[i] = new Color32(l, l, l, p.a);
            }

            var gray = new Texture2D(source.width, source.height, TextureFormat.RGBA32, false) { filterMode = source.filterMode };
            gray.SetPixels32(pixels);
            gray.Apply();
            _grayIcons[sprite] = gray;
            return gray;
        }

        private void DrawOutlined(Rect rect, string text, Color color)
        {
            _slotText.normal.textColor = Color.black;
            foreach (var d in new[] { new Vector2(-1f, 0f), new Vector2(1f, 0f), new Vector2(0f, -1f), new Vector2(0f, 1f) })
            {
                GUI.Label(new Rect(rect.x + d.x, rect.y + d.y, rect.width, rect.height), text, _slotText);
            }
            _slotText.normal.textColor = color;
            GUI.Label(rect, text, _slotText);
        }

        private static Texture2D Solid(Color color)
        {
            var tex = new Texture2D(1, 1);
            tex.SetPixel(0, 0, color);
            tex.Apply();
            return tex;
        }

        private void CreateStyles()
        {
            _panel = Solid(new Color(15f / 255f, 23f / 255f, 42f / 255f, 0.9f));
            _slotBorder = Solid(new Color32(0x8a, 0x6a, 0x30, 0xff));
            _slotFill = Solid(new Color(0.06f, 0.08f, 0.13f, 0.92f));
            _selectedBorder = Solid(new Color32(0xfa, 0xcc, 0x15, 0xff));
            _selectedFill = Solid(new Color(0.24f, 0.18f, 0.04f, 0.96f));

            _status = new GUIStyle(GUI.skin.label) { fontSize = 13, alignment = TextAnchor.MiddleCenter };
            _status.normal.textColor = new Color32(0xf8, 0xfa, 0xfc, 0xff);

            _slotText = new GUIStyle(GUI.skin.label) { fontSize = 13, fontStyle = FontStyle.Bold, alignment = TextAnchor.MiddleCenter, wordWrap = false };
            _slotText.padding = new RectOffset(0, 0, 0, 0);

            _multiplierText = new GUIStyle(GUI.skin.label) { fontSize = 14, fontStyle = FontStyle.Bold, alignment = TextAnchor.MiddleCenter, wordWrap = false };
            _multiplierText.padding = new RectOffset(0, 0, 0, 0);
        }
    }
}
