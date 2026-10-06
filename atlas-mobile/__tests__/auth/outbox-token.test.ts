import AsyncStorage from '@react-native-async-storage/async-storage';
import { addWorkoutSetLog } from '../../src/api/services/workoutService';
import { resetTokenManagerForTests, setTokens } from '../../src/auth/tokenManager';
import { setNetworkOnlineForTests } from '../../src/network/onlineManager';
import { enqueueWorkoutSetOutboxItem, flushOutbox } from '../../src/sync/outbox';
import { installFetchStub, jsonResponse, jwtWithExp, tokens } from '../../testUtils/authFetchStub';

jest.mock('../../src/api/services/workoutService', () => ({
  ...jest.requireActual('../../src/api/services/workoutService'),
  addWorkoutSetLog: jest.fn(async () => ({})),
}));

describe('outbox flush token', () => {
  beforeEach(async () => {
    resetTokenManagerForTests();
    jest.clearAllMocks();
    await AsyncStorage.clear();
    setNetworkOnlineForTests(true);
  });

  it('uses a freshly refreshed token at flush time, not the one it was handed', async () => {
    const nowSeconds = Math.floor(Date.now() / 1000);
    setTokens(tokens('1', jwtWithExp(nowSeconds - 60, 'old')));
    const fresh = tokens('2', jwtWithExp(nowSeconds + 900, 'new'));
    installFetchStub(() => jsonResponse(200, fresh));

    await enqueueWorkoutSetOutboxItem({
      workoutId: 'workout-1',
      workoutExerciseId: 'exercise-1',
      setIndex: 1,
      reps: 8,
      weightKg: 80,
      rpe: null,
      idempotencyKey: 'set-1',
    });

    const result = await flushOutbox('stale-session-token');

    expect(result).toEqual({ flushedCount: 1, pendingCount: 0 });
    expect(addWorkoutSetLog).toHaveBeenCalledWith(
      expect.objectContaining({ accessToken: fresh.accessToken }),
      false,
    );
  });
});
