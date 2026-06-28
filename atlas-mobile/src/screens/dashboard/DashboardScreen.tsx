import React, { useState } from 'react';
import {
  ActivityIndicator,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { useNavigation } from '@react-navigation/native';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import {
  useDashboardQuery,
  useMomentumSprintChecklistMutation,
  useNutritionCheckInMutation,
  useReadinessCheckInMutation,
  useToggleHabitMutation,
} from '../../features/dashboard/hooks';
import {
  calculatePercentChange,
  formatDateLabel,
  formatMacro,
  formatMuscleGroupLabel,
  formatSignedPercent,
  formatSignedWeightDeltaKg,
  formatVolumeKg,
  formatWeightKg,
} from '../../features/dashboard/formatters';
import type { MainTabParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { useOnboarding } from '../../state/OnboardingContext';
import { Button, Card, SyncPendingIndicator } from '../../ui';

type MainLiftKey = 'squat' | 'bench' | 'deadlift';

const MAIN_LIFT_LABELS: Record<MainLiftKey, string> = {
  squat: 'Squat',
  bench: 'Bench',
  deadlift: 'Deadlift',
};

const READINESS_OPTIONS: Array<{ value: number; label: string }> = [
  { value: 1, label: 'Low' },
  { value: 2, label: 'Moderate' },
  { value: 3, label: 'High' },
];

type TrendBarDatum = {
  key: string;
  label: string;
  value: number;
  valueLabel: string;
};

function HorizontalTrendBars({
  rows,
  color,
  testID,
}: {
  rows: TrendBarDatum[];
  color: string;
  testID?: string;
}): React.JSX.Element {
  const maxValue = rows.reduce((max, row) => Math.max(max, row.value), 0);

  return (
    <View style={styles.chartRows} testID={testID}>
      {rows.map(row => (
        <View key={row.key} style={styles.chartRow}>
          <Text style={styles.chartLabel}>{row.label}</Text>
          <View style={styles.chartTrack}>
            <View
              style={[
                styles.chartFill,
                {
                  width: `${
                    maxValue <= 0 ? 0 : Math.round((row.value / maxValue) * 100)
                  }%`,
                  backgroundColor: color,
                },
              ]}
            />
          </View>
          <Text style={styles.chartValue}>{row.valueLabel}</Text>
        </View>
      ))}
    </View>
  );
}

export function DashboardScreen(): React.JSX.Element {
  const navigation = useNavigation<BottomTabNavigationProp<MainTabParamList>>();
  const dashboardQuery = useDashboardQuery();
  const toggleHabitMutation = useToggleHabitMutation();
  const momentumSprintMutation = useMomentumSprintChecklistMutation();
  const nutritionMutation = useNutritionCheckInMutation();
  const readinessMutation = useReadinessCheckInMutation();
  const { session, logout } = useAuth();
  const { firstWeekPlan, planExplanation } = useOnboarding();
  const [readinessSkipped, setReadinessSkipped] = useState(false);
  const [readinessAnswers, setReadinessAnswers] = useState({
    energyLevel: null as number | null,
    sleepQuality: null as number | null,
    stressLevel: null as number | null,
  });

  if (dashboardQuery.isLoading) {
    return (
      <View style={styles.loading} testID="dashboard-loading">
        <ActivityIndicator size="large" color="#0f766e" />
      </View>
    );
  }

  if (dashboardQuery.isError || !dashboardQuery.data) {
    return (
      <View style={styles.loading}>
        <Text style={styles.error}>Unable to load dashboard.</Text>
      </View>
    );
  }

  const { summary, habits, nutritionCheckedIn, momentumSprint, dateKey } =
    dashboardQuery.data;
  const estimatedOneRMByLift = summary.estimatedOneRmByLift;
  const oneRMRows: Array<{
    lift: MainLiftKey;
    estimate: number | null | undefined;
  }> = [
    { lift: 'squat', estimate: estimatedOneRMByLift.squat.estimatedOneRmKg },
    { lift: 'bench', estimate: estimatedOneRMByLift.bench.estimatedOneRmKg },
    {
      lift: 'deadlift',
      estimate: estimatedOneRMByLift.deadlift.estimatedOneRmKg,
    },
  ];
  const prHighlights = summary.prEvents.slice(0, 3);

  const currentWeekVolumeKg = summary.weeklyVolumeTrend[0]?.totalVolumeKg ?? 0;
  const previousWeekVolumeKg = summary.weeklyVolumeTrend[1]?.totalVolumeKg ?? 0;
  const weekOverWeekPercent = calculatePercentChange(
    currentWeekVolumeKg,
    previousWeekVolumeKg,
  );
  const trailingFourWeeks = summary.weeklyVolumeTrend.slice(0, 4);
  const trailingFourWeekAverageKg =
    trailingFourWeeks.length === 0
      ? 0
      : trailingFourWeeks.reduce(
          (total, point) => total + point.totalVolumeKg,
          0,
        ) / trailingFourWeeks.length;

  const latestMuscleWeek = summary.weeklyMuscleGroupVolume[0];
  const muscleVolumeRows = Object.entries(
    latestMuscleWeek?.volumeByMuscleGroup ?? {},
  ).sort((left, right) => right[1] - left[1]);
  const totalLatestMuscleVolume = muscleVolumeRows.reduce(
    (total, [, volume]) => total + volume,
    0,
  );

  const trainingStreak = summary.adherenceStreaks.training;
  const proteinStreak = summary.adherenceStreaks.protein;

  const volumeTrendRows: TrendBarDatum[] = [...summary.weeklyVolumeTrend]
    .slice(0, 6)
    .reverse()
    .map(point => ({
      key: point.weekStartDate,
      label: point.weekStartDate.slice(5),
      value: point.totalVolumeKg,
      valueLabel: formatVolumeKg(point.totalVolumeKg),
    }));

  const weightTrendRows: TrendBarDatum[] = [...summary.weightTrendPoints]
    .slice(-8)
    .map(point => ({
      key: point.date,
      label: point.date.slice(5),
      value: point.weightKg,
      valueLabel: formatWeightKg(point.weightKg),
    }));
  const latestWeight =
    summary.weightTrendPoints[summary.weightTrendPoints.length - 1];
  const baselineWeight = summary.weightTrendPoints[0];
  const weightDeltaKg =
    latestWeight && baselineWeight
      ? latestWeight.weightKg - baselineWeight.weightKg
      : null;

  const readinessHistoryRows: TrendBarDatum[] = [
    ...summary.readinessSelfReportHistory,
  ]
    .slice(-7)
    .map(point => ({
      key: point.date,
      label: point.date.slice(5),
      value: point.readinessScore,
      valueLabel: point.readinessScore.toFixed(2),
    }));
  const latestReadiness =
    summary.readinessSelfReportHistory[
      summary.readinessSelfReportHistory.length - 1
    ];

  const canSubmitReadiness =
    readinessAnswers.energyLevel !== null &&
    readinessAnswers.sleepQuality !== null &&
    readinessAnswers.stressLevel !== null;

  const setReadinessValue = (
    field: 'energyLevel' | 'sleepQuality' | 'stressLevel',
    value: number,
  ) => {
    setReadinessAnswers(current => ({
      ...current,
      [field]: value,
    }));
  };

  const submitReadinessCheckin = () => {
    if (!canSubmitReadiness) {
      return;
    }

    readinessMutation.mutate(
      {
        dateKey,
        energyLevel: readinessAnswers.energyLevel as number,
        sleepQuality: readinessAnswers.sleepQuality as number,
        stressLevel: readinessAnswers.stressLevel as number,
      },
      {
        onSuccess: () => {
          setReadinessSkipped(false);
          setReadinessAnswers({
            energyLevel: null,
            sleepQuality: null,
            stressLevel: null,
          });
        },
      },
    );
  };

  return (
    <ScrollView
      contentContainerStyle={styles.container}
      testID="dashboard-screen"
    >
      <Text style={styles.title}>Dashboard</Text>
      <Text style={styles.subtitle}>
        Welcome back, {session?.user.email ?? 'athlete'}.
      </Text>
      <SyncPendingIndicator />
      {firstWeekPlan && firstWeekPlan.days.length > 0 ? (
        <Card testID="first-week-plan-summary">
          <Text style={styles.sectionTitle}>First-Week Plan</Text>
          <Text style={styles.helperText}>
            {planExplanation ??
              'Server-generated kickoff sessions based on onboarding.'}
          </Text>
          {firstWeekPlan.days.map(item => (
            <View
              key={`${item.day}-${item.sessionName}`}
              style={styles.planRow}
            >
              <Text style={styles.planDay}>{item.day}</Text>
              <Text style={styles.planSession}>{item.sessionName}</Text>
            </View>
          ))}
        </Card>
      ) : null}
      <Button
        label="Privacy & Consents"
        variant="secondary"
        onPress={() => navigation.navigate('PrivacySettings')}
        testID="open-privacy-settings"
      />
      <Button
        label="Weekly Check-in"
        variant="secondary"
        onPress={() => navigation.navigate('WeeklyCheckIn')}
        testID="open-weekly-checkin"
      />
      <Button
        label="Form Check"
        variant="secondary"
        onPress={() => navigation.navigate('FormCheck')}
        testID="open-form-check"
      />

      <View style={styles.metricsGrid}>
        <Card style={styles.metricCard}>
          <Text style={styles.metricLabel}>Workouts (last 7 days)</Text>
          <Text style={styles.metricValue}>
            {summary.workoutsCompletedLast7Days}
          </Text>
        </Card>
        <Card style={styles.metricCard}>
          <Text style={styles.metricLabel}>Sets logged (last 7 days)</Text>
          <Text style={styles.metricValue}>{summary.totalSetsLast7Days}</Text>
        </Card>
        <Card style={styles.metricCard}>
          <Text style={styles.metricLabel}>
            Protein adherence (last 7 days)
          </Text>
          <Text style={styles.metricValue}>
            {Math.round(summary.proteinAdherenceLast7DaysPercent)}%
          </Text>
        </Card>
      </View>

      <Card testID="dashboard-nutrition-totals">
        <Text style={styles.sectionTitle}>Today&apos;s Nutrition</Text>
        <View style={styles.dataRow}>
          <Text style={styles.dataLabel}>Calories</Text>
          <Text style={styles.dataValue}>
            {formatMacro(summary.nutritionTotalsToday.calories_kcal, 'kcal')}
          </Text>
        </View>
        <View style={styles.dataRow}>
          <Text style={styles.dataLabel}>Protein</Text>
          <Text style={styles.dataValue}>
            {formatMacro(summary.nutritionTotalsToday.protein_g, 'g')}
          </Text>
        </View>
        <View style={styles.dataRow}>
          <Text style={styles.dataLabel}>Carbs</Text>
          <Text style={styles.dataValue}>
            {formatMacro(summary.nutritionTotalsToday.carbs_g, 'g')}
          </Text>
        </View>
        <View style={styles.dataRow}>
          <Text style={styles.dataLabel}>Fat</Text>
          <Text style={styles.dataValue}>
            {formatMacro(summary.nutritionTotalsToday.fat_g, 'g')}
          </Text>
        </View>
      </Card>

      <Card testID="dashboard-estimated-1rm">
        <Text style={styles.sectionTitle}>Estimated 1RM (Epley)</Text>
        {oneRMRows.map(row => (
          <View key={row.lift} style={styles.dataRow}>
            <Text style={styles.dataLabel}>{MAIN_LIFT_LABELS[row.lift]}</Text>
            <Text style={styles.dataValue}>
              {row.estimate == null ? '--' : formatWeightKg(row.estimate)}
            </Text>
          </View>
        ))}
      </Card>

      <Card testID="dashboard-pr-highlights">
        <Text style={styles.sectionTitle}>PR Highlights</Text>
        {prHighlights.length === 0 ? (
          <Text style={styles.helperText}>
            No recent PRs yet. Keep logging sessions.
          </Text>
        ) : (
          prHighlights.map((event, index) => (
            <View
              key={`${event.lift}-${event.completedAt}`}
              style={[styles.prRow, index === 0 ? styles.prRowFirst : null]}
            >
              <View style={styles.prHeader}>
                <Text style={styles.prLift}>
                  {MAIN_LIFT_LABELS[event.lift]}
                </Text>
                <Text style={styles.prValue}>
                  {formatWeightKg(event.estimatedOneRmKg)}
                </Text>
              </View>
              <Text style={styles.prMeta}>
                {formatDateLabel(event.completedAt)} · {event.reps} reps @{' '}
                {formatWeightKg(event.weightKg)}
              </Text>
              <Text style={styles.prMeta}>
                {event.improvementKg == null
                  ? 'First tracked benchmark'
                  : `PR gain +${formatWeightKg(event.improvementKg)}`}
              </Text>
            </View>
          ))
        )}
      </Card>

      <Card testID="dashboard-volume-trends">
        <Text style={styles.sectionTitle}>Volume Trends</Text>
        <View style={styles.dataRow}>
          <Text style={styles.dataLabel}>This week</Text>
          <Text style={styles.dataValue}>
            {formatVolumeKg(currentWeekVolumeKg)}
          </Text>
        </View>
        <View style={styles.dataRow}>
          <Text style={styles.dataLabel}>Vs previous week</Text>
          <Text style={styles.dataValue}>
            {formatSignedPercent(weekOverWeekPercent)}
          </Text>
        </View>
        <View style={styles.dataRow}>
          <Text style={styles.dataLabel}>4-week average</Text>
          <Text style={styles.dataValue}>
            {formatVolumeKg(trailingFourWeekAverageKg)}
          </Text>
        </View>
        {volumeTrendRows.length > 0 ? (
          <HorizontalTrendBars
            rows={volumeTrendRows}
            color="#0f766e"
            testID="volume-trend-bars"
          />
        ) : null}
      </Card>

      <Card testID="dashboard-muscle-volume-distribution">
        <Text style={styles.sectionTitle}>Muscle Group Distribution</Text>
        {muscleVolumeRows.length === 0 ? (
          <Text style={styles.helperText}>
            No muscle volume logged for this week yet.
          </Text>
        ) : (
          muscleVolumeRows.slice(0, 5).map(([muscleGroup, volume]) => (
            <View key={muscleGroup} style={styles.dataRow}>
              <Text style={styles.dataLabel}>
                {formatMuscleGroupLabel(muscleGroup)}
              </Text>
              <Text style={styles.dataValue}>
                {totalLatestMuscleVolume <= 0
                  ? formatVolumeKg(volume)
                  : `${Math.round(
                      (volume / totalLatestMuscleVolume) * 100,
                    )}% · ${formatVolumeKg(volume)}`}
              </Text>
            </View>
          ))
        )}
      </Card>

      <Card testID="dashboard-adherence-streaks">
        <Text style={styles.sectionTitle}>Adherence Streaks</Text>
        <View style={styles.dataRow}>
          <Text style={styles.dataLabel}>Training</Text>
          <Text style={styles.dataValue}>
            {trainingStreak.currentDays} current · {trainingStreak.longestDays}{' '}
            best
          </Text>
        </View>
        <View style={styles.dataRow}>
          <Text style={styles.dataLabel}>Protein targets</Text>
          <Text style={styles.dataValue}>
            {proteinStreak.currentDays} current · {proteinStreak.longestDays}{' '}
            best
          </Text>
        </View>
      </Card>

      <Card testID="dashboard-weight-trend">
        <Text style={styles.sectionTitle}>Bodyweight Trend</Text>
        {weightTrendRows.length === 0 ? (
          <Text style={styles.helperText}>
            No weight entries yet. Log one from Weekly Check-in.
          </Text>
        ) : (
          <>
            <View style={styles.dataRow}>
              <Text style={styles.dataLabel}>Latest</Text>
              <Text style={styles.dataValue}>
                {formatWeightKg(latestWeight?.weightKg ?? 0)}
              </Text>
            </View>
            <View style={styles.dataRow}>
              <Text style={styles.dataLabel}>Trend window delta</Text>
              <Text style={styles.dataValue}>
                {weightDeltaKg == null
                  ? '--'
                  : formatSignedWeightDeltaKg(weightDeltaKg)}
              </Text>
            </View>
            <HorizontalTrendBars
              rows={weightTrendRows}
              color="#0ea5e9"
              testID="weight-trend-bars"
            />
          </>
        )}
      </Card>

      <Card testID="dashboard-readiness-history">
        <Text style={styles.sectionTitle}>Readiness & Stress</Text>
        {latestReadiness ? (
          <Text style={styles.helperText}>
            Latest {formatDateLabel(latestReadiness.date)} · readiness{' '}
            {latestReadiness.readinessScore.toFixed(2)} · stress{' '}
            {latestReadiness.stressLevel}/3
          </Text>
        ) : (
          <Text style={styles.helperText}>No readiness entries yet.</Text>
        )}
        {readinessHistoryRows.length > 0 ? (
          <HorizontalTrendBars
            rows={readinessHistoryRows}
            color="#f97316"
            testID="readiness-trend-bars"
          />
        ) : null}
      </Card>

      {!readinessSkipped ? (
        <Card testID="dashboard-readiness-checkin">
          <Text style={styles.sectionTitle}>
            Today&apos;s Readiness Check-In
          </Text>
          <Text style={styles.helperText}>
            3 quick questions to tune your day. Optional.
          </Text>

          <View style={styles.readinessQuestion}>
            <Text style={styles.dataLabel}>Energy</Text>
            <View style={styles.inlineButtonRow}>
              {READINESS_OPTIONS.map(option => (
                <Button
                  key={`readiness-energy-${option.value}`}
                  label={option.label}
                  variant={
                    readinessAnswers.energyLevel === option.value
                      ? 'primary'
                      : 'secondary'
                  }
                  onPress={() => setReadinessValue('energyLevel', option.value)}
                  testID={`readiness-energy-${option.value}`}
                />
              ))}
            </View>
          </View>

          <View style={styles.readinessQuestion}>
            <Text style={styles.dataLabel}>Sleep quality</Text>
            <View style={styles.inlineButtonRow}>
              {READINESS_OPTIONS.map(option => (
                <Button
                  key={`readiness-sleep-${option.value}`}
                  label={option.label}
                  variant={
                    readinessAnswers.sleepQuality === option.value
                      ? 'primary'
                      : 'secondary'
                  }
                  onPress={() =>
                    setReadinessValue('sleepQuality', option.value)
                  }
                  testID={`readiness-sleep-${option.value}`}
                />
              ))}
            </View>
          </View>

          <View style={styles.readinessQuestion}>
            <Text style={styles.dataLabel}>Stress</Text>
            <View style={styles.inlineButtonRow}>
              {READINESS_OPTIONS.map(option => (
                <Button
                  key={`readiness-stress-${option.value}`}
                  label={option.label}
                  variant={
                    readinessAnswers.stressLevel === option.value
                      ? 'primary'
                      : 'secondary'
                  }
                  onPress={() => setReadinessValue('stressLevel', option.value)}
                  testID={`readiness-stress-${option.value}`}
                />
              ))}
            </View>
          </View>

          <View style={styles.inlineButtonRow}>
            <Button
              label="Skip"
              variant="secondary"
              onPress={() => setReadinessSkipped(true)}
              testID="readiness-skip-button"
            />
            <Button
              label="Save check-in"
              onPress={submitReadinessCheckin}
              disabled={!canSubmitReadiness || readinessMutation.isPending}
              loading={readinessMutation.isPending}
              testID="readiness-submit-button"
            />
          </View>
        </Card>
      ) : (
        <Card testID="dashboard-readiness-skip-state">
          <Text style={styles.helperText}>
            Readiness check-in skipped for now.
          </Text>
          <Button
            label="Answer now"
            variant="secondary"
            onPress={() => setReadinessSkipped(false)}
            testID="readiness-show-button"
          />
        </Card>
      )}

      <Card>
        <Text style={styles.sectionTitle}>Habits</Text>
        {habits.map(habit => (
          <View key={habit.id} style={styles.habitRow}>
            <Text style={styles.habitCopy}>{habit.label}</Text>
            <Button
              label={habit.completed ? 'Done' : 'Mark'}
              variant={habit.completed ? 'secondary' : 'primary'}
              onPress={() => toggleHabitMutation.mutate(habit.id)}
              loading={
                toggleHabitMutation.isPending &&
                toggleHabitMutation.variables === habit.id
              }
              disabled={
                toggleHabitMutation.isPending &&
                toggleHabitMutation.variables === habit.id
              }
              testID={`habit-toggle-${habit.id}`}
            />
          </View>
        ))}
      </Card>

      {momentumSprint.enrolled &&
      momentumSprint.enrollment &&
      momentumSprint.progress ? (
        <Card testID="momentum-sprint-card">
          <Text style={styles.sectionTitle}>Momentum Sprint</Text>
          <Text style={styles.helperText}>
            Day {momentumSprint.progress.currentDay} of{' '}
            {momentumSprint.progress.totalDays} ·{' '}
            {Math.round(momentumSprint.progress.completionPercent)}% complete
          </Text>
          <Text style={styles.helperText}>
            Streak: {momentumSprint.progress.currentStreak} day(s) · Longest:{' '}
            {momentumSprint.progress.longestStreak}
          </Text>
          {momentumSprint.progress.nextMilestoneDay &&
          momentumSprint.progress.nextMilestoneLabel ? (
            <Text style={styles.helperText}>
              Next reward: Day {momentumSprint.progress.nextMilestoneDay} ·{' '}
              {momentumSprint.progress.nextMilestoneLabel}
            </Text>
          ) : null}
          {momentumSprint.todayChecklist.length === 0 ? (
            <Text style={styles.helperText}>
              No checklist entries available for today.
            </Text>
          ) : (
            momentumSprint.todayChecklist.map(entry => (
              <View
                key={`${entry.date}-${entry.habitKey}`}
                style={styles.habitRow}
              >
                <Text style={styles.habitCopy}>
                  {entry.completed ? '[x]' : '[ ]'} {entry.habitLabel}
                </Text>
                <Button
                  label={entry.completed ? 'Checked' : 'Check'}
                  variant={entry.completed ? 'secondary' : 'primary'}
                  onPress={() =>
                    momentumSprintMutation.mutate({
                      habitKey: entry.habitKey,
                      completed: !entry.completed,
                      dateKey,
                    })
                  }
                  loading={
                    momentumSprintMutation.isPending &&
                    momentumSprintMutation.variables?.habitKey ===
                      entry.habitKey
                  }
                  disabled={
                    momentumSprintMutation.isPending &&
                    momentumSprintMutation.variables?.habitKey ===
                      entry.habitKey
                  }
                  testID={`momentum-sprint-toggle-${entry.habitKey}`}
                />
              </View>
            ))
          )}
        </Card>
      ) : null}

      <Card>
        <Text style={styles.sectionTitle}>Nutrition Check-In</Text>
        <Text style={styles.helperText}>
          {nutritionCheckedIn
            ? 'Check-in completed for today.'
            : 'Log your nutrition adherence for today.'}
        </Text>
        <Button
          label={nutritionCheckedIn ? 'Checked In' : 'Open Nutrition Check-In'}
          variant={nutritionCheckedIn ? 'secondary' : 'primary'}
          onPress={() =>
            nutritionMutation.mutate({
              dateKey,
            })
          }
          disabled={nutritionCheckedIn || nutritionMutation.isPending}
          loading={nutritionMutation.isPending}
          testID="nutrition-checkin-button"
        />
      </Card>

      <Button
        label="Logout"
        variant="danger"
        onPress={() => {
          logout().catch(() => {});
        }}
        testID="logout-button"
      />
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
  metricsGrid: {
    gap: 10,
  },
  metricCard: {
    gap: 6,
  },
  metricLabel: {
    fontSize: 14,
    color: '#64748b',
  },
  metricValue: {
    fontSize: 24,
    fontWeight: '700',
    color: '#0f172a',
  },
  dataRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: 8,
  },
  dataLabel: {
    color: '#334155',
    fontSize: 14,
  },
  dataValue: {
    color: '#0f172a',
    fontSize: 14,
    fontWeight: '600',
  },
  chartRows: {
    marginTop: 8,
    gap: 6,
  },
  chartRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  chartLabel: {
    width: 44,
    color: '#475569',
    fontSize: 12,
  },
  chartTrack: {
    flex: 1,
    height: 8,
    borderRadius: 999,
    backgroundColor: '#e2e8f0',
    overflow: 'hidden',
  },
  chartFill: {
    height: '100%',
    borderRadius: 999,
  },
  chartValue: {
    minWidth: 62,
    textAlign: 'right',
    color: '#0f172a',
    fontSize: 12,
    fontWeight: '600',
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: '700',
    color: '#0f172a',
  },
  prRow: {
    borderTopWidth: 1,
    borderTopColor: '#e2e8f0',
    paddingTop: 10,
    gap: 2,
  },
  prRowFirst: {
    borderTopWidth: 0,
    paddingTop: 0,
  },
  prHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  prLift: {
    color: '#0f172a',
    fontSize: 15,
    fontWeight: '600',
  },
  prValue: {
    color: '#0f172a',
    fontSize: 15,
    fontWeight: '700',
  },
  prMeta: {
    color: '#475569',
    fontSize: 13,
  },
  habitRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 8,
  },
  habitCopy: {
    color: '#0f172a',
    flex: 1,
  },
  helperText: {
    color: '#334155',
  },
  readinessQuestion: {
    gap: 6,
  },
  inlineButtonRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
  },
  planRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    gap: 8,
  },
  planDay: {
    color: '#0f172a',
    fontWeight: '600',
    minWidth: 96,
  },
  planSession: {
    color: '#334155',
    flex: 1,
  },
  error: {
    color: '#b91c1c',
    fontSize: 14,
  },
});
