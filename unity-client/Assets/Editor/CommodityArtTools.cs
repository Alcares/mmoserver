using System;
using System.Collections.Generic;
using System.IO;
using Game.Client;
using Game.V1;
using UnityEditor;
using UnityEngine;
using static Game.EditorTools.IconCanvas;

namespace Game.EditorTools
{
    /// <summary>Imports everything in the commodity art folder as a sprite.</summary>
    public class CommodityArtImporter : AssetPostprocessor
    {
        // Bump when the settings below change so existing icons are reimported.
        public override uint GetVersion() => 2;

        private void OnPreprocessTexture()
        {
            if (!assetPath.StartsWith(CommodityArtTools.Folder + "/")) return;

            var importer = (TextureImporter)assetImporter;
            importer.textureType = TextureImporterType.Sprite;
            importer.spriteImportMode = SpriteImportMode.Single;
            importer.spritePixelsPerUnit = 100f;
            importer.isReadable = true; // the HUD builds grayscale copies for commodities you don't hold
            importer.alphaIsTransparency = true;
            importer.mipmapEnabled = false;
            importer.wrapMode = TextureWrapMode.Clamp;
            importer.textureCompression = TextureImporterCompression.Uncompressed;
        }
    }

    /// <summary>
    /// Draws one flat, outlined icon per commodity into PNGs, and wires them into a CommodityIcons
    /// asset. Existing PNGs are never overwritten by the menu item, so real art dropped over them
    /// (same file names) survives.
    /// </summary>
    public static class CommodityArtTools
    {
        public const string Folder = "Assets/Art/Commodities";
        private const string AssetPath = Folder + "/CommodityIcons.asset";

        private static readonly (CommodityType type, string file, Func<IconCanvas> draw)[] Icons =
        {
            (CommodityType.CommodityOil, "oil", Oil),
            (CommodityType.CommodityWheat, "wheat", Wheat),
            (CommodityType.CommodityGold, "gold", Gold),
            (CommodityType.CommoditySilver, "silver", Silver),
            (CommodityType.CommodityCoffee, "coffee", Coffee),
            (CommodityType.CommodityLithium, "lithium", Lithium),
        };

        [MenuItem("Game/Commodity Art/Create Missing Icons")]
        public static void CreateMissing() => Generate(false);

        public static void Generate(bool overwrite)
        {
            Directory.CreateDirectory(Folder);

            foreach (var (_, file, draw) in Icons)
            {
                string path = $"{Folder}/{file}.png";
                if (overwrite || !File.Exists(path))
                {
                    draw().Save(path);
                    AssetDatabase.ImportAsset(path);
                }
            }

            var set = AssetDatabase.LoadAssetAtPath<CommodityIcons>(AssetPath);
            if (set == null)
            {
                set = ScriptableObject.CreateInstance<CommodityIcons>();
                AssetDatabase.CreateAsset(set, AssetPath);
            }

            var entries = new List<CommodityIcons.Entry>();
            foreach (var (type, file, _) in Icons)
            {
                entries.Add(new CommodityIcons.Entry
                {
                    commodity = type,
                    sprite = AssetDatabase.LoadAssetAtPath<Sprite>($"{Folder}/{file}.png"),
                });
            }
            set.entries = entries.ToArray();
            EditorUtility.SetDirty(set);
            AssetDatabase.SaveAssets();
        }

        // ---- shared pieces -------------------------------------------------------------

        private static void Shadow(IconCanvas c)
        {
            c.Draw(Ellipse(P(128f, 30f), P(96f, 13f)), new Color(0f, 0f, 0f, 0.22f));
        }

        private struct Metal
        {
            public Color FrontTop, FrontBottom, Top, Shine, Stamp;
        }

        private static void Ingot(IconCanvas c, float cx, float baseY, float w, float h, Metal m)
        {
            float hw = w / 2f, inset = 10f, depth = 15f;
            c.Draw(Poly(P(cx - hw, baseY), P(cx + hw, baseY), P(cx + hw - inset, baseY + h), P(cx - hw + inset, baseY + h)),
                m.FrontTop, m.FrontBottom, 4f);
            c.Draw(Poly(P(cx - hw + inset, baseY + h), P(cx + hw - inset, baseY + h),
                    P(cx + hw - inset - 13f, baseY + h + depth), P(cx - hw + inset + 13f, baseY + h + depth)),
                m.Top, 4f);
            c.Draw(Box(P(cx + hw * 0.32f, baseY + h * 0.42f), P(15f, 7f), 3f), m.Stamp);
            c.Draw(Segment(P(cx - hw + 22f, baseY + h - 11f), P(cx - hw + 52f, baseY + h - 11f), 3f), m.Shine);
        }

        private static void Sparkle(IconCanvas c, float x, float y, float r)
        {
            float k = r * 0.28f;
            c.Draw(Poly(P(x, y + r), P(x + k, y + k), P(x + r, y), P(x + k, y - k),
                P(x, y - r), P(x - k, y - k), P(x - r, y), P(x - k, y + k)), Color.white, 2f);
        }

        // ---- oil: a couple of barrels --------------------------------------------------

        private static void Barrel(IconCanvas c, float cx, float baseY, float w, float h, Color label, bool drop)
        {
            float half = w / 2f, bodyH = h - 8f;
            c.Draw(Box(P(cx, baseY + bodyH / 2f), P(half, bodyH / 2f), 16f), Hex("#3f4d63"), Hex("#232b38"), 4f);

            // Highlight stripe on the left, then the label band and the steel hoops on top of it.
            c.Draw(Box(P(cx - half * 0.5f, baseY + bodyH / 2f), P(5f, bodyH / 2f - 22f), 5f), new Color(1f, 1f, 1f, 0.16f));
            c.Draw(Box(P(cx, baseY + bodyH * 0.5f), P(half, 19f), 3f), label, 3.5f);
            if (drop)
            {
                float dy = baseY + bodyH * 0.5f - 3f;
                c.Draw(Poly(P(cx - 9.5f, dy + 1f), P(cx + 9.5f, dy + 1f), P(cx, dy + 24f)), Ink);
                c.Draw(Ellipse(P(cx, dy - 1f), P(10f, 10f)), Ink);
            }
            foreach (float y in new[] { baseY + 14f, baseY + bodyH - 20f })
            {
                c.Draw(Box(P(cx, y), P(half + 3f, 6.5f), 5f), Hex("#a3aebf"), Hex("#6b788d"), 3.5f);
            }

            c.Draw(Ellipse(P(cx, baseY + bodyH), P(half - 2f, 13f)), Hex("#8794aa"), 3.5f);
            c.Draw(Ellipse(P(cx, baseY + bodyH), P(half - 14f, 7f)), Hex("#4d5a70"));
            c.Draw(Ellipse(P(cx + half * 0.38f, baseY + bodyH + 1f), P(5f, 2.6f)), Hex("#262e3b"));
        }

        private static IconCanvas Oil()
        {
            var c = new IconCanvas();
            Shadow(c);
            Barrel(c, 88f, 48f, 86f, 122f, Hex("#d9503f"), false);
            Barrel(c, 166f, 30f, 94f, 136f, Hex("#f2b51d"), true);
            c.Offset = P(0f, 30f);
            return c;
        }

        // ---- wheat: a bushel basket of stalks ------------------------------------------

        private static IconCanvas Wheat()
        {
            var c = new IconCanvas();
            Shadow(c);

            var stem = Hex("#c99a3a");
            var grain = Hex("#f2c44a");
            foreach (float a in new[] { -34f, 34f, -22f, 22f, -11f, 11f, 0f })
            {
                float rad = a * Mathf.Deg2Rad;
                var dir = P(Mathf.Sin(rad), Mathf.Cos(rad));
                var perp = P(dir.y, -dir.x);
                var start = P(128f + dir.x * 10f, 104f);
                float len = 122f - Mathf.Abs(a) * 0.5f;
                var tip = start + dir * len;

                c.Draw(Segment(start, tip, 2.6f), stem, 2.4f);
                for (int i = 0; i < 6; i++)
                {
                    var along = start + dir * (len - 50f + i * 9f);
                    foreach (int side in new[] { -1, 1 })
                    {
                        c.Draw(Ellipse(along + perp * (side * 6.5f), P(4.8f, 10.5f), -a - side * 30f), grain, 2.2f);
                    }
                }
                c.Draw(Ellipse(tip + dir * 3f, P(4.8f, 11f), -a), grain, 2.2f);
            }

            var basket = Poly(P(76f, 40f), P(180f, 40f), P(200f, 108f), P(56f, 108f));
            c.Draw(basket, Hex("#dca85f"), Hex("#a9722f"), 4.5f);
            foreach (float y in new[] { 58f, 76f, 94f })
            {
                float grow = (y - 40f) / 68f * 20f;
                c.Draw(Segment(P(76f - grow + 5f, y), P(180f + grow - 5f, y), 2.6f), Hex("#8e5f26"));
            }
            for (int i = 0; i < 6; i++)
            {
                float x = 74f + i * 21f;
                c.Draw(Segment(P(x, 45f), P(x + (i - 2.5f) * 3.5f, 104f), 1.6f), HexAlpha("#8e5f26", 0.6f));
            }
            c.Draw(Box(P(128f, 110f), P(82f, 9f), 6f), Hex("#b98240"), 4.5f);

            c.Offset = P(0f, -5f);
            return c;
        }

        private static Color HexAlpha(string hex, float alpha)
        {
            var col = Hex(hex);
            col.a = alpha;
            return col;
        }

        // ---- gold and silver: ingots ---------------------------------------------------

        private static IconCanvas Gold()
        {
            var c = new IconCanvas();
            Shadow(c);
            var m = new Metal
            {
                FrontTop = Hex("#f3b92e"), FrontBottom = Hex("#c98a0a"), Top = Hex("#ffe37e"),
                Shine = Hex("#fff7cf"), Stamp = Hex("#a86f06"),
            };
            Ingot(c, 76f, 36f, 98f, 44f, m);
            Ingot(c, 180f, 36f, 98f, 44f, m);
            Ingot(c, 128f, 88f, 98f, 44f, m);
            c.Offset = P(0f, 47f);
            return c;
        }

        private static IconCanvas Silver()
        {
            var c = new IconCanvas();
            Shadow(c);
            var m = new Metal
            {
                FrontTop = Hex("#d3dae4"), FrontBottom = Hex("#96a2b3"), Top = Hex("#f4f7fb"),
                Shine = Color.white, Stamp = Hex("#7a8797"),
            };
            Ingot(c, 118f, 36f, 124f, 48f, m);
            Ingot(c, 138f, 92f, 100f, 42f, m);
            Sparkle(c, 208f, 168f, 15f);
            c.Offset = P(0f, 44f);
            return c;
        }

        // ---- coffee: a burlap sack with a bean -----------------------------------------

        private static void Bean(IconCanvas c, Vector2 pos, float scale, float rot)
        {
            c.Draw(Ellipse(pos, P(22f, 30f) * scale, rot), Hex("#6a3d1c"), Hex("#4a2710"), 3.5f * scale);
            var a = pos + Rot(P(-4f, 24f) * scale, rot);
            var b = pos + Rot(P(6f, 0f) * scale, rot);
            var d = pos + Rot(P(-6f, -24f) * scale, rot);
            c.Draw(Segment(a, b, 2.6f * scale), Hex("#c98b52"));
            c.Draw(Segment(b, d, 2.6f * scale), Hex("#c98b52"));
        }

        private static Vector2 Rot(Vector2 v, float deg)
        {
            float r = deg * Mathf.Deg2Rad, cs = Mathf.Cos(r), sn = Mathf.Sin(r);
            return new Vector2(v.x * cs - v.y * sn, v.x * sn + v.y * cs);
        }

        private static IconCanvas Coffee()
        {
            var c = new IconCanvas();
            Shadow(c);

            var sackTop = Hex("#dcb679");
            var sackBottom = Hex("#b98b4c");
            c.Draw(Poly(P(96f, 176f), P(110f, 164f), P(118f, 178f), P(128f, 164f), P(138f, 178f), P(146f, 164f), P(160f, 176f),
                P(154f, 140f), P(102f, 140f)), sackTop, sackBottom, 4f);
            c.Draw(Box(P(128f, 92f), P(68f, 62f), 36f), sackTop, sackBottom, 4.5f);
            c.Draw(Box(P(128f, 141f), P(32f, 7f), 4f), Hex("#7a4a1e"), 3.5f);

            c.Draw(Box(P(128f, 90f), P(38f, 44f), 10f), Hex("#efe0b8"), Hex("#d9c58f"), 2.5f);
            Bean(c, P(128f, 90f), 0.95f, 24f);

            Bean(c, P(190f, 52f), 0.5f, 55f);
            Bean(c, P(214f, 74f), 0.42f, -25f);

            c.Offset = P(0f, 30f);
            return c;
        }

        // ---- lithium: a crystal cluster ------------------------------------------------

        private static void Crystal(IconCanvas c, float cx, float baseY, float w, float h)
        {
            float hw = w / 2f, shoulder = baseY + h * 0.72f;
            var outer = Poly(P(cx - hw, baseY), P(cx + hw, baseY), P(cx + hw, shoulder), P(cx, baseY + h), P(cx - hw, shoulder));
            c.Draw(outer, Hex("#b78cf5"), 4f);
            c.Draw(Poly(P(cx - hw + 1f, baseY + 1f), P(cx, baseY + 1f), P(cx, baseY + h - 1f), P(cx - hw + 1f, shoulder - 0.5f)),
                Hex("#e0c8ff"), Hex("#c39bf8"));
            c.Draw(Poly(P(cx, baseY + 1f), P(cx + hw - 1f, baseY + 1f), P(cx + hw - 1f, shoulder - 0.5f), P(cx, baseY + h - 1f)),
                Hex("#9268d8"), Hex("#7548bd"));
            c.Draw(Segment(P(cx, baseY + 2f), P(cx, baseY + h - 2f), 1.4f), Ink);
            c.Draw(Segment(P(cx - hw * 0.55f, baseY + h * 0.3f), P(cx - hw * 0.55f, baseY + h * 0.62f), 2.6f), new Color(1f, 1f, 1f, 0.7f));
        }

        private static IconCanvas Lithium()
        {
            var c = new IconCanvas();
            Shadow(c);
            Crystal(c, 128f, 40f, 70f, 158f);
            Crystal(c, 78f, 32f, 56f, 104f);
            Crystal(c, 182f, 34f, 60f, 122f);
            Crystal(c, 224f, 30f, 30f, 54f);
            Sparkle(c, 52f, 158f, 13f);
            Sparkle(c, 214f, 178f, 10f);
            c.Offset = P(0f, 21f);
            return c;
        }
    }
}
