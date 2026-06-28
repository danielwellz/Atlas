import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Platform } from 'react-native';
import {
  addWorkoutSetLog,
  completeWorkout,
  getExerciseSubstitutes,
  getCurrentSessionPlan,
  getWorkoutDetail,
  getWorkoutHistory,
  startWorkout,
  type AddWorkoutSetLogInput,
  type CompleteWorkoutInput,
  type ExerciseSubstitutesInput,
  type StartWorkoutInput,
  type WorkoutDetailInput,
  type WorkoutHistoryInput,
} from '../../api/services/workoutService';
import { trackProductEvent } from '../../analytics/eventClient';
import { isNetworkOnline } from '../../network/onlineManager';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';
import { enqueueWorkoutSetOutboxItem, flushOutbox } from '../../sync/outbox';

function resolveWorkoutMode(isMockMode: boolean): boolean {
  return __DEV__ && isMockMode;
}

function currentSessionPlanQueryKey(userId: string | undefined, mockModeEnabled: boolean) {
  return ['workout', 'session-plan', userId ?? 'anonymous', mockModeEnabled] as const;
}

function workoutDetailQueryKey(
  userId: string | undefined,
  mockModeEnabled: boolean,
  workoutId: string,
) {
  return ['workout', 'detail', userId ?? 'anonymous', mockModeEnabled, workoutId] as const;
}

function workoutHistoryQueryKey(
  userId: string | undefined,
  mockModeEnabled: boolean,
  limit: number,
  cursor: string | undefined,
) {
  return ['workout', 'history', userId ?? 'anonymous', mockModeEnabled, limit, cursor ?? ''] as const;
}

function workoutHistoryPrefixQueryKey(userId: string | undefined, mockModeEnabled: boolean) {
  return ['workout', 'history', userId ?? 'anonymous', mockModeEnabled] as const;
}

type StartWorkoutMutationInput = Omit<StartWorkoutInput, 'accessToken'>;
type AddWorkoutSetMutationInput = Omit<AddWorkoutSetLogInput, 'accessToken'> & {
  setIndex: number;
};
type CompleteWorkoutMutationInput = Omit<CompleteWorkoutInput, 'accessToken'>;
type ExerciseSubstitutesMutationInput = Omit<ExerciseSubstitutesInput, 'accessToken'>;
type WorkoutDetailQueryInput = Omit<WorkoutDetailInput, 'accessToken'>;
type WorkoutHistoryQueryInput = Omit<WorkoutHistoryInput, 'accessToken'>;
type WorkoutSet = Awaited<ReturnType<typeof addWorkoutSetLog>>;

function createOfflineWorkoutSet(input: AddWorkoutSetMutationInput): WorkoutSet {
  const now = new Date().toISOString();

  return {
    id: `offline-${input.idempotencyKey}`,
    workoutExerciseId: input.workoutExerciseId,
    setIndex: input.setIndex,
    reps: input.reps,
    weightKg: input.weightKg,
    rpe: input.rpe ?? null,
    completedAt: now,
    createdAt: now,
  };
}

export function useCurrentSessionPlanQuery() {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const useMockWorkouts = resolveWorkoutMode(isMockMode);
  const accessToken = session?.tokens.accessToken ?? '';

  return useQuery({
    queryKey: currentSessionPlanQueryKey(session?.user.id, useMockWorkouts),
    queryFn: () =>
      getCurrentSessionPlan(
        {
          accessToken,
        },
        useMockWorkouts,
      ),
    enabled: useMockWorkouts || Boolean(accessToken),
  });
}

export function useStartWorkoutMutation() {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const queryClient = useQueryClient();
  const useMockWorkouts = resolveWorkoutMode(isMockMode);
  const accessToken = session?.tokens.accessToken ?? '';

  return useMutation({
    mutationFn: async (input: StartWorkoutMutationInput = {}) =>
      startWorkout(
        {
          accessToken,
          programSessionId: input.programSessionId ?? null,
        },
        useMockWorkouts,
      ),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: currentSessionPlanQueryKey(session?.user.id, useMockWorkouts),
        }),
        queryClient.invalidateQueries({
          queryKey: workoutHistoryPrefixQueryKey(session?.user.id, useMockWorkouts),
        }),
      ]);
    },
  });
}

export function useAddWorkoutSetMutation() {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const queryClient = useQueryClient();
  const useMockWorkouts = resolveWorkoutMode(isMockMode);
  const accessToken = session?.tokens.accessToken ?? '';

  return useMutation({
    mutationFn: async (input: AddWorkoutSetMutationInput) => {
      const { setIndex, ...payload } = input;

      if (!useMockWorkouts && !isNetworkOnline()) {
        await enqueueWorkoutSetOutboxItem({
          ...payload,
          setIndex,
        });
        return createOfflineWorkoutSet(input);
      }

      return addWorkoutSetLog(
        {
          accessToken,
          ...payload,
        },
        useMockWorkouts,
      );
    },
    onSuccess: async (data, variables) => {
      const isOfflineSet = data.id.startsWith('offline-');

      if (!isOfflineSet) {
        await Promise.all([
          queryClient.invalidateQueries({
            queryKey: workoutDetailQueryKey(session?.user.id, useMockWorkouts, variables.workoutId),
          }),
          queryClient.invalidateQueries({
            queryKey: workoutHistoryPrefixQueryKey(session?.user.id, useMockWorkouts),
          }),
        ]);
      }

      if (!useMockWorkouts && isNetworkOnline()) {
        const flushResult = await flushOutbox(accessToken);
        if (flushResult.flushedCount > 0) {
          await Promise.all([
            queryClient.invalidateQueries({ queryKey: ['dashboard'] }),
            queryClient.invalidateQueries({
              queryKey: workoutHistoryPrefixQueryKey(session?.user.id, useMockWorkouts),
            }),
          ]);
        }
      }
    },
  });
}

export function useCompleteWorkoutMutation() {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const queryClient = useQueryClient();
  const useMockWorkouts = resolveWorkoutMode(isMockMode);
  const accessToken = session?.tokens.accessToken ?? '';

  return useMutation({
    mutationFn: async (input: CompleteWorkoutMutationInput) =>
      completeWorkout(
        {
          accessToken,
          ...input,
        },
        useMockWorkouts,
      ),
    onSuccess: async (data, variables) => {
      const exerciseCount = data.exercises.length;
      const setCount = data.exercises.reduce((acc, exercise) => acc + exercise.sets.length, 0);
      const startedAtMs = Date.parse(data.startedAt);
      const completedAtMs = data.completedAt ? Date.parse(data.completedAt) : NaN;
      const durationMinutes =
        Number.isFinite(startedAtMs) && Number.isFinite(completedAtMs) && completedAtMs > startedAtMs
          ? Math.max(1, Math.round((completedAtMs - startedAtMs) / 60000))
          : 0;

      await trackProductEvent({
        accessToken,
        eventName: 'workout_completed',
        consentGranted: true,
        useMockMode: isMockMode,
        properties: {
          workout_id: data.id,
          duration_minutes: durationMinutes,
          exercise_count: exerciseCount,
          set_count: setCount,
          completion_source: 'workout_runner',
          platform: Platform.OS,
          app_version: '0.0.1',
        },
      });

      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: workoutDetailQueryKey(session?.user.id, useMockWorkouts, variables.workoutId),
        }),
        queryClient.invalidateQueries({
          queryKey: workoutHistoryPrefixQueryKey(session?.user.id, useMockWorkouts),
        }),
        queryClient.invalidateQueries({
          queryKey: currentSessionPlanQueryKey(session?.user.id, useMockWorkouts),
        }),
        queryClient.invalidateQueries({ queryKey: ['dashboard'] }),
      ]);
    },
  });
}

export function useExerciseSubstitutesMutation() {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const useMockWorkouts = resolveWorkoutMode(isMockMode);
  const accessToken = session?.tokens.accessToken ?? '';

  return useMutation({
    mutationFn: async (input: ExerciseSubstitutesMutationInput) =>
      getExerciseSubstitutes(
        {
          accessToken,
          ...input,
        },
        useMockWorkouts,
      ),
  });
}

export function useWorkoutDetailQuery(input: WorkoutDetailQueryInput | null) {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const useMockWorkouts = resolveWorkoutMode(isMockMode);
  const accessToken = session?.tokens.accessToken ?? '';
  const workoutId = input?.workoutId;

  return useQuery({
    queryKey: workoutId
      ? workoutDetailQueryKey(session?.user.id, useMockWorkouts, workoutId)
      : ['workout', 'detail', session?.user.id ?? 'anonymous', useMockWorkouts, 'none'],
    queryFn: () =>
      getWorkoutDetail(
        {
          accessToken,
          workoutId: workoutId!,
        },
        useMockWorkouts,
      ),
    enabled: (useMockWorkouts || Boolean(accessToken)) && Boolean(workoutId),
  });
}

export function useWorkoutHistoryQuery(input: WorkoutHistoryQueryInput = {}) {
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const useMockWorkouts = resolveWorkoutMode(isMockMode);
  const accessToken = session?.tokens.accessToken ?? '';
  const limit = input.limit ?? 20;

  return useQuery({
    queryKey: workoutHistoryQueryKey(session?.user.id, useMockWorkouts, limit, input.cursor),
    queryFn: () =>
      getWorkoutHistory(
        {
          accessToken,
          limit,
          cursor: input.cursor,
        },
        useMockWorkouts,
      ),
    enabled: useMockWorkouts || Boolean(accessToken),
  });
}
