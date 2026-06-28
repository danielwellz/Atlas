const DATE_KEY_PATTERN = /^\d{4}-\d{2}-\d{2}$/;

export function toUTCDateKey(value: Date = new Date()): string {
  return value.toISOString().slice(0, 10);
}

export function normalizeUTCDateKey(value?: string | null): string {
  if (!value) {
    return toUTCDateKey();
  }

  if (!DATE_KEY_PATTERN.test(value)) {
    throw new Error('Invalid date key format. Expected YYYY-MM-DD.');
  }

  return value;
}

