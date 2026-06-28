export function formatVolumeKg(value: number): string {
  return `${value.toFixed(0)} kg`;
}

export function formatWeightKg(value: number): string {
  return `${value.toFixed(1)} kg`;
}

export function formatMacro(value: number, unit: string): string {
  return `${value.toFixed(1)} ${unit}`;
}

export function formatSignedPercent(value: number | null): string {
  if (value === null) {
    return 'No baseline';
  }
  const sign = value > 0 ? '+' : '';
  return `${sign}${value.toFixed(1)}%`;
}

export function formatSignedWeightDeltaKg(value: number): string {
  const sign = value > 0 ? '+' : '';
  return `${sign}${value.toFixed(1)} kg`;
}

export function calculatePercentChange(
  current: number,
  previous: number,
): number | null {
  if (previous <= 0) {
    return null;
  }
  return ((current - previous) / previous) * 100;
}

export function formatMuscleGroupLabel(input: string): string {
  const normalized = input.replaceAll('_', ' ').trim();
  if (!normalized) {
    return input;
  }
  return normalized[0].toUpperCase() + normalized.slice(1);
}

export function formatDateLabel(dateIso: string): string {
  const parsed = new Date(dateIso);
  if (Number.isNaN(parsed.getTime())) {
    return dateIso;
  }
  return parsed.toISOString().slice(0, 10);
}
