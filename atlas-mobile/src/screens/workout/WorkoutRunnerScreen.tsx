import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ActivityIndicator,
  AppState,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native';
import { useNavigation } from '@react-navigation/native';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import type { components } from '../../api/generated/openapi';
import { getExerciseBiomechanics } from '../../api/services/biomechanicsService';
import {
  injectProgressionRecommendations,
  readPreviousPerformance,
  readPlannedPrescription,
  readProgressionRecommendation,
  type PreviousPerformanceSet,
  type WorkoutSessionPlan,
} from '../../api/services/workoutService';
import { hasBiomechanicsOverlayEntitlement } from '../../features/entitlements';
import {
  useAddWorkoutSetMutation,
  useCompleteWorkoutMutation,
  useCurrentSessionPlanQuery,
  useExerciseSubstitutesMutation,
  useStartWorkoutMutation,
} from '../../features/workout/hooks';
import { loadExerciseBiomechanics } from '../../native/anatomyEngineBridge';
import { openUnity } from '../../native/unityBridge';
import type { MainTabParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { useMockMode } from '../../state/MockModeContext';
import { Button, Card, SyncPendingIndicator } from '../../ui';
import { calculateCompletionRate } from '../../utils/metrics';
import {
  createRestTimerState,
  formatRestTimer,
  pauseRestTimerState,
  resumeRestTimerState,
  stopRestTimerState,
  tickRestTimerState,
  type RestTimerState,
} from '../../utils/restTimer';

type Workout = components['schemas']['Workout'];
type WorkoutSet = components['schemas']['WorkoutSet'];
type ExerciseSubstitute = components['schemas']['ExerciseSubstitute'];

type SetState = {
  reps: string;
  weightKg: string;
  completed: boolean;
  idempotencyKey?: string;
};

type PlannedExercise = {
  workoutExerciseId: string;
  exerciseId: string;
  orderIndex: number;
  name: string;
  targetSets: number;
  targetReps: string;
  restSeconds: number;
  recommendedLoadKg?: number;
  progressionWhy?: string;
  previousPerformanceSets: PreviousPerformanceSet[];
};

function setRowKey(workoutExerciseId: string, setIndex: number): string {
  return `${workoutExerciseId}-${setIndex}`;
}

function createSetIdempotencyKey(
  workoutId: string,
  workoutExerciseId: string,
  setIndex: number,
): string {
  return `set-${workoutId}-${workoutExerciseId}-${setIndex}`;
}

function mergeWorkoutSet(workout: Workout, set: WorkoutSet): Workout {
  return {
    ...workout,
    exercises: workout.exercises.map(exercise => {
      if (exercise.id !== set.workoutExerciseId) {
        return exercise;
      }

      const existingIndex = exercise.sets.findIndex(item => item.setIndex === set.setIndex);
      if (existingIndex >= 0) {
        const nextSets = [...exercise.sets];
        nextSets[existingIndex] = set;
        return {
          ...exercise,
          sets: nextSets,
        };
      }

      return {
        ...exercise,
        sets: [...exercise.sets, set].sort((left, right) => left.setIndex - right.setIndex),
      };
    }),
  };
}

function formatWeightForInput(value: number): string {
  if (!Number.isFinite(value)) {
    return '';
  }

  if (Number.isInteger(value)) {
    return String(value);
  }

  return value.toFixed(1);
}

function resolveWorkoutMode(isMockMode: boolean): boolean {
  return __DEV__ && isMockMode;
}

function findPreviousSet(
  previousSets: PreviousPerformanceSet[],
  setIndex: number,
): PreviousPerformanceSet | null {
  if (previousSets.length === 0) {
    return null;
  }

  const exact = previousSets.find(set => set.setIndex === setIndex);
  if (exact) {
    return exact;
  }

  return previousSets[previousSets.length - 1] ?? null;
}

function roundWeightIncrement(value: number, increment: number = 0.5): number {
  if (!Number.isFinite(value)) {
    return 0;
  }

  return Math.round(value / increment) * increment;
}

function formatSignedDeltaKg(delta: number): string {
  const normalized = Math.abs(delta);
  const magnitude = Number.isInteger(normalized) ? String(normalized) : normalized.toFixed(1);
  const sign = delta >= 0 ? '+' : '-';

  return `${sign}${magnitude} kg`;
}

type NudgeSuggestion = {
  suggestedWeightKg: number;
  label: string;
};

function formatSwapReason(substitute: ExerciseSubstitute): string {
  const pattern =
    substitute.why.matchedPattern
      .map(value => value.replace(/_/g, ' '))
      .find(value => value.trim().length > 0) ?? 'movement pattern';
  const muscles = substitute.why.matchedMuscles
    .slice(0, 2)
    .map(value => value.replace(/_/g, ' '))
    .join('/');

  let label = `Matches ${pattern}`;
  if (muscles) {
    label += ` + ${muscles}`;
  }

  if (substitute.why.equipmentFit === 'exact') {
    label += ' (full equipment fit)';
  } else if (substitute.why.equipmentFit === 'partial') {
    label += ' (partial equipment fit)';
  }

  return label;
}

function resolveNudgeSuggestion(exercise: PlannedExercise, setIndex: number): NudgeSuggestion | null {
  const previousSet = findPreviousSet(exercise.previousPerformanceSets, setIndex);
  if (!previousSet) {
    return null;
  }

  if (
    exercise.recommendedLoadKg !== undefined &&
    Number.isFinite(exercise.recommendedLoadKg) &&
    exercise.recommendedLoadKg >= 0
  ) {
    const suggestedWeightKg = roundWeightIncrement(exercise.recommendedLoadKg);
    const deltaKg = roundWeightIncrement(suggestedWeightKg - previousSet.weightKg);
    if (Math.abs(deltaKg) < 0.1) {
      return null;
    }

    return {
      suggestedWeightKg,
      label: `Nudge ${formatSignedDeltaKg(deltaKg)}`,
    };
  }

  const deltaKg = 2.5;
  const suggestedWeightKg = roundWeightIncrement(previousSet.weightKg + deltaKg);
  return {
    suggestedWeightKg,
    label: `Nudge ${formatSignedDeltaKg(deltaKg)}`,
  };
}

function initializeSetStateFromWorkout(workout: Workout): Record<string, SetState> {
  const next: Record<string, SetState> = {};

  for (const exercise of workout.exercises) {
    const prescription = readPlannedPrescription(exercise);
    const recommendation = readProgressionRecommendation(exercise);
    const previousPerformance = readPreviousPerformance(exercise);
    const loggedSetByIndex = new Map<number, WorkoutSet>(
      exercise.sets.map(set => [set.setIndex, set]),
    );
    const previousSetByIndex = new Map<number, PreviousPerformanceSet>(
      (previousPerformance?.sets ?? []).map(set => [set.setIndex, set]),
    );

    for (let setIndex = 1; setIndex <= prescription.sets; setIndex += 1) {
      const key = setRowKey(exercise.id, setIndex);
      const loggedSet = loggedSetByIndex.get(setIndex);
      const previousSet =
        previousSetByIndex.get(setIndex) ??
        (previousPerformance?.sets.length
          ? previousPerformance.sets[previousPerformance.sets.length - 1]
          : undefined);
      const defaultReps = previousSet ? String(previousSet.reps) : '';
      const defaultWeight = previousSet
        ? formatWeightForInput(previousSet.weightKg)
        : recommendation.recommendedLoadKg !== undefined
          ? formatWeightForInput(recommendation.recommendedLoadKg)
          : '';

      next[key] = {
        reps: loggedSet ? String(loggedSet.reps) : defaultReps,
        weightKg: loggedSet ? formatWeightForInput(loggedSet.weightKg) : defaultWeight,
        completed: Boolean(loggedSet),
      };
    }
  }

  return next;
}

export function WorkoutRunnerScreen(): React.JSX.Element {
  const navigation = useNavigation<BottomTabNavigationProp<MainTabParamList>>();
  const { session } = useAuth();
  const { isMockMode } = useMockMode();
  const useMockWorkouts = resolveWorkoutMode(isMockMode);
  const planQuery = useCurrentSessionPlanQuery();
  const startWorkoutMutation = useStartWorkoutMutation();
  const addSetMutation = useAddWorkoutSetMutation();
  const completeWorkoutMutation = useCompleteWorkoutMutation();
  const substitutesMutation = useExerciseSubstitutesMutation();
  const [activeWorkout, setActiveWorkout] = useState<Workout | null>(null);
  const [setState, setSetState] = useState<Record<string, SetState>>({});
  const [pendingSetRows, setPendingSetRows] = useState<Record<string, boolean>>({});
  const [swapOpenExerciseId, setSwapOpenExerciseId] = useState<string | null>(null);
  const [swapLoadingExerciseId, setSwapLoadingExerciseId] = useState<string | null>(null);
  const [swapOptionsByExerciseId, setSwapOptionsByExerciseId] = useState<
    Record<string, ExerciseSubstitute[]>
  >({});
  const [swapErrorByExerciseId, setSwapErrorByExerciseId] = useState<Record<string, string>>({});
  const [expandedWhyByExerciseId, setExpandedWhyByExerciseId] = useState<Record<string, boolean>>(
    {},
  );
  const [restTimer, setRestTimer] = useState<RestTimerState>(() => stopRestTimerState());
  const [submissionError, setSubmissionError] = useState<string | null>(null);
  const [previewLoadingByExerciseId, setPreviewLoadingByExerciseId] = useState<
    Record<string, boolean>
  >({});
  const hasBiomechanicsEntitlement = hasBiomechanicsOverlayEntitlement(session?.user);
  const [autoStartAttempted, setAutoStartAttempted] = useState(false);
  const inFlightRowsRef = useRef(new Set<string>());
  const restIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const appStateRef = useRef(AppState.currentState);
  const pausedByBackgroundRef = useRef(false);

  const clearRestInterval = useCallback(() => {
    if (restIntervalRef.current) {
      clearInterval(restIntervalRef.current);
      restIntervalRef.current = null;
    }
  }, []);

  const stopTimer = useCallback(() => {
    pausedByBackgroundRef.current = false;
    setRestTimer(stopRestTimerState());
  }, []);

  const pauseRestTimer = useCallback(() => {
    pausedByBackgroundRef.current = false;
    setRestTimer(previous => pauseRestTimerState(previous));
  }, []);

  const resumeRestTimer = useCallback(() => {
    pausedByBackgroundRef.current = false;
    setRestTimer(previous => resumeRestTimerState(previous));
  }, []);

  const startRestTimer = useCallback(
    (seconds: number) => {
      pausedByBackgroundRef.current = false;
      setRestTimer(createRestTimerState(seconds));
    },
    [],
  );

  useEffect(() => {
    if (!restTimer.isRunning) {
      clearRestInterval();
      return;
    }

    if (restIntervalRef.current) {
      return;
    }

    restIntervalRef.current = setInterval(() => {
      setRestTimer(previous => tickRestTimerState(previous));
    }, 250);

    return () => {
      clearRestInterval();
    };
  }, [clearRestInterval, restTimer.isRunning]);

  useEffect(() => {
    const subscription = AppState.addEventListener('change', nextState => {
      const wasActive = appStateRef.current === 'active';
      appStateRef.current = nextState;

      if (wasActive && nextState !== 'active') {
        setRestTimer(previous => {
          if (!previous.isRunning) {
            return previous;
          }

          pausedByBackgroundRef.current = true;
          return pauseRestTimerState(previous);
        });
        return;
      }

      if (nextState === 'active' && pausedByBackgroundRef.current) {
        pausedByBackgroundRef.current = false;
        setRestTimer(previous => resumeRestTimerState(previous));
      }
    });

    return () => {
      subscription.remove();
    };
  }, []);

  useEffect(
    () => () => {
      pausedByBackgroundRef.current = false;
      clearRestInterval();
    },
    [clearRestInterval],
  );

  const startWorkoutForPlan = useCallback(
    async (plan: WorkoutSessionPlan) => {
      setSubmissionError(null);
      const workoutResponse = await startWorkoutMutation.mutateAsync({
        programSessionId: plan.session.id,
      });
      const workout = injectProgressionRecommendations(workoutResponse, plan);

      setActiveWorkout(workout);
      setSetState(initializeSetStateFromWorkout(workout));
      setSwapOpenExerciseId(null);
      setSwapLoadingExerciseId(null);
      setSwapOptionsByExerciseId({});
      setSwapErrorByExerciseId({});
      setExpandedWhyByExerciseId({});
      stopTimer();
      return workout;
    },
    [startWorkoutMutation, stopTimer],
  );

  useEffect(() => {
    if (!planQuery.data || activeWorkout || autoStartAttempted) {
      return;
    }

    setAutoStartAttempted(true);
    startWorkoutForPlan(planQuery.data).catch(() => {
      setSubmissionError('Unable to start workout session.');
      setAutoStartAttempted(false);
    });
  }, [planQuery.data, activeWorkout, autoStartAttempted, startWorkoutForPlan]);

  const plannedExercises = useMemo<PlannedExercise[]>(() => {
    if (!activeWorkout) {
      return [];
    }

    return activeWorkout.exercises.map(exercise => {
      const prescription = readPlannedPrescription(exercise);
      const recommendation = readProgressionRecommendation(exercise);
      const previousPerformance = readPreviousPerformance(exercise);

      return {
        workoutExerciseId: exercise.id,
        exerciseId: exercise.exerciseId,
        orderIndex: exercise.orderIndex,
        name: exercise.exerciseName,
        targetSets: prescription.sets,
        targetReps: prescription.repsRange,
        restSeconds: prescription.restSeconds,
        recommendedLoadKg: recommendation.recommendedLoadKg,
        progressionWhy: recommendation.progressionWhy,
        previousPerformanceSets: previousPerformance?.sets ?? [],
      };
    });
  }, [activeWorkout]);

  const completion = useMemo(() => {
    const totalSets = plannedExercises.reduce((count, exercise) => count + exercise.targetSets, 0);
    const completedSets = Object.values(setState).filter(value => value.completed).length;

    return calculateCompletionRate(completedSets, totalSets);
  }, [plannedExercises, setState]);

  const updateSetField = (
    workoutExerciseId: string,
    setIndex: number,
    nextField: Partial<SetState>,
  ) => {
    const key = setRowKey(workoutExerciseId, setIndex);

    setSetState(previous => ({
      ...previous,
      [key]: {
        reps: previous[key]?.reps ?? '',
        weightKg: previous[key]?.weightKg ?? '',
        completed: previous[key]?.completed ?? false,
        idempotencyKey: previous[key]?.idempotencyKey,
        ...nextField,
      },
    }));
  };

  const logSet = async (exercise: PlannedExercise, setIndex: number) => {
    if (!activeWorkout) {
      return;
    }

    const key = setRowKey(exercise.workoutExerciseId, setIndex);
    const current = setState[key] ?? { reps: '', weightKg: '', completed: false };

    if (current.completed || inFlightRowsRef.current.has(key)) {
      return;
    }

    const reps = Number.parseInt(current.reps.trim() || '0', 10);
    const weightKg = Number.parseFloat(current.weightKg.trim() || '0');
    if (!Number.isFinite(reps) || reps <= 0 || !Number.isFinite(weightKg) || weightKg < 0) {
      setSubmissionError('Enter valid reps and weight before logging a set.');
      return;
    }

    const idempotencyKey =
      current.idempotencyKey ??
      createSetIdempotencyKey(activeWorkout.id, exercise.workoutExerciseId, setIndex);

    updateSetField(exercise.workoutExerciseId, setIndex, {
      idempotencyKey,
    });

    inFlightRowsRef.current.add(key);
    setPendingSetRows(previous => ({
      ...previous,
      [key]: true,
    }));
    setSubmissionError(null);

    try {
      const loggedSet = await addSetMutation.mutateAsync({
        workoutId: activeWorkout.id,
        workoutExerciseId: exercise.workoutExerciseId,
        setIndex,
        reps,
        weightKg,
        idempotencyKey,
      });

      setActiveWorkout(previous => (previous ? mergeWorkoutSet(previous, loggedSet) : previous));
      updateSetField(exercise.workoutExerciseId, setIndex, {
        completed: true,
        reps: String(loggedSet.reps),
        weightKg: formatWeightForInput(loggedSet.weightKg),
      });
      startRestTimer(exercise.restSeconds);
    } catch {
      setSubmissionError('Unable to log set. Please retry.');
    } finally {
      inFlightRowsRef.current.delete(key);
      setPendingSetRows(previous => ({
        ...previous,
        [key]: false,
      }));
    }
  };

  const openSwapOptions = async (exercise: PlannedExercise) => {
    if (swapOpenExerciseId === exercise.workoutExerciseId) {
      setSwapOpenExerciseId(null);
      return;
    }

    const existingOptions = swapOptionsByExerciseId[exercise.workoutExerciseId];
    if (existingOptions) {
      setSwapOpenExerciseId(exercise.workoutExerciseId);
      return;
    }

    setSwapLoadingExerciseId(exercise.workoutExerciseId);
    setSwapErrorByExerciseId(previous => ({
      ...previous,
      [exercise.workoutExerciseId]: '',
    }));

    try {
      const substitutes = await substitutesMutation.mutateAsync({
        exerciseId: exercise.exerciseId,
        limit: 5,
      });

      setSwapOptionsByExerciseId(previous => ({
        ...previous,
        [exercise.workoutExerciseId]: substitutes,
      }));
      setSwapOpenExerciseId(exercise.workoutExerciseId);
    } catch {
      setSwapErrorByExerciseId(previous => ({
        ...previous,
        [exercise.workoutExerciseId]: 'Unable to load substitute options.',
      }));
    } finally {
      setSwapLoadingExerciseId(previous =>
        previous === exercise.workoutExerciseId ? null : previous,
      );
    }
  };

  const applySwap = (exercise: PlannedExercise, selected: ExerciseSubstitute) => {
    setActiveWorkout(previous => {
      if (!previous) {
        return previous;
      }

      return {
        ...previous,
        exercises: previous.exercises.map(item => {
          if (item.id !== exercise.workoutExerciseId) {
            return item;
          }

          return {
            ...item,
            exerciseId: selected.exercise.id,
            exerciseSlug: selected.exercise.slug,
            exerciseName: selected.exercise.name,
            plannedJson: {
              ...item.plannedJson,
              recommended_load_kg: null,
              progression_why: null,
              adjustment_reasons: null,
            },
          };
        }),
      };
    });
    setSwapOpenExerciseId(null);
    setExpandedWhyByExerciseId(previous => ({
      ...previous,
      [exercise.workoutExerciseId]: false,
    }));
  };

  const openAnatomyPreview = useCallback(
    async (exercise: PlannedExercise) => {
      if (!hasBiomechanicsEntitlement) {
        navigation.navigate('Paywall', {
          feature: 'biomechanics_overlays',
        });
        return;
      }

      const accessToken = session?.tokens.accessToken ?? '';
      if (!useMockWorkouts && !accessToken) {
        setSubmissionError('You must be signed in to load anatomy preview.');
        return;
      }

      setSubmissionError(null);
      setPreviewLoadingByExerciseId(previous => ({
        ...previous,
        [exercise.workoutExerciseId]: true,
      }));

      try {
        const biomechanics = await getExerciseBiomechanics(
          {
            accessToken,
            exerciseId: exercise.exerciseId,
          },
          useMockWorkouts,
        );

        await openUnity();
        await loadExerciseBiomechanics(biomechanics);
      } catch (error) {
        const message =
          error instanceof Error && error.message
            ? error.message
            : 'Unable to open anatomy preview right now.';
        setSubmissionError(message);
      } finally {
        setPreviewLoadingByExerciseId(previous => ({
          ...previous,
          [exercise.workoutExerciseId]: false,
        }));
      }
    },
    [hasBiomechanicsEntitlement, navigation, session?.tokens.accessToken, useMockWorkouts],
  );

  const completeWorkout = async () => {
    if (!activeWorkout) {
      return;
    }

    setSubmissionError(null);

    try {
      const completedWorkout = await completeWorkoutMutation.mutateAsync({
        workoutId: activeWorkout.id,
      });
      setActiveWorkout(completedWorkout);
      stopTimer();
      navigation.navigate('Dashboard');
    } catch {
      setSubmissionError('Unable to complete workout. Please retry.');
    }
  };

  if (planQuery.isLoading) {
    return (
      <View style={styles.loading} testID="workout-loading">
        <ActivityIndicator size="large" color="#0f766e" />
      </View>
    );
  }

  if (planQuery.isError) {
    return (
      <View style={styles.loading} testID="workout-plan-error">
        <Text style={styles.error}>Unable to load today&apos;s session plan.</Text>
      </View>
    );
  }

  if (!planQuery.data) {
    return (
      <View style={styles.loading} testID="workout-no-plan">
        <Text style={styles.subtitle}>No scheduled session found for this week.</Text>
      </View>
    );
  }

  const sessionPlan = planQuery.data;
  const showStartingState = !activeWorkout;

  return (
    <ScrollView contentContainerStyle={styles.container} testID="workout-runner-screen">
      <Text style={styles.title}>Workout Runner</Text>
      <Text style={styles.subtitle}>
        {sessionPlan.program.name} • Week {sessionPlan.week.weekIndex} • {sessionPlan.dayLabel}
      </Text>
      <SyncPendingIndicator />

      {showStartingState ? (
        <Card testID="workout-start-card">
          <Text style={styles.sectionTitle}>Today&apos;s Session</Text>
          <Text style={styles.exerciseName}>{sessionPlan.session.name}</Text>
          <Text style={styles.exerciseMeta}>
            {sessionPlan.session.exercises.length} exercise
            {sessionPlan.session.exercises.length === 1 ? '' : 's'}
          </Text>
          <Button
            label="Start Workout"
            onPress={() => {
              startWorkoutForPlan(sessionPlan).catch(() => {
                setSubmissionError('Unable to start workout session.');
              });
            }}
            loading={startWorkoutMutation.isPending}
            disabled={startWorkoutMutation.isPending}
            testID="start-workout-button"
          />
        </Card>
      ) : null}

      {activeWorkout ? (
        <>
          <Card>
            <Text style={styles.sectionTitle}>Rest Timer</Text>
            <Text style={styles.timerValue}>{formatRestTimer(restTimer.remainingSeconds)}</Text>
            {restTimer.remainingSeconds > 0 ? (
              <Text style={styles.timerStatus}>{restTimer.isRunning ? 'Running' : 'Paused'}</Text>
            ) : null}
            <View style={styles.timerActions}>
              <Button label="Start 60s" variant="secondary" onPress={() => startRestTimer(60)} />
              <Button
                label={restTimer.isRunning ? 'Pause' : 'Resume'}
                variant="secondary"
                disabled={restTimer.remainingSeconds === 0}
                onPress={() => {
                  if (restTimer.isRunning) {
                    pauseRestTimer();
                  } else {
                    resumeRestTimer();
                  }
                }}
              />
              <Button
                label="Reset"
                variant="secondary"
                onPress={() => {
                  stopTimer();
                }}
              />
            </View>
          </Card>

          <Card>
            <Text style={styles.sectionTitle}>Session Completion</Text>
            <Text style={styles.progressCopy}>{completion}% of sets logged</Text>
          </Card>

          {plannedExercises.map(exercise => (
            <Card key={exercise.workoutExerciseId}>
              <Text style={styles.exerciseName} testID={`exercise-name-${exercise.workoutExerciseId}`}>
                {exercise.name}
              </Text>
              <Text
                style={styles.exerciseMeta}
                testID={`exercise-meta-${exercise.workoutExerciseId}`}>
                {exercise.targetSets} sets • {exercise.targetReps} reps • Rest {exercise.restSeconds}s
              </Text>
              {exercise.orderIndex === 1 && exercise.recommendedLoadKg !== undefined ? (
                <View style={styles.recommendationBox}>
                  <Text
                    style={styles.recommendationText}
                    testID={`recommended-load-${exercise.workoutExerciseId}`}>
                    Recommended load: {exercise.recommendedLoadKg.toFixed(1)} kg
                  </Text>
                  {exercise.progressionWhy ? (
                    <>
                      <Button
                        label={expandedWhyByExerciseId[exercise.workoutExerciseId] ? 'Hide Why' : 'Why'}
                        variant="secondary"
                        onPress={() => {
                          setExpandedWhyByExerciseId(previous => ({
                            ...previous,
                            [exercise.workoutExerciseId]: !previous[exercise.workoutExerciseId],
                          }));
                        }}
                        testID={`recommendation-why-toggle-${exercise.workoutExerciseId}`}
                      />
                      {expandedWhyByExerciseId[exercise.workoutExerciseId] ? (
                        <Text
                          style={styles.recommendationWhy}
                          testID={`recommendation-why-${exercise.workoutExerciseId}`}>
                          {exercise.progressionWhy}
                        </Text>
                      ) : null}
                    </>
                  ) : null}
                </View>
              ) : null}
              <View style={styles.swapActions}>
                <Button
                  label={swapOpenExerciseId === exercise.workoutExerciseId ? 'Hide Swaps' : 'Swap'}
                  variant="secondary"
                  disabled={Boolean(activeWorkout.completedAt) || completeWorkoutMutation.isPending}
                  loading={swapLoadingExerciseId === exercise.workoutExerciseId}
                  onPress={() => {
                    openSwapOptions(exercise).catch(() => {
                      setSwapErrorByExerciseId(previous => ({
                        ...previous,
                        [exercise.workoutExerciseId]: 'Unable to load substitute options.',
                      }));
                    });
                  }}
                  testID={`swap-exercise-${exercise.workoutExerciseId}`}
                />
                <Button
                  label="Anatomy Preview"
                  variant="secondary"
                  disabled={Boolean(activeWorkout.completedAt) || completeWorkoutMutation.isPending}
                  loading={Boolean(previewLoadingByExerciseId[exercise.workoutExerciseId])}
                  onPress={() => {
                    openAnatomyPreview(exercise).catch(() => {
                      setSubmissionError('Unable to open anatomy preview right now.');
                    });
                  }}
                  testID={`anatomy-preview-${exercise.workoutExerciseId}`}
                />
              </View>
              {swapErrorByExerciseId[exercise.workoutExerciseId] ? (
                <Text style={styles.error}>{swapErrorByExerciseId[exercise.workoutExerciseId]}</Text>
              ) : null}
              {swapOpenExerciseId === exercise.workoutExerciseId ? (
                <View style={styles.swapOptions} testID={`swap-options-${exercise.workoutExerciseId}`}>
                  {(swapOptionsByExerciseId[exercise.workoutExerciseId] ?? []).length === 0 ? (
                    <Text style={styles.exerciseMeta}>
                      No substitutes match current constraints.
                    </Text>
                  ) : (
                    (swapOptionsByExerciseId[exercise.workoutExerciseId] ?? []).map(
                      (candidate, candidateIndex) => (
                        <View key={`${exercise.workoutExerciseId}-${candidate.exercise.id}`}>
                          <Button
                            label={candidate.exercise.name}
                            variant="secondary"
                            onPress={() => applySwap(exercise, candidate)}
                            testID={`swap-option-${exercise.workoutExerciseId}-${candidateIndex}`}
                          />
                          <Text
                            style={styles.swapReason}
                            testID={`swap-option-why-${exercise.workoutExerciseId}-${candidateIndex}`}>
                            {formatSwapReason(candidate)}
                          </Text>
                        </View>
                      ),
                    )
                  )}
                </View>
              ) : null}

              {Array.from({ length: exercise.targetSets }, (_, index) => {
                const setIndex = index + 1;
                const key = setRowKey(exercise.workoutExerciseId, setIndex);
                const currentSet = setState[key] ?? {
                  reps: '',
                  weightKg: '',
                  completed: false,
                };
                const isPending = Boolean(pendingSetRows[key]);
                const isLocked =
                  currentSet.completed ||
                  isPending ||
                  Boolean(activeWorkout.completedAt) ||
                  completeWorkoutMutation.isPending;
                const nudgeSuggestion = resolveNudgeSuggestion(exercise, setIndex);

                return (
                  <View key={key} style={styles.setBlock}>
                    <View style={styles.setRow}>
                      <Text style={styles.setLabel}>Set {setIndex}</Text>
                      <TextInput
                        value={currentSet.reps}
                        onChangeText={text =>
                          updateSetField(exercise.workoutExerciseId, setIndex, { reps: text })
                        }
                        placeholder="Reps"
                        keyboardType="number-pad"
                        style={styles.setInput}
                        editable={!isLocked}
                        testID={`set-reps-${exercise.workoutExerciseId}-${setIndex}`}
                      />
                      <TextInput
                        value={currentSet.weightKg}
                        onChangeText={text =>
                          updateSetField(exercise.workoutExerciseId, setIndex, { weightKg: text })
                        }
                        placeholder="kg"
                        keyboardType="decimal-pad"
                        style={styles.setInput}
                        editable={!isLocked}
                        testID={`set-weight-${exercise.workoutExerciseId}-${setIndex}`}
                      />
                      <Button
                        label={currentSet.completed ? 'Done' : 'Log'}
                        variant={currentSet.completed ? 'secondary' : 'primary'}
                        loading={isPending}
                        disabled={isLocked}
                        onPress={() => {
                          logSet(exercise, setIndex).catch(() => {
                            setSubmissionError('Unable to log set. Please retry.');
                          });
                        }}
                        style={styles.logButton}
                        testID={`log-set-${exercise.workoutExerciseId}-${setIndex}`}
                      />
                    </View>
                    {nudgeSuggestion && !currentSet.completed ? (
                      <View style={styles.nudgeRow}>
                        <Text
                          style={styles.nudgeText}
                          testID={`set-nudge-copy-${exercise.workoutExerciseId}-${setIndex}`}>
                          {nudgeSuggestion.label}
                        </Text>
                        <Button
                          label={`Use ${formatWeightForInput(nudgeSuggestion.suggestedWeightKg)} kg`}
                          variant="secondary"
                          disabled={isLocked}
                          onPress={() => {
                            updateSetField(exercise.workoutExerciseId, setIndex, {
                              weightKg: formatWeightForInput(nudgeSuggestion.suggestedWeightKg),
                            });
                          }}
                          style={styles.nudgeButton}
                          testID={`set-nudge-${exercise.workoutExerciseId}-${setIndex}`}
                        />
                      </View>
                    ) : null}
                  </View>
                );
              })}
            </Card>
          ))}

          <Button
            label="Complete Workout"
            variant="primary"
            onPress={() => {
              completeWorkout().catch(() => {
                setSubmissionError('Unable to complete workout. Please retry.');
              });
            }}
            loading={completeWorkoutMutation.isPending}
            disabled={completeWorkoutMutation.isPending || Boolean(activeWorkout.completedAt)}
            testID="complete-workout-button"
          />
        </>
      ) : null}

      {submissionError ? <Text style={styles.error}>{submissionError}</Text> : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: 16,
    gap: 12,
    backgroundColor: '#f8fafc',
  },
  loading: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#f8fafc',
  },
  title: {
    fontSize: 24,
    fontWeight: '700',
    color: '#0f172a',
  },
  subtitle: {
    color: '#475569',
  },
  sectionTitle: {
    fontSize: 17,
    fontWeight: '700',
    color: '#0f172a',
  },
  timerValue: {
    fontSize: 34,
    fontWeight: '700',
    color: '#0f766e',
  },
  timerStatus: {
    color: '#0f172a',
    fontSize: 13,
    fontWeight: '600',
  },
  timerActions: {
    flexDirection: 'row',
    gap: 8,
    flexWrap: 'wrap',
  },
  progressCopy: {
    color: '#334155',
    fontSize: 16,
  },
  exerciseName: {
    fontSize: 18,
    fontWeight: '700',
    color: '#0f172a',
  },
  exerciseMeta: {
    color: '#64748b',
  },
  recommendationBox: {
    marginTop: 6,
    gap: 6,
    padding: 8,
    borderRadius: 8,
    backgroundColor: '#ecfdf5',
  },
  recommendationText: {
    color: '#065f46',
    fontWeight: '600',
  },
  recommendationWhy: {
    color: '#334155',
    fontSize: 13,
    lineHeight: 18,
  },
  swapActions: {
    marginTop: 4,
    alignSelf: 'flex-start',
    gap: 8,
  },
  swapOptions: {
    gap: 8,
  },
  swapReason: {
    marginTop: 4,
    color: '#475569',
    fontSize: 13,
    lineHeight: 18,
  },
  setRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  setBlock: {
    gap: 6,
  },
  setLabel: {
    minWidth: 44,
    color: '#0f172a',
    fontWeight: '600',
  },
  setInput: {
    flex: 1,
    borderWidth: 1,
    borderColor: '#cbd5e1',
    borderRadius: 8,
    paddingHorizontal: 10,
    paddingVertical: 8,
    color: '#0f172a',
    backgroundColor: '#ffffff',
  },
  logButton: {
    minWidth: 74,
  },
  nudgeRow: {
    marginLeft: 52,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 8,
  },
  nudgeText: {
    color: '#1d4ed8',
    fontSize: 13,
    fontWeight: '600',
  },
  nudgeButton: {
    minHeight: 32,
    paddingHorizontal: 10,
  },
  error: {
    color: '#b91c1c',
    fontSize: 14,
  },
});
