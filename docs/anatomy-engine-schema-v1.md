# Anatomy Engine Schema v1

Atlas uses a versioned command envelope for RN -> native -> Unity messaging.

## Envelope

- Unity topic: `anatomy.engine.v1`
- `schemaVersion`: `anatomy-engine.v1`

```json
{
  "schemaVersion": "anatomy-engine.v1",
  "requestId": "anatomy-m7l9v2-xk3n1d",
  "command": "load_exercise_biomechanics",
  "loadExerciseBiomechanics": {
    "biomechanics": {
      "exerciseId": "exercise-1",
      "exerciseSlug": "back-squat",
      "exerciseName": "Back Squat",
      "animationAssetKey": "biomechanics/back-squat/clip_v1.fbx",
      "animationAssetUri": "s3://atlas-assets/biomechanics/back-squat/clip_v1.fbx",
      "rigVersion": "atlas-humanoid-v1",
      "muscleHighlights": [],
      "jointAngles": []
    }
  }
}
```

## Commands

1. `load_exercise_biomechanics`
- Payload key: `loadExerciseBiomechanics`
- Value: `{ biomechanics: ExerciseBiomechanicsMetadata }`

2. `set_highlight_muscles`
- Payload key: `setHighlightMuscles`
- Value: `{ muscleGroups?: string[], highlights?: MuscleHighlightMetadata[] }`

3. `set_layer_visibility`
- Payload key: `setLayerVisibility`
- Value: `{ showSkeleton: boolean, showMuscles: boolean }`

4. `set_joint_angle_overlay`
- Payload key: `setJointAngleOverlay`
- Value: `{ enabled: boolean, jointAngles?: JointAngleMetadata[] }`

## Unity -> RN command result callback

Unity emits `unity.anatomy.command` for each command:

```json
{
  "schemaVersion": "anatomy-engine.v1",
  "requestId": "anatomy-m7l9v2-xk3n1d",
  "command": "set_layer_visibility",
  "status": "applied",
  "reason": ""
}
```

## Lifecycle/state callbacks

Native emits:
- `unity.lifecycle` for open/close counts and mode
- `unity.state` for high-level state transitions

`unity.state` payload:

```json
{
  "state": "loading | loaded | failed | closed",
  "mode": "unity | fallback | native | background",
  "reason": "",
  "openCount": 2,
  "closeCount": 1
}
```
