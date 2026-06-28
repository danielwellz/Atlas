function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

export function calculateCompletionRate(completed: number, total: number): number {
  if (total <= 0) {
    return 0;
  }

  const percent = (completed / total) * 100;
  return Math.round(clamp(percent, 0, 100));
}
