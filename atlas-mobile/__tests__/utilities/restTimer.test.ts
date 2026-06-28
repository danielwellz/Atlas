import {
  createRestTimerState,
  formatRestTimer,
  pauseRestTimerState,
  resumeRestTimerState,
  tickRestTimerState,
} from '../../src/utils/restTimer';

describe('formatRestTimer', () => {
  it('formats positive values as mm:ss', () => {
    expect(formatRestTimer(125)).toBe('02:05');
  });

  it('clamps negative values to 00:00', () => {
    expect(formatRestTimer(-4)).toBe('00:00');
  });
});

describe('rest timer state helpers', () => {
  it('counts down and completes when elapsed', () => {
    const started = createRestTimerState(10, 0);
    const midway = tickRestTimerState(started, 4_100);
    const finished = tickRestTimerState(midway, 10_001);

    expect(midway.remainingSeconds).toBe(6);
    expect(midway.isRunning).toBe(true);
    expect(finished.remainingSeconds).toBe(0);
    expect(finished.isRunning).toBe(false);
  });

  it('pauses and resumes without losing remaining time', () => {
    const started = createRestTimerState(20, 1_000);
    const paused = pauseRestTimerState(started, 6_200);
    const resumed = resumeRestTimerState(paused, 10_000);
    const afterResume = tickRestTimerState(resumed, 12_001);

    expect(paused.isRunning).toBe(false);
    expect(paused.remainingSeconds).toBe(15);
    expect(afterResume.isRunning).toBe(true);
    expect(afterResume.remainingSeconds).toBe(13);
  });
});
