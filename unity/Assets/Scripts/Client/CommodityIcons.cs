using System;
using Game.V1;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// One icon sprite per commodity, drawn at its trading station and in the HUD.
    /// Swap any sprite for real art; a missing entry falls back to a plain disc / no icon.
    /// </summary>
    [CreateAssetMenu(menuName = "Game/Commodity Icons")]
    public class CommodityIcons : ScriptableObject
    {
        [Serializable]
        public struct Entry
        {
            public CommodityType commodity;
            public Sprite sprite;
        }

        public Entry[] entries = Array.Empty<Entry>();

        public Sprite Get(CommodityType commodity)
        {
            foreach (var e in entries)
            {
                if (e.commodity == commodity) return e.sprite;
            }
            return null;
        }
    }
}
