using System;

[Serializable]
public sealed class ExerciseBiomechanicsMetadata
{
    public string exerciseId;
    public string exerciseSlug;
    public string exerciseName;
    public string animationAssetKey;
    public string animationAssetUri;
    public string rigVersion;
    public MuscleHighlightMetadata[] muscleHighlights;
    public JointAngleMetadata[] jointAngles;
}

[Serializable]
public sealed class MuscleHighlightMetadata
{
    public string muscleGroup;
    public float activationLevel;
    public string role;
    public string colorHex;
}

[Serializable]
public sealed class JointAngleMetadata
{
    public string joint;
    public float minDegrees;
    public float maxDegrees;
    public float targetDegrees;
    public string unit;
    public string proximalBone;
    public string jointBone;
    public string distalBone;
}
