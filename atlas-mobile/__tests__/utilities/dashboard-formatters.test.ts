import {
  calculatePercentChange,
  formatDateLabel,
  formatMacro,
  formatMuscleGroupLabel,
  formatSignedPercent,
  formatSignedWeightDeltaKg,
  formatVolumeKg,
  formatWeightKg,
} from '../../src/features/dashboard/formatters';

describe('dashboard formatters', () => {
  it('formats volume and weight values', () => {
    expect(formatVolumeKg(1260.49)).toBe('1260 kg');
    expect(formatWeightKg(83.456)).toBe('83.5 kg');
  });

  it('formats macro values with unit', () => {
    expect(formatMacro(2050.2, 'kcal')).toBe('2050.2 kcal');
  });

  it('formats signed percent and weight delta values', () => {
    expect(formatSignedPercent(4.234)).toBe('+4.2%');
    expect(formatSignedPercent(-1.244)).toBe('-1.2%');
    expect(formatSignedPercent(null)).toBe('No baseline');
    expect(formatSignedWeightDeltaKg(-0.64)).toBe('-0.6 kg');
    expect(formatSignedWeightDeltaKg(0.64)).toBe('+0.6 kg');
  });

  it('formats labels and computes percent change', () => {
    expect(formatMuscleGroupLabel('upper_back')).toBe('Upper back');
    expect(formatDateLabel('2026-02-27T08:00:00.000Z')).toBe('2026-02-27');
    expect(calculatePercentChange(100, 80)).toBeCloseTo(25);
    expect(calculatePercentChange(100, 0)).toBeNull();
  });
});
