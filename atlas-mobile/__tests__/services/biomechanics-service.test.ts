import { getExerciseBiomechanics } from '../../src/api/services/biomechanicsService';

const mockGet = jest.fn();

jest.mock('../../src/api/client', () => ({
  atlasApiClient: {
    GET: (...args: unknown[]) => mockGet(...args),
  },
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}));

describe('getExerciseBiomechanics', () => {
  beforeEach(() => {
    mockGet.mockReset();
  });

  it('calls the typed API client instead of raw fetch', async () => {
    const biomechanics = {
      exerciseId: 'ex-1',
      exerciseSlug: 'back-squat',
      exerciseName: 'Back Squat',
      animationAssetKey: 'k',
      animationAssetUri: 'u',
      rigVersion: 'v1',
      muscleHighlights: [],
      jointAngles: [],
      metadata: {},
    };
    mockGet.mockResolvedValue({ data: { biomechanics } });

    await expect(
      getExerciseBiomechanics({ accessToken: 'token-1', exerciseId: 'ex-1' }, false),
    ).resolves.toEqual(biomechanics);
    expect(mockGet).toHaveBeenCalledWith('/api/v1/exercises/{id}/biomechanics', {
      params: { path: { id: 'ex-1' } },
      headers: { Authorization: 'Bearer token-1' },
    });
  });

  it('throws the fallback message on an error response', async () => {
    mockGet.mockResolvedValue({ data: undefined, error: { message: 'nope' } });

    await expect(
      getExerciseBiomechanics({ accessToken: 'token-1', exerciseId: 'ex-1' }, false),
    ).rejects.toThrow('Unable to load biomechanics preview.');
  });
});
