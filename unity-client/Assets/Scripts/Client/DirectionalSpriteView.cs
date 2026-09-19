using UnityEngine;

namespace Game.Client
{
    /// <summary>Plays an 8-direction idle/walk animation from a DirectionalAnimationSet.</summary>
    public class DirectionalSpriteView : PlayerAvatar
    {
        // Stay on the current facing until the movement angle is this far from its center
        // (sectors are 45 wide, so 27.5 gives a 5 degree dead zone that stops diagonal flicker).
        private const float SwitchAngle = 27.5f;
        private const float MinDirectionSqr = 0.0025f;

        private DirectionalAnimationSet _set;
        private SpriteRenderer _renderer;
        private int _direction; // 0 = S, facing the camera
        private bool _walking;
        private float _clock;

        public override float LabelHeight => _set.labelHeight;

        public void Build(DirectionalAnimationSet set)
        {
            _set = set;
            var go = new GameObject("Sprite");
            go.transform.SetParent(transform, false);
            _renderer = go.AddComponent<SpriteRenderer>();
            Animate(Vector2.zero, false);
        }

        public override void SetSortingOrder(int order)
        {
            _renderer.sortingOrder = order;
        }

        public override void Animate(Vector2 direction, bool moving)
        {
            if (direction.sqrMagnitude > MinDirectionSqr) _direction = Quantize(direction, _direction);

            if (moving != _walking)
            {
                _walking = moving;
                _clock = 0f;
            }
            _clock += Time.deltaTime;

            var clip = _walking ? _set.walk : _set.idle;
            int frame = clip.sheet == null ? 0 : (int)(_clock * clip.fps) % Mathf.Max(1, clip.frames);
            var sprite = _set.GetSprite(_walking, _direction, frame, out bool flip);
            if (sprite == null) return;

            _renderer.sprite = sprite;
            _renderer.flipX = flip;
        }

        /// <summary>Direction index 0..7 = S, SW, W, NW, N, NE, E, SE (clockwise from south).</summary>
        public static int Quantize(Vector2 direction, int current)
        {
            float angle = Mathf.Atan2(direction.y, direction.x) * Mathf.Rad2Deg;
            if (Mathf.Abs(Mathf.DeltaAngle(angle, CenterAngle(current))) < SwitchAngle) return current;

            int index = Mathf.RoundToInt((-90f - angle) / 45f);
            return ((index % 8) + 8) % 8;
        }

        private static float CenterAngle(int direction) => -90f - 45f * direction;
    }
}
