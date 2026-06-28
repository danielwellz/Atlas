using UnityEngine;

public static class AtlasBridgeBootstrap
{
    [RuntimeInitializeOnLoadMethod(RuntimeInitializeLoadType.AfterSceneLoad)]
    private static void EnsureBridgeInstance()
    {
        AtlasBridgeBehaviour existingBridge = Object.FindFirstObjectByType<AtlasBridgeBehaviour>();
        if (existingBridge != null)
        {
            return;
        }

        GameObject bridgeObject = new GameObject("AtlasBridge");
        Object.DontDestroyOnLoad(bridgeObject);
        bridgeObject.AddComponent<AtlasBridgeBehaviour>();
    }
}
