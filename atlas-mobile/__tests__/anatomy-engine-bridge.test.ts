import {
  ANATOMY_ENGINE_COMMANDS,
  ANATOMY_ENGINE_SCHEMA_VERSION,
  ANATOMY_ENGINE_TOPIC,
  loadExerciseBiomechanics,
  setHighlightMuscles,
  setJointAngleOverlay,
  setLayerVisibility,
} from '../src/native/anatomyEngineBridge';
import { sendMessageToUnity } from '../src/native/unityBridge';

jest.mock('../src/native/unityBridge', () => ({
  sendMessageToUnity: jest.fn(async () => undefined),
}));

const mockSendMessageToUnity = sendMessageToUnity as jest.MockedFunction<typeof sendMessageToUnity>;

const BIOMECHANICS_PAYLOAD = {
  exerciseId: 'exercise-1',
  exerciseSlug: 'back-squat',
  exerciseName: 'Back Squat',
  animationAssetKey: 'biomechanics/back-squat/clip_v1.fbx',
  animationAssetUri: 's3://atlas-assets/biomechanics/back-squat/clip_v1.fbx',
  rigVersion: 'atlas-humanoid-v1',
  muscleHighlights: [
    { muscleGroup: 'quads', activationLevel: 1, role: 'primary', colorHex: '#FF6B35' },
  ],
  jointAngles: [{ joint: 'knee', minDegrees: 70, maxDegrees: 175, targetDegrees: 95, unit: 'deg' }],
};

describe('anatomyEngineBridge', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('sends load_exercise_biomechanics command', async () => {
    await loadExerciseBiomechanics(BIOMECHANICS_PAYLOAD, 'request-load');

    expect(mockSendMessageToUnity).toHaveBeenCalledTimes(1);
    expect(mockSendMessageToUnity.mock.calls[0][0]).toBe(ANATOMY_ENGINE_TOPIC);

    const command = JSON.parse(mockSendMessageToUnity.mock.calls[0][1]) as Record<string, unknown>;
    expect(command).toMatchObject({
      schemaVersion: ANATOMY_ENGINE_SCHEMA_VERSION,
      requestId: 'request-load',
      command: ANATOMY_ENGINE_COMMANDS.loadExerciseBiomechanics,
      loadExerciseBiomechanics: {
        biomechanics: BIOMECHANICS_PAYLOAD,
      },
    });
  });

  it('sends set_highlight_muscles command', async () => {
    await setHighlightMuscles(
      {
        muscleGroups: ['quads', 'glutes'],
      },
      'request-highlights',
    );

    const command = JSON.parse(mockSendMessageToUnity.mock.calls[0][1]) as Record<string, unknown>;
    expect(command).toMatchObject({
      schemaVersion: ANATOMY_ENGINE_SCHEMA_VERSION,
      requestId: 'request-highlights',
      command: ANATOMY_ENGINE_COMMANDS.setHighlightMuscles,
      setHighlightMuscles: {
        muscleGroups: ['quads', 'glutes'],
      },
    });
  });

  it('sends set_layer_visibility command', async () => {
    await setLayerVisibility(
      {
        showSkeleton: false,
        showMuscles: true,
      },
      'request-layers',
    );

    const command = JSON.parse(mockSendMessageToUnity.mock.calls[0][1]) as Record<string, unknown>;
    expect(command).toMatchObject({
      schemaVersion: ANATOMY_ENGINE_SCHEMA_VERSION,
      requestId: 'request-layers',
      command: ANATOMY_ENGINE_COMMANDS.setLayerVisibility,
      setLayerVisibility: {
        showSkeleton: false,
        showMuscles: true,
      },
    });
  });

  it('sends set_joint_angle_overlay command', async () => {
    await setJointAngleOverlay(
      {
        enabled: true,
        jointAngles: [{ joint: 'hip', minDegrees: 60, maxDegrees: 140, targetDegrees: 90, unit: 'deg' }],
      },
      'request-overlay',
    );

    const command = JSON.parse(mockSendMessageToUnity.mock.calls[0][1]) as Record<string, unknown>;
    expect(command).toMatchObject({
      schemaVersion: ANATOMY_ENGINE_SCHEMA_VERSION,
      requestId: 'request-overlay',
      command: ANATOMY_ENGINE_COMMANDS.setJointAngleOverlay,
      setJointAngleOverlay: {
        enabled: true,
        jointAngles: [{ joint: 'hip', minDegrees: 60, maxDegrees: 140, targetDegrees: 90, unit: 'deg' }],
      },
    });
  });
});
