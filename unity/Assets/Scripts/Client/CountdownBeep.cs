using Game.V1;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// Beeps once per second of the pre-round countdown, in step with the "STARTING IN" banner.
    /// The beep is synthesized, so it needs no audio asset.
    /// </summary>
    [RequireComponent(typeof(GameState))]
    public class CountdownBeep : MonoBehaviour
    {
        private const int SampleRate = 44100;

        [Tooltip("Pitch of the beep in Hz.")]
        [SerializeField] private float frequency = 880f;
        [Range(0f, 1f)]
        [SerializeField] private float volume = 0.5f;

        private GameState _state;
        private AudioSource _source;
        private AudioClip _beep;
        private int _lastSecond; // the whole second last beeped, 0 outside a countdown

        private void Awake()
        {
            _state = GetComponent<GameState>();
            _source = gameObject.AddComponent<AudioSource>();
            _source.playOnAwake = false;
            _beep = MakeBeep(frequency);
        }

        // Beeps whenever the rounded-up seconds left change, the same rounding the banner shows,
        // so "3", "2" and "1" each get one beep as they appear.
        private void Update()
        {
            if (_state.Phase != GamePhase.Countdown || _state.SecondsLeft is not { } left)
            {
                _lastSecond = 0;
                return;
            }

            int second = Mathf.CeilToInt(left);
            if (second <= 0 || second == _lastSecond) return;

            _lastSecond = second;
            _source.PlayOneShot(_beep, volume);
        }

        // A soft chime: a sine with a faint octave above it, a few milliseconds of fade-in so it
        // doesn't click, and an exponential fade-out instead of a hard stop.
        private static AudioClip MakeBeep(float frequency)
        {
            const float length = 0.25f;
            const float attack = 0.008f;
            const float decay = 18f; // per second; the tail is inaudible well before the clip ends

            var samples = new float[(int)(SampleRate * length)];
            for (int i = 0; i < samples.Length; i++)
            {
                float t = (float)i / SampleRate;
                float envelope = Mathf.Min(1f, t / attack) * Mathf.Exp(-decay * t);
                float tone = Mathf.Sin(2f * Mathf.PI * frequency * t)
                             + 0.2f * Mathf.Sin(4f * Mathf.PI * frequency * t);
                samples[i] = 0.6f * envelope * tone;
            }

            var clip = AudioClip.Create("Countdown beep", samples.Length, 1, SampleRate, false);
            clip.SetData(samples, 0);
            return clip;
        }
    }
}
