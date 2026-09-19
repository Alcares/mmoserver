using System.Collections.Generic;
using UnityEngine;
using UnityEngine.Rendering;

namespace Game.Client
{
    /// <summary>
    /// Port of web/crewmate.js: an animated suited trader assembled from tinted
    /// sprite parts. Parts are authored in the JS canvas's 32-unit design space
    /// (y down); Y() flips into Unity's y-up local space and the root scale maps
    /// design units to world units.
    /// </summary>
    public class CrewmateView : PlayerAvatar
    {
        public struct Palette
        {
            public Color Suit, SuitShadow, Tie, Skin, Hair;

            public Palette(string suit, string suitShadow, string tie, string skin, string hair)
            {
                Suit = Hex(suit);
                SuitShadow = Hex(suitShadow);
                Tie = Hex(tie);
                Skin = Hex(skin);
                Hair = Hex(hair);
            }
        }

        public static readonly Palette Emerald = new("#065f46", "#064e3b", "#f59e0b", "#a5694f", "#171717");

        // Same palettes as the web client, minus emerald, which marks the local player.
        private static readonly Palette[] OtherPalettes =
        {
            new("#1e3a8a", "#172554", "#dc2626", "#ffd8c4", "#3a2010"),
            new("#334155", "#1e293b", "#0284c7", "#f3c5a5", "#1c1917"),
            new("#18181b", "#09090b", "#eab308", "#dfa67b", "#451a03"),
            new("#881337", "#4c0519", "#38bdf8", "#6e432d", "#0a0a0a"),
            new("#94a3b8", "#64748b", "#2563eb", "#fcd5be", "#292524"),
        };

        public static Palette ForPlayer(uint id) => OtherPalettes[id % (uint)OtherPalettes.Length];

        private const float DesignHeight = 32f;
        private const float Stroke = 1.1f;
        private static readonly Color Ink = Color.black;

        private Transform _root, _body, _torso, _backArm, _briefcase, _backLeg, _frontLeg, _frontArm;
        private int _order;
        private float _scale;
        private bool _facingLeft;

        public void Build(Palette pal, float worldSize)
        {
            _scale = worldSize / DesignHeight;

            _root = Child(transform, "Crewmate", Vector2.zero);
            _root.localScale = new Vector3(_scale, _scale, 1f);
            _root.gameObject.AddComponent<SortingGroup>();
            _body = Child(_root, "Body", Vector2.zero);

            // Back arm and briefcase.
            _backArm = Child(_body, "BackArm", new Vector2(-7f, -3f));
            Circle(_backArm, Vector2.zero, 3f, pal.Skin);
            _briefcase = Child(_backArm, "Briefcase", new Vector2(-1f, -2f));
            Rect(_briefcase, new Vector2(3.5f, -4f), new Vector2(11f, 8f), 2f, Hex("#78350f"), true);
            Rect(_briefcase, new Vector2(3.5f, -4f), new Vector2(2f, 2f), 0.4f, Hex("#facc15"), false);
            Arc(_briefcase, new Vector2(3.5f, 0f), 2f, 180f, 0f, 1.2f, Hex("#292524"));

            // Back leg (shadowed).
            _backLeg = Child(_body, "BackLeg", Vector2.zero);
            Rect(_backLeg, new Vector2(-3f, -10.5f), new Vector2(6f, 9f), 2f, pal.SuitShadow, true);
            Rect(_backLeg, new Vector2(-3.25f, -14.75f), new Vector2(7.5f, 3.5f), 1.5f, Hex("#0f172a"), true);

            // Torso, head and face. The torso leans while walking.
            _torso = Child(_body, "Torso", Vector2.zero);
            Rect(_torso, new Vector2(0f, -2.5f), new Vector2(16f, 15f), 6f, pal.Suit, true);
            Rect(_torso, new Vector2(-5.5f, -4f), new Vector2(5f, 12f), 2.5f, pal.SuitShadow, false);
            Poly(_torso, "collar", Color.white, P(-2, -5), P(6, -5), P(2, 3));
            Poly(_torso, "tie", pal.Tie, P(1, -4), P(3.5f, -4), P(4.5f, 4), P(2.2f, 7), P(0.5f, 4));

            Circle(_torso, new Vector2(0f, 11f), 8.5f, pal.Skin);
            Poly(_torso, "hair", pal.Hair, HairOutline());
            Circle(_torso, new Vector2(-4f, 10f), 2.3f, pal.Skin, 1.4f);
            Line(_torso, Ink, 1.8f, P(7.5f, -12), P(9.5f, -10), P(7, -9));
            Ellipse(_torso, new Vector2(4f, 11.5f), new Vector2(3.2f, 4f), Color.white, 1.6f);
            Circle(_torso, new Vector2(5.2f, 11.5f), 1.6f, Hex("#0f172a"), 0f);
            Circle(_torso, new Vector2(5.8f, 12.3f), 0.6f, Color.white, 0f);
            Line(_torso, pal.Hair, 2f, P(1.5f, -16.5f), P(7, -15));
            Line(_torso, Ink, 1.5f, Quad(P(3, -6.5f), P(6, -6.5f), P(7, -8), 4));

            // Front leg.
            _frontLeg = Child(_body, "FrontLeg", Vector2.zero);
            Rect(_frontLeg, new Vector2(4f, -10.5f), new Vector2(6f, 9f), 2f, pal.Suit, true);
            Rect(_frontLeg, new Vector2(4.25f, -14.75f), new Vector2(8.5f, 3.5f), 1.5f, Hex("#0f172a"), true);

            // Front arm and fist.
            _frontArm = Child(_body, "FrontArm", new Vector2(3f, -3f));
            Circle(_frontArm, Vector2.zero, 3.8f, pal.Suit, 1f);
            Rect(_frontArm, new Vector2(1.75f, 0f), new Vector2(1.5f, 4f), 0f, Color.white, false);
            Circle(_frontArm, new Vector2(3.2f, 0f), 2.7f, pal.Skin, 0.8f);
        }

        public override float LabelHeight => 0.95f;

        public override void SetSortingOrder(int order)
        {
            _root.GetComponent<SortingGroup>().sortingOrder = order;
        }

        public override void Animate(Vector2 direction, bool moving)
        {
            if (direction.x < -0.05f) _facingLeft = true;
            else if (direction.x > 0.05f) _facingLeft = false;
            Pose(moving, _facingLeft);
        }

        private void Pose(bool moving, bool facingLeft)
        {
            float t = Time.time;
            float walk = moving ? t * 20f : 0f;
            float swing = Mathf.Sin(walk);
            float bob = moving ? -Mathf.Abs(swing) * 2.5f : Mathf.Sin(t * 3.5f) * 1.2f;
            float lean = moving ? 0.12f : 0f;
            float leg = moving ? swing * 6f : 0f;
            float arm = moving ? swing * 5f : 0f;

            _root.localScale = new Vector3(facingLeft ? -_scale : _scale, _scale, 1f);
            _body.localPosition = new Vector3(0f, -bob, 0f);
            _torso.localRotation = Quaternion.Euler(0f, 0f, -lean * Mathf.Rad2Deg);
            _backArm.localPosition = new Vector3(-7f - arm * 0.5f, -3f, 0f);
            _briefcase.localRotation = Quaternion.Euler(0f, 0f, -(moving ? swing * 0.2f : 0f) * Mathf.Rad2Deg);
            _backLeg.localPosition = new Vector3(-leg, 0f, 0f);
            _frontLeg.localPosition = new Vector3(leg, 0f, 0f);
            _frontArm.localPosition = new Vector3(3f + arm, -3f, 0f);
        }

        // Points below are written in canvas coordinates (y down), matching crewmate.js.
        private static Vector2 P(float x, float y) => new(x, -y);

        private static Vector2[] HairOutline()
        {
            var pts = new List<Vector2> { P(-8.5f, -11) };
            pts.AddRange(Quad(P(-8.5f, -11), P(-9, -20), P(2, -19.5f), 8));
            pts.AddRange(Quad(P(2, -19.5f), P(8, -19), P(7.5f, -13), 8));
            pts.AddRange(Quad(P(7.5f, -13), P(3, -15), P(-4, -13), 8));
            pts.AddRange(Quad(P(-4, -13), P(-7, -11), P(-8.5f, -11), 8));
            return pts.ToArray();
        }

        private static Vector2[] Quad(Vector2 a, Vector2 c, Vector2 b, int segments)
        {
            var pts = new Vector2[segments + 1];
            for (int i = 0; i <= segments; i++)
            {
                float t = i / (float)segments;
                pts[i] = (1 - t) * (1 - t) * a + 2 * (1 - t) * t * c + t * t * b;
            }
            return pts;
        }

        private static Transform Child(Transform parent, string name, Vector2 localPos)
        {
            var go = new GameObject(name);
            go.transform.SetParent(parent, false);
            go.transform.localPosition = localPos;
            return go.transform;
        }

        private SpriteRenderer Part(Transform parent, Sprite sprite, Vector2 pos, Color color)
        {
            var go = new GameObject("part");
            go.transform.SetParent(parent, false);
            go.transform.localPosition = pos;
            var sr = go.AddComponent<SpriteRenderer>();
            sr.sprite = sprite;
            sr.color = color;
            sr.sortingOrder = _order++;
            return sr;
        }

        private void Circle(Transform parent, Vector2 pos, float radius, Color fill, float stroke = Stroke)
        {
            Ellipse(parent, pos, new Vector2(radius, radius), fill, stroke);
        }

        private void Ellipse(Transform parent, Vector2 pos, Vector2 radii, Color fill, float stroke)
        {
            if (stroke > 0f)
            {
                Part(parent, Shapes.Circle, pos, Ink).transform.localScale = (radii + Vector2.one * stroke) * 2f;
            }
            Part(parent, Shapes.Circle, pos, fill).transform.localScale = radii * 2f;
        }

        private void Rect(Transform parent, Vector2 pos, Vector2 size, float radius, Color fill, bool outlined)
        {
            if (outlined)
            {
                SlicedPart(parent, pos, size + Vector2.one * Stroke * 2f, radius + Stroke, Ink);
            }
            SlicedPart(parent, pos, size, radius, fill);
        }

        private void SlicedPart(Transform parent, Vector2 pos, Vector2 size, float radius, Color color)
        {
            var sr = Part(parent, Shapes.RoundedRect(Mathf.Max(radius, 0.1f)), pos, color);
            sr.drawMode = SpriteDrawMode.Sliced;
            sr.size = size;
        }

        private void Poly(Transform parent, string key, Color color, params Vector2[] points)
        {
            Part(parent, Shapes.Polygon(key, points), Vector2.zero, color);
        }

        private void Line(Transform parent, Color color, float width, params Vector2[] points)
        {
            for (int i = 0; i < points.Length - 1; i++)
            {
                Segment(parent, points[i], points[i + 1], width, color);
            }
        }

        private void Arc(Transform parent, Vector2 center, float radius, float fromDeg, float toDeg, float width, Color color)
        {
            const int segments = 8;
            var pts = new Vector2[segments + 1];
            for (int i = 0; i <= segments; i++)
            {
                float a = Mathf.Deg2Rad * Mathf.Lerp(fromDeg, toDeg, i / (float)segments);
                pts[i] = center + new Vector2(Mathf.Cos(a), Mathf.Sin(a)) * radius;
            }
            Line(parent, color, width, pts);
        }

        private void Segment(Transform parent, Vector2 a, Vector2 b, float width, Color color)
        {
            var dir = b - a;
            var sr = Part(parent, Shapes.Square, (a + b) / 2f, color);
            sr.transform.localScale = new Vector3(dir.magnitude + width * 0.5f, width, 1f);
            sr.transform.localRotation = Quaternion.Euler(0f, 0f, Mathf.Atan2(dir.y, dir.x) * Mathf.Rad2Deg);
        }

        private static Color Hex(string hex)
        {
            ColorUtility.TryParseHtmlString(hex, out var c);
            return c;
        }
    }
}
