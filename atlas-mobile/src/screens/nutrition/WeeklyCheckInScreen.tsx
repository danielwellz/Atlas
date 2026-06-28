import React, { useMemo, useState } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getLatestNutritionWeeklyCheckin,
  getNutritionWeightTrend,
  runWeeklyNutritionCheckin,
  upsertNutritionWeightEntry,
  type NutritionWeeklyCheckin,
  type WeightUnit,
} from '../../api/services/nutritionService';
import { toUTCDateKey } from '../../api/dateKey';
import { useAuth } from '../../state/AuthContext';
import { Button, Card, Input } from '../../ui';

const WEIGHT_UNITS: WeightUnit[] = ['kg', 'lb'];

function formatTrendValue(weight: number | null | undefined, unit: WeightUnit | undefined): string {
  if (weight == null || !unit) {
    return '--';
  }
  return `${weight.toFixed(1)} ${unit}`;
}

function formatMacroTarget(value: number | null | undefined): string {
  if (value == null) {
    return '--';
  }
  return `${Math.round(value)}`;
}

function formatCalorieDelta(value: number): string {
  if (value === 0) {
    return 'No calorie change this week.';
  }

  const direction = value > 0 ? 'Increased' : 'Reduced';
  return `${direction} calories by ${Math.abs(Math.round(value))} kcal.`;
}

export function WeeklyCheckInScreen(): React.JSX.Element {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';
  const queryClient = useQueryClient();
  const [unit, setUnit] = useState<WeightUnit>('kg');
  const [weightInput, setWeightInput] = useState('');
  const [validationError, setValidationError] = useState<string | undefined>();

  const trendQuery = useQuery({
    queryKey: ['nutrition', 'weight-trend', session?.user.id],
    queryFn: () => getNutritionWeightTrend({ accessToken }),
    enabled: Boolean(accessToken),
  });

  const weeklyCheckinQuery = useQuery({
    queryKey: ['nutrition', 'weekly-checkin', session?.user.id],
    queryFn: () => getLatestNutritionWeeklyCheckin({ accessToken }),
    enabled: Boolean(accessToken),
    retry: false,
  });

  const saveWeightMutation = useMutation({
    mutationFn: (weight: number) =>
      upsertNutritionWeightEntry({
        accessToken,
        dateKey: toUTCDateKey(),
        weight,
        unit,
      }),
    onSuccess: async () => {
      setValidationError(undefined);
      setWeightInput('');
      await queryClient.invalidateQueries({
        queryKey: ['nutrition', 'weight-trend', session?.user.id],
      });
    },
  });

  const runCheckinMutation = useMutation({
    mutationFn: () => runWeeklyNutritionCheckin({ accessToken }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['nutrition', 'weekly-checkin', session?.user.id],
      });
    },
  });

  const trendRows = useMemo(
    () => [...(trendQuery.data ?? [])].reverse(),
    [trendQuery.data],
  );

  const latestCheckin: NutritionWeeklyCheckin | undefined = weeklyCheckinQuery.data;

  function submitWeightEntry() {
    const normalizedWeight = weightInput.trim().replace(',', '.');
    const parsedWeight = Number(normalizedWeight);

    if (!Number.isFinite(parsedWeight) || parsedWeight <= 0) {
      setValidationError('Enter a valid weight greater than 0.');
      return;
    }

    setValidationError(undefined);
    saveWeightMutation.mutate(parsedWeight);
  }

  return (
    <ScrollView contentContainerStyle={styles.container} testID="weekly-checkin-screen">
      <Text style={styles.title}>Weekly Check-in</Text>
      <Text style={styles.subtitle}>Log bodyweight, run target adjustments, and review your 8-week trend.</Text>

      <Card>
        <Text style={styles.sectionTitle}>Log Weight</Text>
        <Input
          label="Weight"
          value={weightInput}
          onChangeText={setWeightInput}
          keyboardType="decimal-pad"
          placeholder={unit === 'kg' ? 'e.g. 81.5' : 'e.g. 179.6'}
          error={validationError}
          testID="weekly-checkin-weight-input"
        />
        <View style={styles.unitRow}>
          {WEIGHT_UNITS.map(candidateUnit => (
            <Button
              key={candidateUnit}
              label={candidateUnit.toUpperCase()}
              variant={unit === candidateUnit ? 'primary' : 'secondary'}
              onPress={() => setUnit(candidateUnit)}
              testID={`weekly-checkin-unit-${candidateUnit}`}
            />
          ))}
        </View>
        <Button
          label="Save Weight"
          onPress={submitWeightEntry}
          loading={saveWeightMutation.isPending}
          disabled={saveWeightMutation.isPending}
          testID="weekly-checkin-save-button"
        />
        {saveWeightMutation.isError ? (
          <Text style={styles.error}>Unable to save weight entry.</Text>
        ) : null}
      </Card>

      <Card testID="weekly-checkin-adjustment-card">
        <Text style={styles.sectionTitle}>Recommended Targets</Text>
        <Button
          label="Run Weekly Check-In"
          onPress={() => runCheckinMutation.mutate()}
          loading={runCheckinMutation.isPending}
          disabled={runCheckinMutation.isPending}
          testID="weekly-checkin-run-button"
        />
        {runCheckinMutation.isError ? (
          <Text style={styles.error}>Unable to run weekly check-in.</Text>
        ) : null}
        {weeklyCheckinQuery.isLoading ? (
          <ActivityIndicator size="small" color="#0f766e" testID="weekly-checkin-loading" />
        ) : latestCheckin ? (
          <>
            <View style={styles.targetsHeaderRow}>
              <Text style={styles.targetHeaderLabel}>Target</Text>
              <Text style={styles.targetHeaderValue}>Before</Text>
              <Text style={styles.targetHeaderValue}>After</Text>
            </View>
            <View style={styles.targetsRow}>
              <Text style={styles.targetLabel}>Calories</Text>
              <Text style={styles.targetValue} testID="weekly-checkin-calories-before">
                {`${formatMacroTarget(latestCheckin.previous_targets.calories_target)} kcal`}
              </Text>
              <Text style={styles.targetValue} testID="weekly-checkin-calories-target">
                {`${formatMacroTarget(latestCheckin.new_targets.calories_target)} kcal`}
              </Text>
            </View>
            <View style={styles.targetsRow}>
              <Text style={styles.targetLabel}>Protein</Text>
              <Text style={styles.targetValue} testID="weekly-checkin-protein-before">
                {`${formatMacroTarget(latestCheckin.previous_targets.protein_g_target)} g`}
              </Text>
              <Text style={styles.targetValue} testID="weekly-checkin-protein-after">
                {`${formatMacroTarget(latestCheckin.new_targets.protein_g_target)} g`}
              </Text>
            </View>
            <View style={styles.targetsRow}>
              <Text style={styles.targetLabel}>Carbs</Text>
              <Text style={styles.targetValue} testID="weekly-checkin-carbs-before">
                {`${formatMacroTarget(latestCheckin.previous_targets.carbs_g_target)} g`}
              </Text>
              <Text style={styles.targetValue} testID="weekly-checkin-carbs-after">
                {`${formatMacroTarget(latestCheckin.new_targets.carbs_g_target)} g`}
              </Text>
            </View>
            <View style={styles.targetsRow}>
              <Text style={styles.targetLabel}>Fat</Text>
              <Text style={styles.targetValue} testID="weekly-checkin-fat-before">
                {`${formatMacroTarget(latestCheckin.previous_targets.fat_g_target)} g`}
              </Text>
              <Text style={styles.targetValue} testID="weekly-checkin-fat-after">
                {`${formatMacroTarget(latestCheckin.new_targets.fat_g_target)} g`}
              </Text>
            </View>
            <Text style={styles.helperText} testID="weekly-checkin-adjustment-summary">
              {`Observed ${latestCheckin.weight_change.toFixed(2)} kg/week vs goal pace ${latestCheckin.goal_pace_kg_per_week.toFixed(2)} kg/week. ${formatCalorieDelta(latestCheckin.calorie_delta)}`}
            </Text>
            <Text style={styles.helperText} testID="weekly-checkin-explanation">
              {latestCheckin.explanation}
            </Text>
          </>
        ) : (
          <Text style={styles.helperText}>Run your first weekly check-in to get updated targets.</Text>
        )}
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Weight Trend (8 Weeks)</Text>
        {trendQuery.isLoading ? (
          <ActivityIndicator size="small" color="#0f766e" testID="weekly-checkin-trend-loading" />
        ) : trendQuery.isError ? (
          <Text style={styles.error}>Unable to load trend.</Text>
        ) : trendRows.length === 0 ? (
          <Text style={styles.helperText}>No trend data available yet.</Text>
        ) : (
          trendRows.map(point => (
            <View key={point.weekStartDate} style={styles.trendRow} testID={`weight-trend-row-${point.weekStartDate}`}>
              <View style={styles.trendHeader}>
                <Text style={styles.trendWeek}>{`Week of ${point.weekStartDate}`}</Text>
                <Text
                  style={styles.trendValue}
                  testID={`weight-trend-value-${point.weekStartDate}`}>
                  {formatTrendValue(point.weight, point.unit)}
                </Text>
              </View>
              {point.entryDate ? (
                <Text style={styles.helperText}>{`Logged on ${point.entryDate}`}</Text>
              ) : (
                <Text style={styles.helperText}>No entry</Text>
              )}
            </View>
          ))
        )}
      </Card>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: 16,
    gap: 12,
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
    fontSize: 18,
    fontWeight: '700',
    color: '#0f172a',
  },
  unitRow: {
    flexDirection: 'row',
    gap: 8,
  },
  targetsRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: 8,
  },
  targetsHeaderRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: 8,
  },
  targetLabel: {
    color: '#334155',
    fontSize: 14,
    flex: 1,
  },
  targetHeaderLabel: {
    color: '#475569',
    fontSize: 12,
    fontWeight: '600',
    flex: 1,
  },
  targetHeaderValue: {
    color: '#475569',
    fontSize: 12,
    fontWeight: '600',
    minWidth: 70,
    textAlign: 'right',
  },
  targetValue: {
    color: '#0f172a',
    fontSize: 14,
    fontWeight: '700',
    minWidth: 70,
    textAlign: 'right',
  },
  trendRow: {
    borderTopWidth: 1,
    borderTopColor: '#e2e8f0',
    paddingTop: 10,
    gap: 4,
  },
  trendHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 8,
  },
  trendWeek: {
    color: '#0f172a',
    fontSize: 14,
    fontWeight: '600',
  },
  trendValue: {
    color: '#0f172a',
    fontSize: 14,
    fontWeight: '700',
  },
  helperText: {
    color: '#475569',
    fontSize: 13,
  },
  error: {
    color: '#b91c1c',
    fontSize: 14,
  },
});
