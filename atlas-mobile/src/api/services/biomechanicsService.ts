import { API_BASE_URL, getApiErrorMessage } from '../client';

export type MuscleHighlight = {
  muscleGroup: string;
  activationLevel: number;
  role: string;
  colorHex?: string;
};

export type JointAngle = {
  joint: string;
  minDegrees: number;
  maxDegrees: number;
  targetDegrees: number;
  unit: string;
};

export type ExerciseBiomechanics = {
  exerciseId: string;
  exerciseSlug: string;
  exerciseName: string;
  animationAssetKey: string;
  animationAssetUri: string;
  rigVersion: string;
  muscleHighlights: MuscleHighlight[];
  jointAngles: JointAngle[];
  metadata: Record<string, unknown>;
};

type ExerciseBiomechanicsInput = {
  accessToken: string;
  exerciseId: string;
};

type ExerciseBiomechanicsResponse = {
  biomechanics: ExerciseBiomechanics;
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

  const response = await fetch(`${API_BASE_URL}/api/v1/exercises/${input.exerciseId}/biomechanics`, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
      Accept: 'application/json',
    },
  });

  if (!response.ok) {
    const message = getApiErrorMessage(await response.json().catch(() => null), 'Unable to load biomechanics preview.');
    throw new Error(message);
  }

  const payload = (await response.json()) as ExerciseBiomechanicsResponse;
  if (!payload.biomechanics) {
    throw new Error('Biomechanics payload was empty.');
  }

  return payload.biomechanics;
}
