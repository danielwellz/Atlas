import {
  DEFAULT_DASHBOARD,
  WORKOUT_PLAN_BY_PROGRAM,
} from '../mockData';
import type { DashboardData, WorkoutPlan } from '../types';

const NETWORK_LATENCY_MS = 220;
const DEFAULT_MOCK_PROGRAM_ID = Object.keys(WORKOUT_PLAN_BY_PROGRAM)[0] ?? '';
let dashboardState: DashboardData = {
  ...DEFAULT_DASHBOARD,
  habits: DEFAULT_DASHBOARD.habits.map(habit => ({ ...habit })),
};

type SetLogInput = {
  exerciseId: string;
  setIndex: number;
  reps: number;
  weightKg: number;
};

const setLogs: Record<string, SetLogInput[]> = {};

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => {
    setTimeout(resolve, ms);
  });
}

export async function fetchWorkoutPlan(): Promise<WorkoutPlan> {
  await sleep(NETWORK_LATENCY_MS);

  return (
    WORKOUT_PLAN_BY_PROGRAM[DEFAULT_MOCK_PROGRAM_ID] ?? {
      dayLabel: 'Week 1',
      exercises: [],
    }
  );
}

export async function fetchDashboard(): Promise<DashboardData> {
  await sleep(NETWORK_LATENCY_MS);

  return {
    ...dashboardState,
    habits: dashboardState.habits.map(habit => ({ ...habit })),
  };
}

export async function toggleHabit(habitId: string): Promise<DashboardData> {
  await sleep(NETWORK_LATENCY_MS);

  dashboardState = {
    ...dashboardState,
    habits: dashboardState.habits.map(habit =>
      habit.id === habitId
        ? {
            ...habit,
            completed: !habit.completed,
          }
        : habit,
    ),
  };

  return {
    ...dashboardState,
    habits: dashboardState.habits.map(habit => ({ ...habit })),
  };
}

export async function submitNutritionCheckIn(): Promise<DashboardData> {
  await sleep(NETWORK_LATENCY_MS);

  dashboardState = {
    ...dashboardState,
    nutritionCheckedIn: true,
  };

  return {
    ...dashboardState,
    habits: dashboardState.habits.map(habit => ({ ...habit })),
  };
}

export async function logWorkoutSet(input: SetLogInput): Promise<void> {
  await sleep(NETWORK_LATENCY_MS / 3);

  if (!setLogs[input.exerciseId]) {
    setLogs[input.exerciseId] = [];
  }

  const existing = setLogs[input.exerciseId].find(item => item.setIndex === input.setIndex);

  if (existing) {
    existing.reps = input.reps;
    existing.weightKg = input.weightKg;
    return;
  }

  setLogs[input.exerciseId].push(input);
}
