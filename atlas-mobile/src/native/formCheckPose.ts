import { DeviceEventEmitter, NativeModules } from 'react-native';
import {
  summarizePoseFrames,
  type FormCheckMovementType,
  type FormCheckPoseSummary,
  type PoseAngleFrame,
} from '../features/formCheck/scoring';

const FORM_CHECK_POSE_EVENT = 'FormCheckPoseFrame';

type NativeFormCheckPoseModule = {
  startDetection: (movementType: string) => Promise<void>;
  stopDetection: () => Promise<Partial<FormCheckPoseSummary>>;
  addListener?: (eventName: string) => void;
  removeListeners?: (count: number) => void;
};

type FallbackRuntimeState = {
  isRunning: boolean;
  phase: number;
  timerId: ReturnType<typeof setInterval> | null;
  frames: PoseAngleFrame[];
  startedAtMs: number;
};

const fallbackRuntime: FallbackRuntimeState = {
  isRunning: false,
  phase: 0,
  timerId: null,
  frames: [],
  startedAtMs: 0,
};

function getNativeModule(): NativeFormCheckPoseModule | null {
  const nativeModule = NativeModules.FormCheckPoseModule as
    | NativeFormCheckPoseModule
    | undefined;

  return nativeModule ?? null;
}

function toFiniteNumber(value: unknown, fallback = 0): number {
  if (typeof value !== 'number' || Number.isNaN(value) || !Number.isFinite(value)) {
    return fallback;
  }
  return value;
}

function normalizeSummary(
  summary: Partial<FormCheckPoseSummary> | null | undefined,
  movementType: FormCheckMovementType,
): FormCheckPoseSummary {
  if (!summary) {
    return summarizePoseFrames({ movementType, frames: fallbackRuntime.frames });
  }

  return {
    movementType,
    sampleCount: toFiniteNumber(summary.sampleCount, fallbackRuntime.frames.length),
    repetitionCount: toFiniteNumber(summary.repetitionCount, 0),
    rangeOfMotionDegrees: toFiniteNumber(summary.rangeOfMotionDegrees, 0),
    rangeOfMotionScore: toFiniteNumber(summary.rangeOfMotionScore, 0),
    kneeTrackingScore: toFiniteNumber(summary.kneeTrackingScore, 0),
    symmetryScore: toFiniteNumber(summary.symmetryScore, 0),
    overallScore: toFiniteNumber(summary.overallScore, 0),
    feedback:
      Array.isArray(summary.feedback) && summary.feedback.every(item => typeof item === 'string')
        ? summary.feedback
        : ['Review your next set for more complete feedback.'],
    minLeftKneeDeg: toFiniteNumber(summary.minLeftKneeDeg, 0),
    minRightKneeDeg: toFiniteNumber(summary.minRightKneeDeg, 0),
    maxLeftKneeDeg: toFiniteNumber(summary.maxLeftKneeDeg, 0),
    maxRightKneeDeg: toFiniteNumber(summary.maxRightKneeDeg, 0),
  };
}

function pushFallbackFrame(frame: PoseAngleFrame): void {
  fallbackRuntime.frames.push(frame);
  if (fallbackRuntime.frames.length > 900) {
    fallbackRuntime.frames.splice(0, fallbackRuntime.frames.length - 900);
  }
  DeviceEventEmitter.emit(FORM_CHECK_POSE_EVENT, frame);
}

function buildFallbackPoseFrame(): PoseAngleFrame {
  const timestampMs = Date.now();
  const depth = (Math.sin(fallbackRuntime.phase) + 1) / 2;
  const sway = Math.sin(fallbackRuntime.phase * 0.5);

  const leftKneeDeg = 173 - depth * 92 + sway * 3;
  const rightKneeDeg = 171 - depth * 90 - sway * 3;
  const leftHipDeg = 169 - depth * 78 + sway * 2;
  const rightHipDeg = 167 - depth * 80 - sway * 2;

  fallbackRuntime.phase += 0.22;

  return {
    timestampMs,
    leftKneeDeg,
    rightKneeDeg,
    leftHipDeg,
    rightHipDeg,
  };
}

function resetFallbackRuntime(): void {
  if (fallbackRuntime.timerId) {
    clearInterval(fallbackRuntime.timerId);
  }

  fallbackRuntime.isRunning = false;
  fallbackRuntime.timerId = null;
  fallbackRuntime.phase = 0;
  fallbackRuntime.frames = [];
  fallbackRuntime.startedAtMs = 0;
}

export function subscribeToPoseFrames(callback: (frame: PoseAngleFrame) => void): () => void {
  const subscription = DeviceEventEmitter.addListener(FORM_CHECK_POSE_EVENT, event => {
    callback({
      timestampMs: toFiniteNumber(event?.timestampMs, Date.now()),
      leftKneeDeg: toFiniteNumber(event?.leftKneeDeg),
      rightKneeDeg: toFiniteNumber(event?.rightKneeDeg),
      leftHipDeg: toFiniteNumber(event?.leftHipDeg),
      rightHipDeg: toFiniteNumber(event?.rightHipDeg),
    });
  });

  return () => {
    subscription.remove();
  };
}

export async function startFormCheckDetection(movementType: FormCheckMovementType): Promise<void> {
  const nativeModule = getNativeModule();

  fallbackRuntime.frames = [];

  if (nativeModule) {
    await nativeModule.startDetection(movementType);
    return;
  }

  resetFallbackRuntime();
  fallbackRuntime.isRunning = true;
  fallbackRuntime.startedAtMs = Date.now();

  fallbackRuntime.timerId = setInterval(() => {
    if (!fallbackRuntime.isRunning) {
      return;
    }
    pushFallbackFrame(buildFallbackPoseFrame());
  }, 120);
}

export async function stopFormCheckDetection(
  movementType: FormCheckMovementType,
): Promise<FormCheckPoseSummary> {
  const nativeModule = getNativeModule();

  if (nativeModule) {
    const result = await nativeModule.stopDetection();
    return normalizeSummary(result, movementType);
  }

  fallbackRuntime.isRunning = false;
  if (fallbackRuntime.timerId) {
    clearInterval(fallbackRuntime.timerId);
    fallbackRuntime.timerId = null;
  }

  const summary = summarizePoseFrames({ movementType, frames: fallbackRuntime.frames });
  fallbackRuntime.frames = [];

  return summary;
}

export function resetFormCheckPoseRuntime(): void {
  resetFallbackRuntime();
}
