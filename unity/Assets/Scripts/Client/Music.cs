using Game.V1;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// Loops one track while a game waits for players, another once it runs, and a third while the
    /// training spectator is replaying. The main menu, the countdown and the recap-over screen are
    /// silent.
    /// </summary>
    [RequireComponent(typeof(GameState))]
    public class Music : MonoBehaviour
    {
        [Tooltip("Looped while a joined game waits for players; stops for the countdown.")]
        [SerializeField] private AudioClip lobbyTrack;
        [Tooltip("Looped once the round is running, through game over.")]
        [SerializeField] private AudioClip gameTrack;
        [Tooltip("Looped while spectating training, until the last snapshot has played.")]
        [SerializeField] private AudioClip spectatorTrack;
        [Tooltip("The game track plays at this volume, 0 to 1; the lobby track at half of it and the spectator track at 0.6 of it.")]
        [Range(0f, 1f)]
        [SerializeField] private float volume = 0.5f;

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

        // Only the spectator reports a playback speed, so that is what tells a spectator apart.
        // The phase is still Unspecified between joining and the first GameStatus, which counts
        // as the lobby too.
        private AudioClip Wanted()
        {
            if (!_state.Joined) return null;
            if (_state.PlaybackSpeed.HasValue) return _state.SpectatingOver ? null : spectatorTrack;
            return _state.Phase switch
            {
                GamePhase.Countdown => null, // the countdown beeps play alone
                GamePhase.Running or GamePhase.Finished => gameTrack,
                _ => lobbyTrack,
            };
        }

        // Starts only when the track changes, so a status update doesn't rewind it.
        private void Refresh()
        {
            var wanted = Wanted();
            if (_source.clip == wanted) return;

            _source.Stop();
            _source.clip = wanted;
            _source.volume = volume * (wanted == lobbyTrack ? 0.5f : wanted == spectatorTrack ? 0.6f : 1f);
            if (wanted != null) _source.Play();
        }
    }
}
