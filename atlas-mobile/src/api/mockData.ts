import type { DashboardData, Program, WeekScheduleItem, WorkoutPlan } from './types';

export const PROGRAM_LIBRARY: Omit<Program, 'isEnrolled'>[] = [
  {
    id: 'strength-foundation',
    name: 'Strength Foundation',
    goal: 'Build baseline full-body strength',
    durationWeeks: 8,
    equipment: ['Dumbbells', 'Bench'],
    summary: 'Progressive overload with a 3-day split for consistent strength gains.',
  },
  {
    id: 'athletic-lean',
    name: 'Athletic Lean',
    goal: 'Body recomposition with conditioning',
    durationWeeks: 6,
    equipment: ['Bodyweight', 'Resistance Bands'],
    summary: 'Hybrid strength and conditioning sessions designed for busy schedules.',
  },
  {
    id: 'endurance-builder',
    name: 'Endurance Builder',
    goal: 'Improve aerobic capacity and durability',
    durationWeeks: 10,
    equipment: ['Bodyweight', 'Treadmill'],
    summary: 'Structured intervals and zone work to increase stamina week over week.',
  },
];

export const WEEK_SCHEDULE_BY_PROGRAM: Record<string, WeekScheduleItem[]> = {
  'strength-foundation': [
    { day: 'Monday', workoutName: 'Lower Body Strength', focus: 'Squat and hinge', durationMinutes: 55 },
    { day: 'Wednesday', workoutName: 'Upper Push/Pull', focus: 'Chest and back', durationMinutes: 50 },
    { day: 'Friday', workoutName: 'Full Body Power', focus: 'Compound lifts', durationMinutes: 60 },
  ],
  'athletic-lean': [
    { day: 'Tuesday', workoutName: 'Metabolic Circuit', focus: 'Conditioning', durationMinutes: 40 },
    { day: 'Thursday', workoutName: 'Strength Intervals', focus: 'Tempo lifting', durationMinutes: 45 },
    { day: 'Saturday', workoutName: 'Field Session', focus: 'Athletic movement', durationMinutes: 50 },
  ],
  'endurance-builder': [
    { day: 'Monday', workoutName: 'Zone 2 Base', focus: 'Steady effort', durationMinutes: 45 },
    { day: 'Wednesday', workoutName: 'Tempo Intervals', focus: 'Threshold pace', durationMinutes: 50 },
    { day: 'Sunday', workoutName: 'Long Session', focus: 'Durability', durationMinutes: 70 },
  ],
};

export const WORKOUT_PLAN_BY_PROGRAM: Record<string, WorkoutPlan> = {
  'strength-foundation': {
    dayLabel: 'Week 1 • Monday',
    exercises: [
      { id: 'back-squat', name: 'Back Squat', targetSets: 4, targetReps: '5', restSeconds: 120 },
      { id: 'romanian-deadlift', name: 'Romanian Deadlift', targetSets: 3, targetReps: '8', restSeconds: 90 },
      { id: 'walking-lunge', name: 'Walking Lunge', targetSets: 3, targetReps: '10/leg', restSeconds: 60 },
    ],
  },
  'athletic-lean': {
    dayLabel: 'Week 1 • Tuesday',
    exercises: [
      { id: 'thruster', name: 'DB Thruster', targetSets: 4, targetReps: '10', restSeconds: 75 },
      { id: 'burpee', name: 'Burpees', targetSets: 3, targetReps: '12', restSeconds: 60 },
      { id: 'mountain-climber', name: 'Mountain Climbers', targetSets: 3, targetReps: '30 sec', restSeconds: 45 },
    ],
  },
  'endurance-builder': {
    dayLabel: 'Week 1 • Monday',
    exercises: [
      { id: 'incline-run', name: 'Incline Run', targetSets: 4, targetReps: '4 min', restSeconds: 90 },
      { id: 'split-squat', name: 'Split Squat', targetSets: 3, targetReps: '10/leg', restSeconds: 75 },
      { id: 'plank', name: 'Plank Hold', targetSets: 3, targetReps: '60 sec', restSeconds: 45 },
    ],
  },
};

export const DEFAULT_DASHBOARD: DashboardData = {
  metrics: {
    weeklyWorkoutsCompleted: 3,
    streakDays: 9,
    averageWorkoutMinutes: 52,
  },
  habits: [
    { id: 'water', label: '2L hydration', completed: true },
    { id: 'sleep', label: '7+ hours sleep', completed: false },
    { id: 'steps', label: '8k steps', completed: false },
  ],
  nutritionCheckedIn: false,
};
