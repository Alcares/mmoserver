using System;
using System.Collections.Generic;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// Player art delivered as two sprite sheets (idle, walk). In each sheet a ROW is one
    /// facing direction and a COLUMN is one animation frame; every cell is the same size
    /// and the character's feet sit at the same spot in every cell.
    ///
    /// Row order, top to bottom, clockwise starting with the character facing the camera:
    ///   S, SW, W, NW, N, NE, E, SE        (S = walking down the screen)
    /// With Mirror West the sheet has only 5 rows (S, SE, E, NE, N) and SW/W/NW are
    /// the SE/E/NE rows flipped horizontally.
    ///
    /// Sprites are cut from the sheets at runtime, so any PNG dropped over the sheet works.
    /// </summary>
    [CreateAssetMenu(menuName = "Game/Directional Animation Set")]
    public class DirectionalAnimationSet : ScriptableObject
    {
        [Serializable]
        public class Clip
        {
            public Texture2D sheet;
            [Min(1)] public int frames = 1;
            [Min(0.1f)] public float fps = 8f;
        }

        public const int DirectionCount = 8;

        public Clip idle = new() { frames = 4, fps = 4f };
        public Clip walk = new() { frames = 6, fps = 10f };

        [Tooltip("Sheets have 5 rows (S, SE, E, NE, N); the west-facing rows are mirrored.")]
        public bool mirrorWest;

        [Tooltip("World-space (tile) height of one sheet cell, not of the character inside it.")]
        [Min(0.1f)] public float cellWorldHeight = 2f;

        [Tooltip("Where the player's position sits inside a cell (0,0 = bottom-left). Put this at the feet.")]
        public Vector2 pivot = new(0.5f, 0.1f);

        [Tooltip("Height above the player's position where the name label is drawn.")]
        public float labelHeight = 1.8f;

        [NonSerialized] private readonly Dictionary<int, Sprite> _sprites = new();

        public int Rows => mirrorWest ? 5 : DirectionCount;

        /// <summary>Direction index 0..7 (S, SW, W, NW, N, NE, E, SE) to sheet row + horizontal flip.</summary>
        public void Resolve(int direction, out int row, out bool flip)
        {
            if (!mirrorWest)
            {
                row = direction;
                flip = false;
                return;
            }

            // 5-row sheet rows: 0=S, 1=SE, 2=E, 3=NE, 4=N.
            switch (direction)
            {
                case 0: row = 0; flip = false; break;
                case 1: row = 1; flip = true; break;
                case 2: row = 2; flip = true; break;
                case 3: row = 3; flip = true; break;
                case 4: row = 4; flip = false; break;
                case 5: row = 3; flip = false; break;
                case 6: row = 2; flip = false; break;
                default: row = 1; flip = false; break;
            }
        }

        public Sprite GetSprite(bool walking, int direction, int frame, out bool flip)
        {
            var clip = walking ? walk : idle;
            Resolve(direction, out int row, out flip);
            if (clip.sheet == null) return null;

            int key = ((walking ? 1 : 0) * DirectionCount + row) * 1024 + frame;
            if (_sprites.TryGetValue(key, out var cached) && cached != null) return cached;

            var tex = clip.sheet;
            int cellW = tex.width / Mathf.Max(1, clip.frames);
            int cellH = tex.height / Rows;
            var rect = new Rect(frame * cellW, tex.height - (row + 1) * cellH, cellW, cellH);
            var sprite = Sprite.Create(tex, rect, pivot, cellH / cellWorldHeight, 0, SpriteMeshType.FullRect);
            sprite.name = $"{(walking ? "walk" : "idle")}_r{row}_f{frame}";
            _sprites[key] = sprite;
            return sprite;
        }
    }
}
