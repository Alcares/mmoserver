using System;
using System.IO;
using UnityEngine;

namespace Game.EditorTools
{
    /// <summary>
    /// Tiny signed-distance-field rasterizer for generating flat-shaded, outlined icons.
    /// Origin is bottom-left, y up. Shapes are drawn in call order, back to front.
    /// </summary>
    internal sealed class IconCanvas
    {
        public const int Size = 256;
        public static readonly Color Ink = new Color32(0x14, 0x18, 0x1f, 255);

        public readonly struct Sd
        {
            public readonly Func<Vector2, float> F; // negative inside
            public readonly Vector2 Min, Max;

            public Sd(Func<Vector2, float> f, Vector2 min, Vector2 max)
            {
                F = f;
                Min = min;
                Max = max;
            }
        }

        private readonly Color[] _px = new Color[Size * Size];

        /// <summary>Translates everything drawn, so a finished composition can be recentred.</summary>
        public Vector2 Offset;

        public static Vector2 P(float x, float y) => new(x, y);

        public static Color Hex(string hex)
        {
            ColorUtility.TryParseHtmlString(hex, out var c);
            return c;
        }

        public void Draw(Sd s, Color fill, float outline = 0f, Color? outlineColor = null)
        {
            Draw(s, fill, fill, outline, outlineColor);
        }

        /// <summary>Vertical gradient from bottom to top colour across the shape's bounds.</summary>
        public void Draw(Sd s, Color top, Color bottom, float outline = 0f, Color? outlineColor = null)
        {
            var ink = outlineColor ?? Ink;
            float pad = outline + 1.5f;
            int x0 = Mathf.Max(0, Mathf.FloorToInt(s.Min.x + Offset.x - pad));
            int x1 = Mathf.Min(Size - 1, Mathf.CeilToInt(s.Max.x + Offset.x + pad));
            int y0 = Mathf.Max(0, Mathf.FloorToInt(s.Min.y + Offset.y - pad));
            int y1 = Mathf.Min(Size - 1, Mathf.CeilToInt(s.Max.y + Offset.y + pad));

            for (int y = y0; y <= y1; y++)
            {
                for (int x = x0; x <= x1; x++)
                {
                    var p = new Vector2(x + 0.5f, y + 0.5f) - Offset;
                    float d = s.F(p);

                    if (outline > 0f)
                    {
                        float o = Coverage(d - outline);
                        if (o > 0f) Blend(y * Size + x, ink, o);
                    }

                    float c = Coverage(d);
                    if (c > 0f)
                    {
                        float t = Mathf.InverseLerp(s.Min.y, s.Max.y, p.y);
                        Blend(y * Size + x, Color.Lerp(bottom, top, t), c);
                    }
                }
            }
        }

        public void Save(string path)
        {
            var tex = new Texture2D(Size, Size, TextureFormat.RGBA32, false);
            tex.SetPixels(_px);
            File.WriteAllBytes(path, tex.EncodeToPNG());
            UnityEngine.Object.DestroyImmediate(tex);
        }

        private static float Coverage(float signedDistance) => Mathf.Clamp01(0.5f - signedDistance);

        private void Blend(int index, Color src, float coverage)
        {
            var dst = _px[index];
            float a = src.a * coverage;
            float outA = a + dst.a * (1f - a);
            if (outA <= 0f) return;
            _px[index] = new Color(
                (src.r * a + dst.r * dst.a * (1f - a)) / outA,
                (src.g * a + dst.g * dst.a * (1f - a)) / outA,
                (src.b * a + dst.b * dst.a * (1f - a)) / outA,
                outA);
        }

        private static Vector2 Rotate(Vector2 v, float degrees)
        {
            float r = degrees * Mathf.Deg2Rad, cs = Mathf.Cos(r), sn = Mathf.Sin(r);
            return new Vector2(v.x * cs - v.y * sn, v.x * sn + v.y * cs);
        }

        /// <summary>Rounded rectangle; rotation is counter-clockwise degrees about its centre.</summary>
        public static Sd Box(Vector2 center, Vector2 half, float radius, float rotation = 0f)
        {
            float r = Mathf.Min(radius, Mathf.Min(half.x, half.y));
            float reach = half.magnitude;
            return new Sd(p =>
            {
                var q = Rotate(p - center, -rotation);
                q = new Vector2(Mathf.Abs(q.x), Mathf.Abs(q.y)) - (half - Vector2.one * r);
                return new Vector2(Mathf.Max(q.x, 0f), Mathf.Max(q.y, 0f)).magnitude + Mathf.Min(Mathf.Max(q.x, q.y), 0f) - r;
            }, center - Vector2.one * reach, center + Vector2.one * reach);
        }

        public static Sd Ellipse(Vector2 center, Vector2 radii, float rotation = 0f)
        {
            float reach = Mathf.Max(radii.x, radii.y);
            return new Sd(p =>
            {
                var q = Rotate(p - center, -rotation);
                float k0 = new Vector2(q.x / radii.x, q.y / radii.y).magnitude;
                float k1 = new Vector2(q.x / (radii.x * radii.x), q.y / (radii.y * radii.y)).magnitude;
                return k1 < 1e-6f ? -Mathf.Min(radii.x, radii.y) : k0 * (k0 - 1f) / k1;
            }, center - Vector2.one * reach, center + Vector2.one * reach);
        }

        public static Sd Segment(Vector2 a, Vector2 b, float radius)
        {
            return new Sd(p =>
            {
                var pa = p - a;
                var ba = b - a;
                float h = Mathf.Clamp01(Vector2.Dot(pa, ba) / Mathf.Max(ba.sqrMagnitude, 1e-6f));
                return (pa - ba * h).magnitude - radius;
            }, Vector2.Min(a, b) - Vector2.one * radius, Vector2.Max(a, b) + Vector2.one * radius);
        }

        public static Sd Poly(params Vector2[] v)
        {
            Vector2 min = v[0], max = v[0];
            foreach (var p in v)
            {
                min = Vector2.Min(min, p);
                max = Vector2.Max(max, p);
            }

            return new Sd(p =>
            {
                float d = (p - v[0]).sqrMagnitude;
                float s = 1f;
                for (int i = 0, j = v.Length - 1; i < v.Length; j = i++)
                {
                    var e = v[j] - v[i];
                    var w = p - v[i];
                    var b = w - e * Mathf.Clamp01(Vector2.Dot(w, e) / e.sqrMagnitude);
                    d = Mathf.Min(d, b.sqrMagnitude);
                    bool c1 = p.y >= v[i].y, c2 = p.y < v[j].y, c3 = e.x * w.y > e.y * w.x;
                    if ((c1 && c2 && c3) || (!c1 && !c2 && !c3)) s = -s;
                }
                return s * Mathf.Sqrt(d);
            }, min, max);
        }
    }
}
