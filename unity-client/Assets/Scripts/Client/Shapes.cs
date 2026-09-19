using System.Collections.Generic;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// Runtime-generated white sprites (tinted via SpriteRenderer.color) so the
    /// client needs no authored art. All caches use Unity's null check because
    /// the sprites are destroyed when leaving play mode.
    /// </summary>
    public static class Shapes
    {
        private const int PolygonPixelsPerUnit = 16;

        private static Sprite _circle;
        private static Sprite _square;
        private static readonly Dictionary<int, Sprite> RoundedRects = new();
        private static readonly Dictionary<string, Sprite> Polygons = new();

        /// <summary>1 unit diameter, centered.</summary>
        public static Sprite Circle
        {
            get
            {
                if (_circle == null) _circle = MakeRoundedRect(128, 64, 128, false);
                return _circle;
            }
        }

        /// <summary>1x1 unit, centered.</summary>
        public static Sprite Square
        {
            get
            {
                if (_square == null)
                {
                    var tex = NewTexture(4, 4, FilterMode.Point);
                    var px = new Color32[16];
                    for (int i = 0; i < px.Length; i++) px[i] = new Color32(255, 255, 255, 255);
                    tex.SetPixels32(px);
                    tex.Apply();
                    _square = Sprite.Create(tex, new Rect(0, 0, 4, 4), new Vector2(0.5f, 0.5f), 4);
                }
                return _square;
            }
        }

        /// <summary>
        /// 9-sliced rounded rectangle. Use with SpriteRenderer.drawMode = Sliced and
        /// set .size; cornerRadius is in world units regardless of that size.
        /// </summary>
        public static Sprite RoundedRect(float cornerRadius)
        {
            int key = Mathf.RoundToInt(cornerRadius * 1000f);
            if (!RoundedRects.TryGetValue(key, out var sprite) || sprite == null)
            {
                const int texSize = 32;
                const int cornerPx = 12;
                sprite = MakeRoundedRect(texSize, cornerPx, cornerPx / cornerRadius, true);
                RoundedRects[key] = sprite;
            }
            return sprite;
        }

        /// <summary>
        /// Filled polygon. Points are in sprite units (y-up) around a pivot at the origin;
        /// the sprite is 1 unit per unit, so scale the parent to size it. Cached by key.
        /// </summary>
        public static Sprite Polygon(string key, Vector2[] points)
        {
            if (Polygons.TryGetValue(key, out var cached) && cached != null) return cached;

            Vector2 min = points[0], max = points[0];
            foreach (var p in points)
            {
                min = Vector2.Min(min, p);
                max = Vector2.Max(max, p);
            }

            const int pad = 2;
            int w = Mathf.CeilToInt((max.x - min.x) * PolygonPixelsPerUnit) + pad * 2;
            int h = Mathf.CeilToInt((max.y - min.y) * PolygonPixelsPerUnit) + pad * 2;
            var tex = NewTexture(w, h, FilterMode.Bilinear);
            var px = new Color32[w * h];

            // 2x2 supersampling for smooth edges.
            for (int y = 0; y < h; y++)
            {
                for (int x = 0; x < w; x++)
                {
                    int hits = 0;
                    for (int sy = 0; sy < 2; sy++)
                    {
                        for (int sx = 0; sx < 2; sx++)
                        {
                            var p = new Vector2(
                                min.x + (x - pad + (sx + 0.5f) / 2f) / PolygonPixelsPerUnit,
                                min.y + (y - pad + (sy + 0.5f) / 2f) / PolygonPixelsPerUnit);
                            if (Contains(points, p)) hits++;
                        }
                    }
                    px[y * w + x] = new Color32(255, 255, 255, (byte)(hits * 255 / 4));
                }
            }

            tex.SetPixels32(px);
            tex.Apply();

            var pivot = new Vector2(
                (-min.x * PolygonPixelsPerUnit + pad) / w,
                (-min.y * PolygonPixelsPerUnit + pad) / h);
            var sprite = Sprite.Create(tex, new Rect(0, 0, w, h), pivot, PolygonPixelsPerUnit);
            Polygons[key] = sprite;
            return sprite;
        }

        private static bool Contains(Vector2[] poly, Vector2 p)
        {
            bool inside = false;
            for (int i = 0, j = poly.Length - 1; i < poly.Length; j = i++)
            {
                if ((poly[i].y > p.y) != (poly[j].y > p.y) &&
                    p.x < (poly[j].x - poly[i].x) * (p.y - poly[i].y) / (poly[j].y - poly[i].y) + poly[i].x)
                {
                    inside = !inside;
                }
            }
            return inside;
        }

        private static Sprite MakeRoundedRect(int size, int radiusPx, float pixelsPerUnit, bool sliced)
        {
            var tex = NewTexture(size, size, FilterMode.Bilinear);
            var px = new Color32[size * size];
            float half = size / 2f;

            for (int y = 0; y < size; y++)
            {
                for (int x = 0; x < size; x++)
                {
                    float qx = Mathf.Abs(x + 0.5f - half) - (half - radiusPx);
                    float qy = Mathf.Abs(y + 0.5f - half) - (half - radiusPx);
                    float d = new Vector2(Mathf.Max(qx, 0f), Mathf.Max(qy, 0f)).magnitude - radiusPx;
                    px[y * size + x] = new Color32(255, 255, 255, (byte)(Mathf.Clamp01(0.5f - d) * 255f));
                }
            }

            tex.SetPixels32(px);
            tex.Apply();

            var border = sliced ? new Vector4(radiusPx, radiusPx, radiusPx, radiusPx) : Vector4.zero;
            return Sprite.Create(tex, new Rect(0, 0, size, size), new Vector2(0.5f, 0.5f),
                pixelsPerUnit, 0, SpriteMeshType.FullRect, border);
        }

        private static Texture2D NewTexture(int w, int h, FilterMode filter)
        {
            return new Texture2D(w, h, TextureFormat.RGBA32, false)
            {
                filterMode = filter,
                wrapMode = TextureWrapMode.Clamp,
            };
        }
    }
}
