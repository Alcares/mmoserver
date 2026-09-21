using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// World-space text with a dark outline (TextMesh has none, so the outline is
    /// four offset copies behind the fill). Uses the built-in font; no TMP assets needed.
    /// </summary>
    public class OutlinedLabel : MonoBehaviour
    {
        private const float OutlineOffset = 0.035f;
        private static readonly Color OutlineColor = new(15f / 255f, 23f / 255f, 42f / 255f, 0.9f);
        private static Font _font;

        private TextMesh[] _meshes;

        public string Text
        {
            set
            {
                foreach (var m in _meshes) m.text = value;
            }
        }

        public static OutlinedLabel Create(Transform parent, Vector2 localPos, Color fill, int sortingOrder)
        {
            if (_font == null) _font = Resources.GetBuiltinResource<Font>("LegacyRuntime.ttf");

            var go = new GameObject("Label");
            go.transform.SetParent(parent, false);
            go.transform.localPosition = localPos;

            var label = go.AddComponent<OutlinedLabel>();
            var offsets = new[]
            {
                new Vector2(OutlineOffset, 0f), new Vector2(-OutlineOffset, 0f),
                new Vector2(0f, OutlineOffset), new Vector2(0f, -OutlineOffset),
                Vector2.zero, // fill last so it draws on top
            };

            label._meshes = new TextMesh[offsets.Length];
            for (int i = 0; i < offsets.Length; i++)
            {
                var child = new GameObject("Text");
                child.transform.SetParent(go.transform, false);
                child.transform.localPosition = offsets[i];

                var mesh = child.AddComponent<TextMesh>();
                mesh.font = _font;
                mesh.fontStyle = FontStyle.Bold;
                mesh.fontSize = 64;
                mesh.characterSize = 0.06f;
                mesh.anchor = TextAnchor.LowerCenter;
                mesh.alignment = TextAlignment.Center;
                mesh.color = i == offsets.Length - 1 ? fill : OutlineColor;

                var r = child.GetComponent<MeshRenderer>();
                r.sharedMaterial = _font.material;
                r.sortingOrder = sortingOrder + i;

                label._meshes[i] = mesh;
            }

            return label;
        }
    }
}
