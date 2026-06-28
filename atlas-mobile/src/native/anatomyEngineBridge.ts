import { sendMessageToUnity } from './unityBridge';

export const ANATOMY_ENGINE_SCHEMA_VERSION = 'anatomy-engine.v1';
export const ANATOMY_ENGINE_TOPIC = 'anatomy.engine.v1';

export const ANATOMY_ENGINE_COMMANDS = {
  loadExerciseBiomechanics: 'load_exercise_biomechanics',
  setHighlightMuscles: 'set_highlight_muscles',
  setLayerVisibility: 'set_layer_visibility',
  setJointAngleOverlay: 'set_joint_angle_overlay',
} as const;

export type AnatomyEngineCommandName =
  typeof ANATOMY_ENGINE_COMMANDS[keyof typeof ANATOMY_ENGINE_COMMANDS];

export type MuscleHighlightPayload = {
  muscleGroup: string;
  activationLevel: number;
  role: string;
  colorHex?: string;
};

export type JointAnglePayload = {
  joint: string;
  minDegrees: number;
  maxDegrees: number;
  targetDegrees: number;
  unit: string;
  proximalBone?: string;
  jointBone?: string;
  distalBone?: string;
};

export type ExerciseBiomechanicsPayload = {
  exerciseId: string;
  exerciseSlug: string;
  exerciseName: string;
  animationAssetKey: string;
  animationAssetUri: string;
  rigVersion: string;
  muscleHighlights: MuscleHighlightPayload[];
  jointAngles: JointAnglePayload[];
  [key: string]: unknown;
};

export type AnatomyEngineCommandMessage = {
  schemaVersion: typeof ANATOMY_ENGINE_SCHEMA_VERSION;
  requestId: string;
  command: AnatomyEngineCommandName;
  loadExerciseBiomechanics?: {
    biomechanics: ExerciseBiomechanicsPayload;
  };
  setHighlightMuscles?: {
    muscleGroups?: string[];
    highlights?: MuscleHighlightPayload[];
  };
  setLayerVisibility?: {
    showSkeleton: boolean;
    showMuscles: boolean;
  };
  setJointAngleOverlay?: {
    enabled: boolean;
    jointAngles?: JointAnglePayload[];
  };
};

export async function loadExerciseBiomechanics(
  biomechanics: ExerciseBiomechanicsPayload,
  requestId: string = createAnatomyEngineRequestId(),
): Promise<void> {
  await sendAnatomyEngineCommand({
    schemaVersion: ANATOMY_ENGINE_SCHEMA_VERSION,
    requestId,
    command: ANATOMY_ENGINE_COMMANDS.loadExerciseBiomechanics,
    loadExerciseBiomechanics: {
      biomechanics,
    },
  });
}

export async function setHighlightMuscles(
  input: {
    muscleGroups?: string[];
    highlights?: MuscleHighlightPayload[];
  },
  requestId: string = createAnatomyEngineRequestId(),
): Promise<void> {
  await sendAnatomyEngineCommand({
    schemaVersion: ANATOMY_ENGINE_SCHEMA_VERSION,
    requestId,
    command: ANATOMY_ENGINE_COMMANDS.setHighlightMuscles,
    setHighlightMuscles: {
      muscleGroups: input.muscleGroups,
      highlights: input.highlights,
    },
  });
}

export async function setLayerVisibility(
  input: {
    showSkeleton: boolean;
    showMuscles: boolean;
  },
  requestId: string = createAnatomyEngineRequestId(),
): Promise<void> {
  await sendAnatomyEngineCommand({
    schemaVersion: ANATOMY_ENGINE_SCHEMA_VERSION,
    requestId,
    command: ANATOMY_ENGINE_COMMANDS.setLayerVisibility,
    setLayerVisibility: {
      showSkeleton: input.showSkeleton,
      showMuscles: input.showMuscles,
    },
  });
}

export async function setJointAngleOverlay(
  input: {
    enabled: boolean;
    jointAngles?: JointAnglePayload[];
  },
  requestId: string = createAnatomyEngineRequestId(),
): Promise<void> {
  await sendAnatomyEngineCommand({
    schemaVersion: ANATOMY_ENGINE_SCHEMA_VERSION,
    requestId,
    command: ANATOMY_ENGINE_COMMANDS.setJointAngleOverlay,
    setJointAngleOverlay: {
      enabled: input.enabled,
      jointAngles: input.jointAngles,
    },
  });
}

export async function sendAnatomyEngineCommand(message: AnatomyEngineCommandMessage): Promise<void> {
  await sendMessageToUnity(ANATOMY_ENGINE_TOPIC, JSON.stringify(message));
}

export function createAnatomyEngineRequestId(prefix: string = 'anatomy'): string {
  const entropy = Math.random().toString(36).slice(2, 8);
  return `${prefix}-${Date.now().toString(36)}-${entropy}`;
}
