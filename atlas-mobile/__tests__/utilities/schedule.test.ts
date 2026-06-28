import { summarizeSchedule } from '../../src/utils/schedule';

describe('summarizeSchedule', () => {
  it('returns fallback for empty schedules', () => {
    expect(summarizeSchedule([])).toBe('No training days selected');
  });

  it('sorts and de-duplicates days in calendar order', () => {
    expect(summarizeSchedule(['Friday', 'Monday', 'Monday', 'Wednesday'])).toBe(
      'Monday • Wednesday • Friday',
    );
  });
});
