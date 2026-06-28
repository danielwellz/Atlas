using System;
using System.Runtime.InteropServices;
using UnityEngine;

public sealed class AtlasBridgeBehaviour : MonoBehaviour
{
    [Serializable]
    private struct BridgeMessage
    {
        public string topic;
        public string payload;
    }

    private const string AndroidBridgeClass = "com.atlasmobile.unity.AtlasUnityBridge";
    private const string LegacyAnatomyPreviewTopic = "anatomy.preview";
    private const string UnityCommandResultTopic = "unity.anatomy.command";

    [Serializable]
    private struct UnityReadyPayload
    {
        public string source;
        public string schemaVersion;
    }

    [Serializable]
    private struct CommandResultPayload
    {
        public string schemaVersion;
        public string requestId;
        public string command;
        public string status;
        public string reason;
    }

#if UNITY_IOS && !UNITY_EDITOR
    [DllImport("__Internal")]
    private static extern void AtlasUnitySendMessageToReact(string topic, string payload);
#endif

    private void Start()
    {
        UnityReadyPayload readyPayload = new UnityReadyPayload
        {
            source = "unity",
            schemaVersion = AnatomyEngineBridgeSchema.Version,
        };
        SendToReact("unity.ready", JsonUtility.ToJson(readyPayload));
    }

    // Invoked by React Native native code through UnitySendMessage.
    public void OnReactNativeMessage(string encodedMessage)
    {
        if (string.IsNullOrEmpty(encodedMessage))
        {
            return;
        }

        BridgeMessage message;
        try
        {
            message = JsonUtility.FromJson<BridgeMessage>(encodedMessage);
        }
        catch (Exception)
        {
            return;
        }

        if (string.IsNullOrWhiteSpace(message.topic))
        {
            return;
        }

        Debug.Log($"[AtlasBridge] RN -> Unity topic={message.topic}");

        if (string.Equals(message.topic, LegacyAnatomyPreviewTopic, StringComparison.OrdinalIgnoreCase))
        {
            if (BiomechanicsPlaybackController.Instance != null)
            {
                BiomechanicsPlaybackController.Instance.LoadExerciseBiomechanicsFromJSON(message.payload);
            }
            else
            {
                Debug.LogWarning("[AtlasBridge] anatomy.preview received but no BiomechanicsPlaybackController is active.");
            }
        }
        else if (IsAnatomyEngineTopic(message.topic))
        {
            HandleAnatomyEngineCommand(message.payload);
        }

        SendToReact("unity.ack", encodedMessage);
    }

    private static bool IsAnatomyEngineTopic(string topic)
    {
        return string.Equals(topic, AnatomyEngineBridgeSchema.Topic, StringComparison.OrdinalIgnoreCase) ||
            string.Equals(topic, AnatomyEngineBridgeSchema.LegacyTopic, StringComparison.OrdinalIgnoreCase);
    }

    private static void HandleAnatomyEngineCommand(string commandPayload)
    {
        if (string.IsNullOrWhiteSpace(commandPayload))
        {
            EmitCommandResult(command: string.Empty, requestId: string.Empty, status: "failed", reason: "missing_payload");
            return;
        }

        AnatomyEngineCommandMessage commandMessage;
        try
        {
            commandMessage = JsonUtility.FromJson<AnatomyEngineCommandMessage>(commandPayload);
        }
        catch (Exception)
        {
            EmitCommandResult(command: string.Empty, requestId: string.Empty, status: "failed", reason: "invalid_json");
            return;
        }

        if (commandMessage == null || string.IsNullOrWhiteSpace(commandMessage.command))
        {
            EmitCommandResult(command: string.Empty, requestId: string.Empty, status: "failed", reason: "missing_command");
            return;
        }

        if (!string.Equals(commandMessage.schemaVersion, AnatomyEngineBridgeSchema.Version, StringComparison.OrdinalIgnoreCase))
        {
            EmitCommandResult(
                command: commandMessage.command,
                requestId: commandMessage.requestId,
                status: "failed",
                reason: "schema_version_mismatch");
            return;
        }

        BiomechanicsPlaybackController playbackController = BiomechanicsPlaybackController.Instance;
        if (playbackController == null)
        {
            EmitCommandResult(
                command: commandMessage.command,
                requestId: commandMessage.requestId,
                status: "failed",
                reason: "playback_controller_missing");
            return;
        }

        bool applied = false;
        switch (commandMessage.command)
        {
            case AnatomyEngineBridgeSchema.CommandLoadExerciseBiomechanics:
                if (commandMessage.loadExerciseBiomechanics != null &&
                    commandMessage.loadExerciseBiomechanics.biomechanics != null)
                {
                    playbackController.LoadExerciseBiomechanics(commandMessage.loadExerciseBiomechanics.biomechanics);
                    applied = true;
                }
                break;
            case AnatomyEngineBridgeSchema.CommandSetHighlightMuscles:
                if (commandMessage.setHighlightMuscles != null)
                {
                    playbackController.SetHighlightMuscles(
                        commandMessage.setHighlightMuscles.highlights,
                        commandMessage.setHighlightMuscles.muscleGroups);
                    applied = true;
                }
                break;
            case AnatomyEngineBridgeSchema.CommandSetLayerVisibility:
                if (commandMessage.setLayerVisibility != null)
                {
                    playbackController.SetLayerVisibility(
                        commandMessage.setLayerVisibility.showSkeleton,
                        commandMessage.setLayerVisibility.showMuscles);
                    applied = true;
                }
                break;
            case AnatomyEngineBridgeSchema.CommandSetJointAngleOverlay:
                if (commandMessage.setJointAngleOverlay != null)
                {
                    playbackController.SetJointAngleOverlay(
                        commandMessage.setJointAngleOverlay.enabled,
                        commandMessage.setJointAngleOverlay.jointAngles);
                    applied = true;
                }
                break;
        }

        EmitCommandResult(
            command: commandMessage.command,
            requestId: commandMessage.requestId,
            status: applied ? "applied" : "failed",
            reason: applied ? string.Empty : "invalid_command_payload");
    }

    private static void EmitCommandResult(string command, string requestId, string status, string reason)
    {
        CommandResultPayload payload = new CommandResultPayload
        {
            schemaVersion = AnatomyEngineBridgeSchema.Version,
            requestId = requestId ?? string.Empty,
            command = command ?? string.Empty,
            status = status ?? string.Empty,
            reason = reason ?? string.Empty,
        };

        SendToReact(UnityCommandResultTopic, JsonUtility.ToJson(payload));
    }

    public static void SendToReact(string topic, string payload)
    {
#if UNITY_ANDROID && !UNITY_EDITOR
        using (AndroidJavaClass bridgeClass = new AndroidJavaClass(AndroidBridgeClass))
        {
            bridgeClass.CallStatic("sendMessageToReact", topic, payload);
        }
#elif UNITY_IOS && !UNITY_EDITOR
        AtlasUnitySendMessageToReact(topic, payload);
#else
        Debug.Log($"[AtlasBridge] Editor message topic={topic} payload={payload}");
#endif
    }
}
