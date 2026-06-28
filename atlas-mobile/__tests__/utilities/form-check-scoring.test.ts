import { summarizePoseFrames, type PoseAngleFrame } from '../../src/features/formCheck/scoring';

function buildFrame(
  timestampMs: number,
  leftKneeDeg: number,
  rightKneeDeg: number,
  leftHipDeg: number,
  rightHipDeg: number,
): PoseAngleFrame {
  return {
    timestampMs,
    leftKneeDeg,
    rightKneeDeg,
    leftHipDeg,
    rightHipDeg,
  };
}

describe('form-check scoring', () => {
  it('scores deep and symmetric squat reps highly', () => {
    const frames: PoseAngleFrame[] = [];

    for (let index = 0; index < 80; index += 1) {
      const phase = index / 8;
      const depth = (Math.sin(phase) + 1) / 2;
      const knee = 172 - depth * 95;
      const hip = 168 - depth * 76;
      frames.push(buildFrame(index * 120, knee + 1.5, knee - 1.5, hip + 1.0, hip - 1.0));
    }

    const summary = summarizePoseFrames({
      movementType: 'squat',
      frames,
    });

    expect(summary.sampleCount).toBe(80);
    expect(summary.repetitionCount).toBeGreaterThan(1);
    expect(summary.rangeOfMotionDegrees).toBeGreaterThan(80);
    expect(summary.overallScore).toBeGreaterThanOrEqual(80);
    expect(summary.kneeTrackingScore).toBeGreaterThanOrEqual(85);
    expect(summary.symmetryScore).toBeGreaterThanOrEqual(85);
  });

  it('penalizes shallow and asymmetric reps', () => {
    const frames: PoseAngleFrame[] = [];

    for (let index = 0; index < 50; index += 1) {
      const phase = index / 10;
      const depth = (Math.sin(phase) + 1) / 2;
      const leftKnee = 175 - depth * 35;
      const rightKnee = 162 - depth * 8 + Math.sin(phase * 2.2) * 18;
      const leftHip = 170 - depth * 22;
      const rightHip = 164 - depth * 8;
      frames.push(buildFrame(index * 120, leftKnee, rightKnee, leftHip, rightHip));
    }

    const summary = summarizePoseFrames({
      movementType: 'squat',
      frames,
    });

    expect(summary.rangeOfMotionDegrees).toBeLessThan(50);
    expect(summary.kneeTrackingScore).toBeLessThan(75);
    expect(summary.symmetryScore).toBeLessThan(85);
    expect(summary.overallScore).toBeLessThan(70);
    expect(summary.feedback.length).toBeGreaterThan(0);
  });

  it('returns a default summary for very short recordings', () => {
    const summary = summarizePoseFrames({
      movementType: 'squat',
      frames: [
        buildFrame(0, 170, 170, 160, 160),
        buildFrame(120, 165, 166, 155, 155),
      ],
    });

    expect(summary.sampleCount).toBe(0);
    expect(summary.overallScore).toBe(0);
    expect(summary.feedback[0]).toMatch(/longer set/i);
  });
});
