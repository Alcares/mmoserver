using UnityEngine;

namespace Game.Client
{
    /// <summary>Visual for one player. Implementations own their sprites and sort order.</summary>
    public abstract class PlayerAvatar : MonoBehaviour
    {
        /// <summary>Height above the player's position where the name label sits.</summary>
        public abstract float LabelHeight { get; }

        /// <summary>
        /// direction is the world-space (y up) movement direction. It is only meaningful
        /// while moving; implementations keep their last facing when it is near zero.
        /// </summary>
        public abstract void Animate(Vector2 direction, bool moving);

        public abstract void SetSortingOrder(int order);
    }
}
