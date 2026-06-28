const DAY_ORDER = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

export function summarizeSchedule(days: string[]): string {
  if (days.length === 0) {
    return 'No training days selected';
  }

  if (days.length === 7) {
    return 'Training every day';
  }

  const sortedDays = [...new Set(days)].sort(
    (left, right) => DAY_ORDER.indexOf(left) - DAY_ORDER.indexOf(right),
  );

  return sortedDays.join(' • ');
}
