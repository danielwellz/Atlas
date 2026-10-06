import type { components } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';

export type MuscleHighlight = components['schemas']['MuscleHighlight'];
export type JointAngle = components['schemas']['JointAngle'];
export type ExerciseBiomechanics = components['schemas']['ExerciseBiomechanics'];

type ExerciseBiomechanicsInput = {
  accessToken: string;
  exerciseId: string;
};

const MOCK_EXERCISE_BIOMECH_BY_ID: Record<string, ExerciseBiomechanics> = {
  'exercise-1': {
    exerciseId: 'exercise-1',
    exerciseSlug: 'back-squat',
    exerciseName: 'Back Squat',
    animationAssetKey: 'biomechanics/back-squat/clip_v1.fbx',
    animationAssetUri: 's3://atlas-assets/biomechanics/back-squat/clip_v1.fbx',
    rigVersion: 'atlas-humanoid-v1',
    muscleHighlights: [
      { muscleGroup: 'quads', activationLevel: 1, role: 'primary', colorHex: '#FF6B35' },
      { muscleGroup: 'glutes', activationLevel: 0.82, role: 'secondary', colorHex: '#F97316' },
      { muscleGroup: 'core', activationLevel: 0.55, role: 'stabilizer', colorHex: '#FB923C' },
    ],
    jointAngles: [
      { joint: 'knee', minDegrees: 70, maxDegrees: 175, targetDegrees: 95, unit: 'deg' },
      { joint: 'hip', minDegrees: 55, maxDegrees: 170, targetDegrees: 100, unit: 'deg' },
    ],
    metadata: {
      source: 'mock',
    },
  },
};

function createFallbackMockBiomechanics(exerciseId: string): ExerciseBiomechanics {
  return {
    exerciseId,
    exerciseSlug: 'exercise-preview',
    exerciseName: 'Exercise Preview',
    animationAssetKey: 'biomechanics/exercise-preview/clip_v1.fbx',
    animationAssetUri: 's3://atlas-assets/biomechanics/exercise-preview/clip_v1.fbx',
    rigVersion: 'atlas-humanoid-v1',
    muscleHighlights: [
      { muscleGroup: 'core', activationLevel: 0.5, role: 'stabilizer', colorHex: '#38BDF8' },
    ],
    jointAngles: [
      { joint: 'hip', minDegrees: 60, maxDegrees: 140, targetDegrees: 90, unit: 'deg' },
    ],
    metadata: {
      source: 'mock-fallback',
    },
  };
}

export async function getExerciseBiomechanics(
  input: ExerciseBiomechanicsInput,
  useMock: boolean,
): Promise<ExerciseBiomechanics> {
  if (useMock) {
    return MOCK_EXERCISE_BIOMECH_BY_ID[input.exerciseId] ??
      createFallbackMockBiomechanics(input.exerciseId);
  }

  const response = await atlasApiClient.GET('/api/v1/exercises/{id}/biomechanics', {
    params: {
      path: {
        id: input.exerciseId,
      },
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load biomechanics preview.'));
  }

  const payload = response.data;
  if (!payload.biomechanics) {
    throw new Error('Biomechanics payload was empty.');
  }

  return payload.biomechanics;
}
