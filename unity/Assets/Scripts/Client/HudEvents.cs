using Game.V1;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// The random-event half of the HUD: a bar in the top stack, draining until the server ends the event.
    /// </summary>
    public partial class Hud
    {
        private Texture2D _eventFill;

        // What each event type announces; null for a type the client does not show.
        private static string EventText(RandomEventOccurred e) => e.EventType switch
        {
            RandomCommodityEventType.RandomCommodityEventClosed => $"{GameState.Label(e.Commodity)} STATION CLOSED",
            RandomCommodityEventType.RandomCommodityEventSanctioned => $"{GameState.Label(e.Commodity)} TRADES TAXED",
            RandomCommodityEventType.RandomCommodityEventSupplyFlood => $"{GameState.Label(e.Commodity)} SUPPLY FLOOD",
            RandomCommodityEventType.RandomCommodityEventSupplyShortage => $"{GameState.Label(e.Commodity)} SUPPLY SHORTAGE",
            _ => null,
        };

        // One bordered box per running event, oldest on top, each fill draining from right to left
        // as that event runs out. Returns the lowest bar, or above if none.
        private Rect DrawEventBars(Rect above)
        {
            if (_eventFill == null) _eventFill = Solid(new Color32(0xb9, 0x1c, 0x1c, 0xe0));

            foreach (var a in _state.ActiveEvents)
            {
                if (EventText(a.Event) is not { } text) continue;

                var rect = new Rect(above.x, above.yMax + TopGap, above.width, TopBoxHeight);
                GUI.color = Color.white;
                GUI.DrawTexture(rect, _slotBorder);
                var inner = new Rect(rect.x + 2f, rect.y + 2f, rect.width - 4f, rect.height - 4f);
                GUI.DrawTexture(inner, _slotFill);
                GUI.DrawTexture(new Rect(inner.x, inner.y, inner.width * a.LeftFraction, inner.height), _eventFill);

                _slotText.alignment = TextAnchor.MiddleCenter;
                DrawOutlined(rect, $"{text}  {Seconds(a.SecondsLeft)}s", Color.white);
                above = rect;
            }
            return above;
        }
    }
}
