using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// Loops a music track while the training spectator is replaying, and stops it otherwise.
    /// </summary>
    [RequireComponent(typeof(GameState))]
    public class SpectatorMusic : MonoBehaviour
    {
        [Tooltip("Looped for as long as this client is spectating training.")]
        [SerializeField] private AudioClip music;

        private GameState _state;
        private AudioSource _source;

        private void Awake()
        {
            _state = GetComponent<GameState>();
            _source = gameObject.AddComponent<AudioSource>();
            _source.clip = music;
            _source.loop = true;
            _source.playOnAwake = false;
        }

        private void OnEnable() => _state.SessionChanged += Refresh;

        private void OnDisable() => _state.SessionChanged -= Refresh;

        // Only the spectator reports a playback speed, so that is what marks spectating; the
        // recap-over screen that follows the last snapshot plays in silence.
        private void Refresh()
        {
            bool spectating = _state.PlaybackSpeed.HasValue && !_state.SpectatingOver;
            if (spectating && !_source.isPlaying && music != null) _source.Play();
            else if (!spectating && _source.isPlaying) _source.Stop();
        }
    }
}
