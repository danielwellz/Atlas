import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { normalizeUTCDateKey, toUTCDateKey } from '../../api/dateKey';
import { getDashboardSummary } from '../../api/services/dashboardService';
import {
  completeMomentumSprintChecklistEntry,
  getMomentumSprintStatus,
  listHabits,
  toggleHabitDailyLog,
  type MomentumSprintStatus,
} from '../../api/services/habitService';
import { isNetworkOnline } from '../../network/onlineManager';
import {
  getNutritionTargets,
  upsertDailyNutritionCheckin,
  type UpsertDailyNutritionCheckinInput,
} from '../../api/services/nutritionService';
import {
  upsertDashboardReadinessCheckin,
  type UpsertReadinessCheckinInput,
} from '../../api/services/readinessService';
import { useAuth } from '../../state/AuthContext';
import { enqueueNutritionOutboxItem, flushOutbox } from '../../sync/outbox';

type DashboardHabit = {
  id: string;
  label: string;
  completed: boolean;
};

export type DashboardQueryData = {
  summary: Awaited<ReturnType<typeof getDashboardSummary>>;
  habits: DashboardHabit[];
  nutritionCheckedIn: boolean;
  momentumSprint: MomentumSprintStatus;
  dateKey: string;
};

type NutritionCheckinMutationInput = Omit<
  UpsertDailyNutritionCheckinInput,
  'accessToken' | 'dateKey'
> & {
  dateKey?: string;
};
type NutritionCheckin = Awaited<ReturnType<typeof upsertDailyNutritionCheckin>>;
type ReadinessCheckinMutationInput = Omit<
  UpsertReadinessCheckinInput,
  'accessToken'
>;

function dashboardQueryKey(userID: string | undefined, dateKey: string) {
  return ['dashboard', userID ?? 'anonymous', dateKey] as const;
}

function dashboardQueryPrefix(userID: string | undefined) {
  return ['dashboard', userID ?? 'anonymous'] as const;
}

export function useDashboardQuery() {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';
  const dateKey = toUTCDateKey();

  return useQuery({
    queryKey: dashboardQueryKey(session?.user.id, dateKey),
    queryFn: async (): Promise<DashboardQueryData> => {
      const [summary, habits, nutritionToday, momentumSprint] =
        await Promise.all([
          getDashboardSummary({ accessToken }),
          listHabits({
            accessToken,
            dateKey,
          }),
          getNutritionTargets({ accessToken }),
          getMomentumSprintStatus({ accessToken }),
        ]);

      return {
        summary,
        habits: habits
          .filter(habit => habit.active)
          .map(habit => ({
            id: habit.id,
            label: habit.name,
            completed: habit.completed,
          })),
        nutritionCheckedIn: Boolean(nutritionToday.checkin),
        momentumSprint,
        dateKey,
      };
    },
    enabled: Boolean(accessToken),
  });
}

type MomentumSprintMutationInput = {
  habitKey: string;
  completed: boolean;
  dateKey?: string;
};

export function useMomentumSprintChecklistMutation() {
  const { session } = useAuth();
  const queryClient = useQueryClient();
  const accessToken = session?.tokens.accessToken ?? '';

  return useMutation({
    mutationFn: (input: MomentumSprintMutationInput) =>
      completeMomentumSprintChecklistEntry({
        accessToken,
        habitKey: input.habitKey,
        completed: input.completed,
        dateKey: input.dateKey,
      }),
    onSuccess: async status => {
      queryClient.setQueriesData<DashboardQueryData>(
        { queryKey: dashboardQueryPrefix(session?.user.id) },
        current => {
          if (!current) {
            return current;
          }

          return {
            ...current,
            momentumSprint: status,
          };
        },
      );

      await queryClient.invalidateQueries({
        queryKey: dashboardQueryPrefix(session?.user.id),
      });
    },
  });
}

export function useToggleHabitMutation() {
  const { session } = useAuth();
  const queryClient = useQueryClient();
  const accessToken = session?.tokens.accessToken ?? '';

  return useMutation({
    mutationFn: (habitId: string) =>
      toggleHabitDailyLog({
        accessToken,
        habitId,
      }),
    onSuccess: async (log, habitID) => {
      queryClient.setQueriesData<DashboardQueryData>(
        { queryKey: dashboardQueryPrefix(session?.user.id) },
        current => {
          if (!current || current.dateKey !== normalizeUTCDateKey(log.date)) {
            return current;
          }

          return {
            ...current,
            habits: current.habits.map(habit =>
              habit.id === habitID
                ? {
                    ...habit,
                    completed: log.completed,
                  }
                : habit,
            ),
          };
        },
      );

      await queryClient.invalidateQueries({
        queryKey: dashboardQueryPrefix(session?.user.id),
      });

      if (isNetworkOnline()) {
        const flushResult = await flushOutbox(accessToken);
        if (flushResult.flushedCount > 0) {
          await queryClient.invalidateQueries({
            queryKey: dashboardQueryPrefix(session?.user.id),
          });
        }
      }
    },
  });
}

export function useNutritionCheckInMutation() {
  const { session } = useAuth();
  const queryClient = useQueryClient();
  const accessToken = session?.tokens.accessToken ?? '';

  return useMutation({
    mutationFn: async (input: NutritionCheckinMutationInput = {}) => {
      const dateKey = normalizeUTCDateKey(input.dateKey);

      if (!isNetworkOnline()) {
        const idempotencyKey = `nutrition-${dateKey}`;
        await enqueueNutritionOutboxItem({
          idempotencyKey,
          dateKey,
          caloriesEstimate: input.caloriesEstimate,
          proteinGEstimate: input.proteinGEstimate,
          notes: input.notes,
        });

        const now = new Date().toISOString();
        const optimisticCheckin: NutritionCheckin = {
          id: `offline-${idempotencyKey}`,
          userId: session?.user.id ?? 'offline-user',
          date: dateKey,
          calories_estimate: input.caloriesEstimate ?? null,
          protein_g_estimate: input.proteinGEstimate ?? null,
          hit_calories: false,
          hit_protein: false,
          notes: input.notes ?? '',
          createdAt: now,
        };
        return optimisticCheckin;
      }

      return upsertDailyNutritionCheckin({
        accessToken,
        dateKey,
        caloriesEstimate: input.caloriesEstimate,
        proteinGEstimate: input.proteinGEstimate,
        notes: input.notes,
      });
    },
    onSuccess: async checkin => {
      queryClient.setQueriesData<DashboardQueryData>(
        { queryKey: dashboardQueryPrefix(session?.user.id) },
        current => {
          if (
            !current ||
            current.dateKey !== normalizeUTCDateKey(checkin.date)
          ) {
            return current;
          }

          return {
            ...current,
            nutritionCheckedIn: true,
          };
        },
      );

      if (isNetworkOnline()) {
        const flushResult = await flushOutbox(accessToken);
        if (
          flushResult.flushedCount > 0 ||
          !checkin.id.startsWith('offline-')
        ) {
          await queryClient.invalidateQueries({
            queryKey: dashboardQueryPrefix(session?.user.id),
          });
        }
      }
    },
  });
}

export function useReadinessCheckInMutation() {
  const { session } = useAuth();
  const queryClient = useQueryClient();
  const accessToken = session?.tokens.accessToken ?? '';

  return useMutation({
    mutationFn: (input: ReadinessCheckinMutationInput) =>
      upsertDashboardReadinessCheckin({
        accessToken,
        dateKey: input.dateKey,
        energyLevel: input.energyLevel,
        sleepQuality: input.sleepQuality,
        stressLevel: input.stressLevel,
      }),
    onSuccess: async checkin => {
      queryClient.setQueriesData<DashboardQueryData>(
        { queryKey: dashboardQueryPrefix(session?.user.id) },
        current => {
          if (!current) {
            return current;
          }

          const checkinDate = normalizeUTCDateKey(checkin.date);
          const existingHistory =
            current.summary.readinessSelfReportHistory.filter(
              point => normalizeUTCDateKey(point.date) !== checkinDate,
            );
          const nextHistory = [
            ...existingHistory,
            {
              date: checkin.date,
              readinessScore: checkin.readinessScore,
              energyLevel: checkin.energyLevel,
              sleepQuality: checkin.sleepQuality,
              stressLevel: checkin.stressLevel,
            },
          ].sort((left, right) => left.date.localeCompare(right.date));

          return {
            ...current,
            summary: {
              ...current.summary,
              readinessSelfReportHistory: nextHistory,
            },
          };
        },
      );

      await queryClient.invalidateQueries({
        queryKey: dashboardQueryPrefix(session?.user.id),
      });
    },
  });
}
