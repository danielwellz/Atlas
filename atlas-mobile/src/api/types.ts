export type Program = {
  id: string;
  name: string;
  goal: string;
  durationWeeks: number;
  equipment: string[];
  summary: string;
  isEnrolled: boolean;
};

export type WeekScheduleItem = {
  day: string;
  workoutName: string;
  focus: string;
  durationMinutes: number;
};

export type WorkoutExercise = {
  id: string;
  name: string;
  targetSets: number;
  targetReps: string;
  restSeconds: number;
};

export type WorkoutPlan = {
  dayLabel: string;
  exercises: WorkoutExercise[];
};

export type DashboardMetrics = {
  weeklyWorkoutsCompleted: number;
  streakDays: number;
  averageWorkoutMinutes: number;
};

export type Habit = {
  id: string;
  label: string;
  completed: boolean;
};

export type DashboardData = {
  metrics: DashboardMetrics;
  habits: Habit[];
  nutritionCheckedIn: boolean;
};
