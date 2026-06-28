function padNumber(value: number): string {
  return value.toString().padStart(2, '0');
}

export function formatRestTimer(totalSeconds: number): string {
  const normalized = Math.max(0, Math.floor(totalSeconds));
  const minutes = Math.floor(normalized / 60);
  const seconds = normalized % 60;

  return `${padNumber(minutes)}:${padNumber(seconds)}`;
}

export type RestTimerState = {
  endAtMs: number | null;
  remainingSeconds: number;
  isRunning: boolean;
};

function normalizeSeconds(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }

  return Math.max(0, Math.floor(value));
}

export function createRestTimerState(totalSeconds: number, nowMs: number = Date.now()): RestTimerState {
  const normalized = normalizeSeconds(totalSeconds);
  if (normalized === 0) {
    return {
      endAtMs: null,
      remainingSeconds: 0,
      isRunning: false,
    };
  }

  return {
    endAtMs: nowMs + normalized * 1000,
    remainingSeconds: normalized,
    isRunning: true,
  };
}

export function stopRestTimerState(): RestTimerState {
  return {
    endAtMs: null,
    remainingSeconds: 0,
    isRunning: false,
  };
}

export function tickRestTimerState(
  current: RestTimerState,
  nowMs: number = Date.now(),
): RestTimerState {
  if (!current.isRunning || current.endAtMs === null) {
    return current;
  }

  const remainingSeconds = normalizeSeconds(Math.ceil((current.endAtMs - nowMs) / 1000));
  if (remainingSeconds === 0) {
    return stopRestTimerState();
  }

  if (remainingSeconds === current.remainingSeconds) {
    return current;
  }

  return {
    ...current,
    remainingSeconds,
  };
}

export function pauseRestTimerState(
  current: RestTimerState,
  nowMs: number = Date.now(),
): RestTimerState {
  if (!current.isRunning) {
    return current;
  }

  const ticked = tickRestTimerState(current, nowMs);
  if (!ticked.isRunning) {
    return ticked;
  }

  return {
    endAtMs: null,
    remainingSeconds: ticked.remainingSeconds,
    isRunning: false,
  };
}

export function resumeRestTimerState(
  current: RestTimerState,
  nowMs: number = Date.now(),
): RestTimerState {
  if (current.isRunning || current.remainingSeconds <= 0) {
    return current;
  }

  return {
    endAtMs: nowMs + current.remainingSeconds * 1000,
    remainingSeconds: current.remainingSeconds,
    isRunning: true,
  };
}
