using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// Loops one track while in a game and another while the training spectator is replaying.
    /// The lobby and the recap-over screen are silent.
    /// </summary>
    [RequireComponent(typeof(GameState))]
    public class Music : MonoBehaviour
    {
        [Tooltip("Looped while in a game on the game server, from the lobby through game over.")]
        [SerializeField] private AudioClip gameTrack;
        [Tooltip("Looped while spectating training, until the last snapshot has played.")]
        [SerializeField] private AudioClip spectatorTrack;

        private GameState _state;
        private AudioSource _source;

        private void Awake()
        {
            _state = GetComponent<GameState>();
            _source = gameObject.AddComponent<AudioSource>();
            _source.loop = true;
            _source.playOnAwake = false;
        }

        private void OnEnable() => _state.SessionChanged += Refresh;

        private void OnDisable() => _state.SessionChanged -= Refresh;

        // Only the spectator reports a playback speed, so that is what tells the two apart.
        private AudioClip Wanted()
        {
            if (!_state.Joined) return null;
            if (!_state.PlaybackSpeed.HasValue) return gameTrack;
            return _state.SpectatingOver ? null : spectatorTrack;
        }

        // Restarts only when the track changes, so a status update mid-game doesn't rewind it.
        private void Refresh()
        {
            var wanted = Wanted();
            if (_source.clip == wanted && _source.isPlaying == (wanted != null)) return;

            _source.Stop();
            _source.clip = wanted;
            if (wanted != null) _source.Play();
        }
    }
}
