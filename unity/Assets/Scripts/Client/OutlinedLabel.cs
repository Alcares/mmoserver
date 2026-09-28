using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// World-space text with a dark outline (TextMesh has none, so the outline is
    /// eight offset copies behind the fill). Uses the built-in font unless given another; no TMP assets needed.
    /// </summary>
    public class OutlinedLabel : MonoBehaviour
    {
        private const float OutlineOffset = 0.0525f;
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

        /// <param name="font">A display font to draw with instead of the built-in one, which is emboldened.</param>
        /// <param name="anchor">Which point of the text sits at localPos; the default stacks it upwards from there.</param>
        /// <param name="outline">How far the outline reaches past the fill, in the label's local units.</param>
        public static OutlinedLabel Create(Transform parent, Vector2 localPos, Color fill, int sortingOrder,
            Font font = null, TextAnchor anchor = TextAnchor.LowerCenter, float outline = OutlineOffset)
        {
            if (_font == null) _font = Resources.GetBuiltinResource<Font>("LegacyRuntime.ttf");
            if (font == null) font = _font;

            var go = new GameObject("Label");
            go.transform.SetParent(parent, false);
            go.transform.localPosition = localPos;

            var label = go.AddComponent<OutlinedLabel>();
            // A copy every 45 degrees around the fill, so a thick outline has no notches at the diagonals
            var offsets = new Vector2[9];
            for (int i = 0; i < 8; i++)
            {
                float angle = i * Mathf.PI / 4f;
                offsets[i] = new Vector2(Mathf.Cos(angle), Mathf.Sin(angle)) * outline;
            }
            offsets[8] = Vector2.zero; // fill last so it draws on top

            label._meshes = new TextMesh[offsets.Length];
            for (int i = 0; i < offsets.Length; i++)
            {
                var child = new GameObject("Text");
                child.transform.SetParent(go.transform, false);
                child.transform.localPosition = offsets[i];

                var mesh = child.AddComponent<TextMesh>();
                mesh.font = font;
                mesh.fontStyle = font == _font ? FontStyle.Bold : FontStyle.Normal;
                mesh.fontSize = 64;
                mesh.characterSize = 0.09f;
                mesh.anchor = anchor;
                mesh.alignment = TextAlignment.Center;
                mesh.color = i == offsets.Length - 1 ? fill : OutlineColor;

                var r = child.GetComponent<MeshRenderer>();
                r.sharedMaterial = font.material;
                r.sortingOrder = sortingOrder + i;

                label._meshes[i] = mesh;
            }

            return label;
        }
    }
}
