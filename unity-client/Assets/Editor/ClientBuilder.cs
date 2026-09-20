using System.Linq;
using UnityEditor;
using UnityEditor.Build.Reporting;
using UnityEngine;

namespace Game.EditorTools
{
    /// <summary>Player builds for the batch-mode <c>make client*</c> targets and the Game &gt; Build menu.</summary>
    public static class ClientBuilder
    {
        private const string Root = "Builds";

        [MenuItem("Game/Build/Linux")]
        public static void BuildLinux() => Exit(Build(BuildTarget.StandaloneLinux64, "Linux/MMOClient.x86_64"));

        [MenuItem("Game/Build/macOS")]
        public static void BuildMac() => Exit(Build(BuildTarget.StandaloneOSX, "macOS/MMOClient.app"));

        [MenuItem("Game/Build/All")]
        public static void BuildAll()
        {
            // Build both even if the first fails, so one run reports every broken target.
            var linux = Build(BuildTarget.StandaloneLinux64, "Linux/MMOClient.x86_64");
            var mac = Build(BuildTarget.StandaloneOSX, "macOS/MMOClient.app");
            Exit(linux && mac);
        }

        private static bool Build(BuildTarget target, string path)
        {
            var options = new BuildPlayerOptions
            {
                scenes = EditorBuildSettings.scenes.Where(s => s.enabled).Select(s => s.path).ToArray(),
                locationPathName = $"{Root}/{path}",
                target = target,
                targetGroup = BuildTargetGroup.Standalone,
            };

            var summary = BuildPipeline.BuildPlayer(options).summary;
            Debug.Log($"[ClientBuilder] {target}: {summary.result}, {summary.totalErrors} errors, {summary.outputPath}");
            return summary.result == BuildResult.Succeeded;
        }

        // Batch mode needs an explicit non-zero exit so make stops on failure; in the Editor just leave it open.
        private static void Exit(bool ok)
        {
            if (Application.isBatchMode) EditorApplication.Exit(ok ? 0 : 1);
        }
    }
}
