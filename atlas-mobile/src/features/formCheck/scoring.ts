export type FormCheckMovementType = 'squat' | 'hinge' | 'lunge' | 'push' | 'pull';

export type PoseAngleFrame = {
  timestampMs: number;
  leftKneeDeg: number;
  rightKneeDeg: number;
  leftHipDeg: number;
  rightHipDeg: number;
};

export type FormCheckPoseSummary = {
  movementType: FormCheckMovementType;
  sampleCount: number;
  repetitionCount: number;
  rangeOfMotionDegrees: number;
  rangeOfMotionScore: number;
  kneeTrackingScore: number;
  symmetryScore: number;
  overallScore: number;
  feedback: string[];
  minLeftKneeDeg: number;
  minRightKneeDeg: number;
  maxLeftKneeDeg: number;
  maxRightKneeDeg: number;
};

function clamp(value: number, min: number, max: number): number {
  if (Number.isNaN(value)) {
    return min;
  }
  return Math.min(max, Math.max(min, value));
}

function round1(value: number): number {
  return Math.round(value * 10) / 10;
}

function average(values: number[]): number {
  if (values.length === 0) {
    return 0;
  }
  const total = values.reduce((sum, value) => sum + value, 0);
  return total / values.length;
}

function estimateRepCount(frames: PoseAngleFrame[]): number {
  if (frames.length < 2) {
    return 0;
  }

  let reps = 0;
  let inBottomPosition = false;

  for (const frame of frames) {
    const averageKnee = (frame.leftKneeDeg + frame.rightKneeDeg) / 2;
    const kneeDepth = 180 - averageKnee;

    if (!inBottomPosition && kneeDepth >= 55) {
      inBottomPosition = true;
      reps += 1;
      continue;
    }

    if (inBottomPosition && kneeDepth <= 24) {
      inBottomPosition = false;
    }
  }

  return reps;
}

function defaultSummary(movementType: FormCheckMovementType): FormCheckPoseSummary {
  return {
    movementType,
    sampleCount: 0,
    repetitionCount: 0,
    rangeOfMotionDegrees: 0,
    rangeOfMotionScore: 0,
    kneeTrackingScore: 0,
    symmetryScore: 0,
    overallScore: 0,
    feedback: ['Record a longer set for a reliable form check.'],
    minLeftKneeDeg: 0,
    minRightKneeDeg: 0,
    maxLeftKneeDeg: 0,
    maxRightKneeDeg: 0,
  };
}

export function summarizePoseFrames(input: {
  movementType: FormCheckMovementType;
  frames: PoseAngleFrame[];
}): FormCheckPoseSummary {
  const frames = input.frames.filter(frame =>
    Number.isFinite(frame.leftKneeDeg) && Number.isFinite(frame.rightKneeDeg),
  );

  if (frames.length < 5) {
    return defaultSummary(input.movementType);
  }

  const leftKneeAngles = frames.map(frame => frame.leftKneeDeg);
  const rightKneeAngles = frames.map(frame => frame.rightKneeDeg);

  const minLeftKneeDeg = Math.min(...leftKneeAngles);
  const maxLeftKneeDeg = Math.max(...leftKneeAngles);
  const minRightKneeDeg = Math.min(...rightKneeAngles);
  const maxRightKneeDeg = Math.max(...rightKneeAngles);

  const leftRom = maxLeftKneeDeg - minLeftKneeDeg;
  const rightRom = maxRightKneeDeg - minRightKneeDeg;
  const averageRom = (leftRom + rightRom) / 2;

  const kneeGapValues = frames.map(frame => Math.abs(frame.leftKneeDeg - frame.rightKneeDeg));
  const averageKneeGap = average(kneeGapValues);

  const romDelta = Math.abs(leftRom - rightRom);
  const depthDelta = Math.abs(minLeftKneeDeg - minRightKneeDeg);

  const rangeOfMotionScore = clamp(Math.round((averageRom / 95) * 100), 0, 100);
  const kneeTrackingScore = clamp(Math.round(100 - averageKneeGap * 2.2), 0, 100);
  const symmetryScore = clamp(Math.round(100 - romDelta * 2.4 - depthDelta * 1.2), 0, 100);
  const overallScore = clamp(
    Math.round(rangeOfMotionScore * 0.45 + kneeTrackingScore * 0.3 + symmetryScore * 0.25),
    0,
    100,
  );

  const feedback: string[] = [];
  if (averageRom < 50) {
    feedback.push('Increase depth to improve squat range of motion.');
  }
  if (averageKneeGap > 15) {
    feedback.push('Keep knees tracking evenly over the mid-foot.');
  }
  if (symmetryScore < 70) {
    feedback.push('Work on left/right symmetry and controlled descent.');
  }
  if (feedback.length === 0) {
    feedback.push('Solid rep quality across depth, tracking, and symmetry.');
  }

  return {
    movementType: input.movementType,
    sampleCount: frames.length,
    repetitionCount: estimateRepCount(frames),
    rangeOfMotionDegrees: round1(averageRom),
    rangeOfMotionScore,
    kneeTrackingScore,
    symmetryScore,
    overallScore,
    feedback,
    minLeftKneeDeg: round1(minLeftKneeDeg),
    minRightKneeDeg: round1(minRightKneeDeg),
    maxLeftKneeDeg: round1(maxLeftKneeDeg),
    maxRightKneeDeg: round1(maxRightKneeDeg),
  };
}
