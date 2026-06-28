using System;

public static class AnatomyEngineBridgeSchema
{
    public const string Version = "anatomy-engine.v1";
    public const string Topic = "anatomy.engine.v1";
    public const string LegacyTopic = "anatomy.engine";

    public const string CommandLoadExerciseBiomechanics = "load_exercise_biomechanics";
    public const string CommandSetHighlightMuscles = "set_highlight_muscles";
    public const string CommandSetLayerVisibility = "set_layer_visibility";
    public const string CommandSetJointAngleOverlay = "set_joint_angle_overlay";
}

[Serializable]
public sealed class AnatomyEngineCommandMessage
{
    public string schemaVersion;
    public string requestId;
    public string command;
    public LoadExerciseBiomechanicsCommand loadExerciseBiomechanics;
    public SetHighlightMusclesCommand setHighlightMuscles;
    public SetLayerVisibilityCommand setLayerVisibility;
    public SetJointAngleOverlayCommand setJointAngleOverlay;
}

[Serializable]
public sealed class LoadExerciseBiomechanicsCommand
{
    public ExerciseBiomechanicsMetadata biomechanics;
}

[Serializable]
public sealed class SetHighlightMusclesCommand
{
    public string[] muscleGroups;
    public MuscleHighlightMetadata[] highlights;
}

[Serializable]
public sealed class SetLayerVisibilityCommand
{
    public bool showSkeleton;
    public bool showMuscles;
}

[Serializable]
public sealed class SetJointAngleOverlayCommand
{
    public bool enabled;
    public JointAngleMetadata[] jointAngles;
}
