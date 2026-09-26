using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// Runs a fullscreen player at the monitor's own resolution. Unity otherwise restores the last
    /// saved size, often not the native one, and renders scaled up to fill the screen; under
    /// Hyprland that scaling also maps the mouse wrongly, so the lobby buttons can't be clicked.
    /// </summary>
    public static class NativeResolution
    {
        [RuntimeInitializeOnLoadMethod(RuntimeInitializeLoadType.AfterSceneLoad)]
        private static void Apply()
        {
            if (Application.isEditor || !Screen.fullScreen) return;

            var display = Display.main;
            if (Screen.width == display.systemWidth && Screen.height == display.systemHeight) return;

            Screen.SetResolution(display.systemWidth, display.systemHeight, FullScreenMode.FullScreenWindow);
        }
    }
}
