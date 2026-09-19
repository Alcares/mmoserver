using System.IO;
using Game.Client;
using UnityEditor;
using UnityEngine;

namespace Game.EditorTools
{
    /// <summary>Forces texture settings that suit runtime-cut sprite sheets on everything in the art folder.</summary>
    public class PlayerArtImporter : AssetPostprocessor
    {
        private void OnPreprocessTexture()
        {
            if (!assetPath.StartsWith(PlayerArtTools.Folder + "/")) return;

            var importer = (TextureImporter)assetImporter;
            importer.textureType = TextureImporterType.Default;
            importer.alphaIsTransparency = true;
            importer.mipmapEnabled = false;
            importer.npotScale = TextureImporterNPOTScale.None;
            importer.textureCompression = TextureImporterCompression.Uncompressed;
            importer.wrapMode = TextureWrapMode.Clamp;
            importer.maxTextureSize = 8192;
        }
    }

    public static class PlayerArtTools
    {
        public const string Folder = "Assets/Art/Player";

        private const int Cell = 128;
        private const int IdleFrames = 4;
        private const int WalkFrames = 6;

        [MenuItem("Game/Player Art/Create Template Sheets And Asset")]
        public static void CreateTemplates()
        {
            Directory.CreateDirectory(Folder);

            var idle = WriteSheet("idle", IdleFrames, false);
            var walk = WriteSheet("walk", WalkFrames, true);

            const string assetPath = Folder + "/PlayerArt.asset";
            var set = AssetDatabase.LoadAssetAtPath<DirectionalAnimationSet>(assetPath);
            if (set == null)
            {
                set = ScriptableObject.CreateInstance<DirectionalAnimationSet>();
                AssetDatabase.CreateAsset(set, assetPath);
            }

            set.idle.sheet = idle;
            set.idle.frames = IdleFrames;
            set.walk.sheet = walk;
            set.walk.frames = WalkFrames;
            EditorUtility.SetDirty(set);
            AssetDatabase.SaveAssets();
            Debug.Log($"Player art set ready at {assetPath}. Assign it to WorldView > Player Art.");
        }

        // Existing sheets are never overwritten: that slot is where the real art goes.
        private static Texture2D WriteSheet(string name, int frames, bool walking)
        {
            string path = $"{Folder}/{name}.png";
            if (!File.Exists(path))
            {
                int w = Cell * frames, h = Cell * DirectionalAnimationSet.DirectionCount;
                var pixels = new Color32[w * h];
                for (int row = 0; row < DirectionalAnimationSet.DirectionCount; row++)
                {
                    for (int frame = 0; frame < frames; frame++)
                    {
                        DrawCell(pixels, w, h, frame * Cell, h - (row + 1) * Cell, row, frame, walking);
                    }
                }

                var tex = new Texture2D(w, h, TextureFormat.RGBA32, false);
                tex.SetPixels32(pixels);
                File.WriteAllBytes(path, tex.EncodeToPNG());
                Object.DestroyImmediate(tex);
                AssetDatabase.ImportAsset(path);
            }
            else
            {
                Debug.Log($"{path} already exists; left untouched.");
            }

            return AssetDatabase.LoadAssetAtPath<Texture2D>(path);
        }

        // A placeholder figure with an arrow on the ground showing which way the row faces.
        private static void DrawCell(Color32[] px, int sheetW, int sheetH, int x0, int y0, int direction, int frame, bool walking)
        {
            float angle = (-90f - 45f * direction) * Mathf.Deg2Rad;
            var facing = new Vector2(Mathf.Cos(angle), Mathf.Sin(angle));

            float bob = walking ? Mathf.Abs(Mathf.Sin(frame / (float)WalkFrames * Mathf.PI * 2f)) * 4f : Mathf.Sin(frame / (float)IdleFrames * Mathf.PI * 2f) * 1.5f;
            float step = walking ? Mathf.Sin(frame / (float)WalkFrames * Mathf.PI * 2f) * 7f : 0f;
            var tint = Color.HSVToRGB(direction / 8f, 0.5f, 1f);

            for (int y = 0; y < Cell; y++)
            {
                for (int x = 0; x < Cell; x++)
                {
                    var p = new Vector2(x + 0.5f, y + 0.5f);
                    var c = new Color(tint.r, tint.g, tint.b, 0.12f);

                    Blend(ref c, new Color(0, 0, 0, 0.3f), Ellipse(p, new Vector2(64f, 14f), new Vector2(28f, 8f)));
                    Blend(ref c, new Color(0.06f, 0.09f, 0.2f), Box(p, new Vector2(64f - 9f, 26f + Mathf.Max(0f, step)), new Vector2(6f, 12f)));
                    Blend(ref c, new Color(0.06f, 0.09f, 0.2f), Box(p, new Vector2(64f + 9f, 26f + Mathf.Max(0f, -step)), new Vector2(6f, 12f)));
                    Blend(ref c, new Color32(0x1e, 0x3a, 0x8a, 255), Ellipse(p, new Vector2(64f, 58f + bob), new Vector2(24f, 28f)));
                    Blend(ref c, new Color32(0xff, 0xd8, 0xc4, 255), Ellipse(p, new Vector2(64f, 100f + bob), new Vector2(17f, 17f)));

                    if (facing.y < 0.5f)
                    {
                        var nose = new Vector2(64f + facing.x * 10f, 98f + bob + facing.y * 6f);
                        Blend(ref c, Color.black, Ellipse(p, nose, new Vector2(5.5f, 5.5f)));
                        Blend(ref c, Color.white, Ellipse(p, nose, new Vector2(3.5f, 3.5f)));
                    }

                    for (int i = 1; i <= 6; i++)
                    {
                        float t = i / 6f;
                        var dot = new Vector2(64f + facing.x * t * 44f, 14f + facing.y * t * 18f);
                        Blend(ref c, new Color(1f, 0.85f, 0.1f), Ellipse(p, dot, Vector2.one * (2.5f + t * 3.5f)));
                    }

                    for (int i = 0; i <= frame; i++)
                    {
                        Blend(ref c, new Color(0.1f, 0.1f, 0.1f), Box(p, new Vector2(10f + i * 9f, 6f), new Vector2(3f, 3f)));
                    }

                    if (x < 2 || y < 2 || x >= Cell - 2 || y >= Cell - 2) Blend(ref c, new Color(0f, 0.8f, 1f, 0.8f), 1f);

                    px[(y0 + y) * sheetW + x0 + x] = c;
                }
            }
        }

        private static float Ellipse(Vector2 p, Vector2 center, Vector2 radii)
        {
            float d = ((p - center) / radii).magnitude - 1f;
            return Mathf.Clamp01(0.5f - d * Mathf.Min(radii.x, radii.y));
        }

        private static float Box(Vector2 p, Vector2 center, Vector2 half)
        {
            var q = new Vector2(Mathf.Abs(p.x - center.x) - half.x, Mathf.Abs(p.y - center.y) - half.y);
            float d = Vector2.Max(q, Vector2.zero).magnitude + Mathf.Min(Mathf.Max(q.x, q.y), 0f);
            return Mathf.Clamp01(0.5f - d);
        }

        private static void Blend(ref Color dst, Color src, float coverage)
        {
            float a = src.a * coverage;
            float outA = a + dst.a * (1f - a);
            if (outA <= 0f) return;
            dst = new Color(
                (src.r * a + dst.r * dst.a * (1f - a)) / outA,
                (src.g * a + dst.g * dst.a * (1f - a)) / outA,
                (src.b * a + dst.b * dst.a * (1f - a)) / outA,
                outA);
        }
    }
}
