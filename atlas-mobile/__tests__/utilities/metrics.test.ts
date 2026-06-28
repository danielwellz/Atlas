import { calculateCompletionRate } from '../../src/utils/metrics';

describe('calculateCompletionRate', () => {
  it('returns rounded percentage', () => {
    expect(calculateCompletionRate(7, 12)).toBe(58);
  });

  it('returns 0 when denominator is zero', () => {
    expect(calculateCompletionRate(4, 0)).toBe(0);
  });

  it('clamps to 100', () => {
    expect(calculateCompletionRate(20, 10)).toBe(100);
  });
});
