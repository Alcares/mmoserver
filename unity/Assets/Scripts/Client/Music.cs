using Game.V1;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// Loops one track while a game waits for players, plays a spoken countdown once as it counts
    /// down, loops another once it runs, and a third while the training spectator is replaying.
    /// The main menu and the recap-over screen are silent.
    /// </summary>
    [RequireComponent(typeof(GameState))]
    public class Music : MonoBehaviour
    {
        [Tooltip("Looped while a joined game waits for players.")]
        [SerializeField] private AudioClip lobbyTrack;
        [Tooltip("Played once when the countdown starts; the game track cuts it off.")]
        [SerializeField] private AudioClip countdownTrack;
        [Tooltip("Looped once the round is running, through game over.")]
        [SerializeField] private AudioClip gameTrack;
        [Tooltip("Looped while spectating training, until the last snapshot has played.")]
        [SerializeField] private AudioClip spectatorTrack;
        [Tooltip("Every track plays at this volume, 0 to 1, except the lobby track at half of it.")]
        [Range(0f, 1f)]
        [SerializeField] private float volume = 0.5f;

        private GameState _state;
        private AudioSource _source;

        private void Awake()
        {
            _state = GetComponent<GameState>();
            _source = gameObject.AddComponent<AudioSource>();
            _source.playOnAwake = false;
        }

        private void OnEnable() => _state.SessionChanged += Refresh;

        private void OnDisable() => _state.SessionChanged -= Refresh;

        // Only the spectator reports a playback speed, so that is what tells a spectator apart.
        // The phase is still Unspecified between joining and the first GameStatus, which counts
        // as the lobby too.
        private AudioClip Wanted()
        {
            if (!_state.Joined) return null;
            if (_state.PlaybackSpeed.HasValue) return _state.SpectatingOver ? null : spectatorTrack;
            return _state.Phase switch
            {
                GamePhase.Countdown => countdownTrack,
                GamePhase.Running or GamePhase.Finished => gameTrack,
                _ => lobbyTrack,
            };
        }

        // Starts only when the track changes, so a status update doesn't rewind a track or replay
        // the countdown after it has finished.
        private void Refresh()
        {
            var wanted = Wanted();
            if (_source.clip == wanted) return;

            _source.Stop();
            _source.clip = wanted;
            _source.loop = wanted != countdownTrack;
            _source.volume = wanted == lobbyTrack ? volume * 0.5f : volume;
            if (wanted != null) _source.Play();
        }
    }
}
