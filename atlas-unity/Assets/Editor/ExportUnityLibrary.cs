#if UNITY_EDITOR
using System;
using System.IO;
using System.Linq;
using UnityEditor;
using UnityEditor.Build.Reporting;
using UnityEngine;

public static class ExportUnityLibrary
{
    private static string[] EnabledScenes =>
        EditorBuildSettings.scenes.Where(scene => scene.enabled).Select(scene => scene.path).ToArray();

    [MenuItem("Atlas/Export/Android unityLibrary")]
    public static void ExportAndroidUnityLibrary()
    {
        Export(BuildTarget.Android, "Builds/android");
    }

    [MenuItem("Atlas/Export/iOS Unity Export")]
    public static void ExportIosUnityLibrary()
    {
        Export(BuildTarget.iOS, "Builds/ios");
    }

    private static void Export(BuildTarget target, string relativeOutputDirectory)
    {
        if (EnabledScenes.Length == 0)
        {
            throw new InvalidOperationException("At least one enabled scene is required for Unity export.");
        }

        string projectRoot = Path.GetFullPath(Path.Combine(Application.dataPath, ".."));
        string outputDirectory = Path.Combine(projectRoot, relativeOutputDirectory);
        Directory.CreateDirectory(outputDirectory);

        BuildPlayerOptions options = new BuildPlayerOptions
        {
            scenes = EnabledScenes,
            target = target,
            locationPathName = outputDirectory,
            options = BuildOptions.AcceptExternalModificationsToPlayer,
        };

        BuildReport report = BuildPipeline.BuildPlayer(options);
        if (report.summary.result != BuildResult.Succeeded)
        {
            throw new InvalidOperationException($"Unity export failed for {target}. Check Editor.log for details.");
        }

        Debug.Log($"Unity export succeeded for {target} at {outputDirectory}");
    }
}
#endif
