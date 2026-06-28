import React, { useMemo, useState } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import { useNavigation } from '@react-navigation/native';
import {
  generateNutritionMealPlan,
  getLatestNutritionMealPlan,
  upsertNutritionMealPlan,
  type NutritionMealPlan,
  type NutritionMealPlanItem,
} from '../../api/services/nutritionService';
import { hasDeepNutritionEntitlement } from '../../features/entitlements';
import type { MainTabParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { Button, Card, Input } from '../../ui';

type PlanView = 'plan' | 'grocery';

function dayLabel(dayOfWeek: number): string {
  switch (dayOfWeek) {
    case 1:
      return 'Mon';
    case 2:
      return 'Tue';
    case 3:
      return 'Wed';
    case 4:
      return 'Thu';
    case 5:
      return 'Fri';
    case 6:
      return 'Sat';
    case 7:
      return 'Sun';
    default:
      return `Day ${dayOfWeek}`;
  }
}

function formatServings(value: number): string {
  if (Number.isInteger(value)) {
    return `${value}`;
  }
  return value.toFixed(2);
}

function mealSlotLabel(value: string): string {
  if (!value) {
    return 'Meal';
  }
  return value.charAt(0).toUpperCase() + value.slice(1);
}

export function MealPlanScreen(): React.JSX.Element {
  const navigation = useNavigation<BottomTabNavigationProp<MainTabParamList>>();
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';
  const hasDeepNutrition = hasDeepNutritionEntitlement(session?.user);
  const queryClient = useQueryClient();
  const [activeView, setActiveView] = useState<PlanView>('plan');
  const [servingDrafts, setServingDrafts] = useState<Record<string, string>>({});
  const [servingValidationError, setServingValidationError] = useState<string | undefined>();

  const mealPlanQuery = useQuery({
    queryKey: ['nutrition', 'meal-plan', session?.user.id],
    queryFn: () => getLatestNutritionMealPlan({ accessToken }),
    enabled: Boolean(accessToken),
    retry: false,
  });

  const generateMutation = useMutation({
    mutationFn: () => generateNutritionMealPlan({ accessToken }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['nutrition', 'meal-plan', session?.user.id],
      });
    },
  });

  const savePlanMutation = useMutation({
    mutationFn: (items: Array<{dayOfWeek: number; mealSlot: string; recipeId: string; servings: number}>) =>
      upsertNutritionMealPlan({
        accessToken,
        weekStartDateKey: mealPlan?.week_start ?? '',
        items,
      }),
    onSuccess: async () => {
      setServingValidationError(undefined);
      setServingDrafts({});
      await queryClient.invalidateQueries({
        queryKey: ['nutrition', 'meal-plan', session?.user.id],
      });
    },
  });

  const mealPlan: NutritionMealPlan | undefined = mealPlanQuery.data;

  const planItems = useMemo(
    () =>
      [...(mealPlan?.items ?? [])].sort((a, b) => {
        if (a.day_of_week !== b.day_of_week) {
          return a.day_of_week - b.day_of_week;
        }
        return a.meal_slot.localeCompare(b.meal_slot);
      }),
    [mealPlan?.items],
  );

  const groupedItems = useMemo(() => {
    const grouped = new Map<number, NutritionMealPlanItem[]>();
    for (const item of planItems) {
      const entries = grouped.get(item.day_of_week) ?? [];
      entries.push(item);
      grouped.set(item.day_of_week, entries);
    }
    return grouped;
  }, [planItems]);

  function itemDraftKey(item: NutritionMealPlanItem): string {
    return `${item.day_of_week}-${item.meal_slot}-${item.recipe.id}`;
  }

  function updateServingDraft(item: NutritionMealPlanItem, value: string): void {
    const key = itemDraftKey(item);
    setServingDrafts(current => ({
      ...current,
      [key]: value,
    }));
  }

  function resolveServingValue(item: NutritionMealPlanItem): number | null {
    const key = itemDraftKey(item);
    const draftValue = servingDrafts[key];
    if (draftValue == null || draftValue.trim() === '') {
      return item.servings;
    }

    const normalized = draftValue.trim().replace(',', '.');
    const parsed = Number(normalized);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      return null;
    }
    return parsed;
  }

  function saveMealPlanEdits(): void {
    if (!mealPlan || planItems.length === 0) {
      return;
    }

    const payload: Array<{dayOfWeek: number; mealSlot: string; recipeId: string; servings: number}> = [];
    for (const item of planItems) {
      const servings = resolveServingValue(item);
      if (servings == null) {
        setServingValidationError('Enter valid servings greater than 0 for all meals.');
        return;
      }

      payload.push({
        dayOfWeek: item.day_of_week,
        mealSlot: item.meal_slot,
        recipeId: item.recipe.id,
        servings,
      });
    }

    setServingValidationError(undefined);
    savePlanMutation.mutate(payload);
  }

  if (!hasDeepNutrition) {
    return (
      <ScrollView contentContainerStyle={styles.container} testID="meal-plan-paywall-screen">
        <Text style={styles.title}>Meal Plan</Text>
        <Text style={styles.subtitle}>Deep nutrition planning is available with Pro.</Text>
        <Card testID="meal-plan-paywall-card">
          <Text style={styles.sectionTitle}>Upgrade Required</Text>
          <Text style={styles.helperText}>
            Unlock weekly meal generation, edit workflows, and grocery auto-regeneration.
          </Text>
          <Button
            label="View Plans"
            onPress={() => {
              navigation.navigate('Paywall', {
                feature: 'deep_nutrition',
              });
            }}
            testID="meal-plan-paywall-button"
          />
        </Card>
      </ScrollView>
    );
  }

  return (
    <ScrollView contentContainerStyle={styles.container} testID="meal-plan-screen">
      <Text style={styles.title}>Meal Plan</Text>
      <Text style={styles.subtitle}>Generate weekly meals and a grocery list from your targets.</Text>

      <Card>
        <Text style={styles.sectionTitle}>Generate</Text>
        <Button
          label="Generate Meal Plan"
          onPress={() => generateMutation.mutate()}
          loading={generateMutation.isPending}
          disabled={generateMutation.isPending}
          testID="meal-plan-generate-button"
        />
        {generateMutation.isError ? (
          <Text style={styles.error}>Unable to generate meal plan.</Text>
        ) : null}
        {mealPlan ? (
          <Text style={styles.helperText}>{`Week of ${mealPlan.week_start}`}</Text>
        ) : (
          <Text style={styles.helperText}>No meal plan generated yet.</Text>
        )}
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Targets</Text>
        {mealPlan ? (
          <View style={styles.targetsRow}>
            <Text style={styles.targetValue}>{`${mealPlan.targets.calories_target} kcal`}</Text>
            <Text style={styles.targetValue}>{`P ${mealPlan.targets.protein_g_target}g`}</Text>
            <Text style={styles.targetValue}>{`C ${mealPlan.targets.carbs_g_target}g`}</Text>
            <Text style={styles.targetValue}>{`F ${mealPlan.targets.fat_g_target}g`}</Text>
          </View>
        ) : (
          <Text style={styles.helperText}>Targets appear after generation.</Text>
        )}
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>View</Text>
        <View style={styles.toggleRow}>
          <Button
            label="Plan"
            variant={activeView === 'plan' ? 'primary' : 'secondary'}
            onPress={() => setActiveView('plan')}
            testID="meal-plan-view-plan"
          />
          <Button
            label="Grocery"
            variant={activeView === 'grocery' ? 'primary' : 'secondary'}
            onPress={() => setActiveView('grocery')}
            testID="meal-plan-view-grocery"
          />
        </View>

        {mealPlanQuery.isLoading ? (
          <ActivityIndicator size="small" color="#0f766e" testID="meal-plan-loading" />
        ) : mealPlanQuery.isError ? (
          <Text style={styles.helperText}>Generate a meal plan to see plan and grocery details.</Text>
        ) : !mealPlan ? (
          <Text style={styles.helperText}>Generate a meal plan to get started.</Text>
        ) : activeView === 'plan' ? (
          Array.from(groupedItems.entries()).map(([dayOfWeek, items]) => (
            <View key={dayOfWeek} style={styles.daySection} testID={`meal-plan-day-${dayOfWeek}`}>
              <Text style={styles.dayHeader}>{dayLabel(dayOfWeek)}</Text>
              {items.map(item => (
                <View
                  key={`${dayOfWeek}-${item.meal_slot}-${item.recipe.id}`}
                  style={styles.itemRow}
                  testID={`meal-plan-item-${dayOfWeek}-${item.meal_slot}`}>
                  <Text style={styles.itemTitle}>{`${mealSlotLabel(item.meal_slot)} · ${item.recipe.name}`}</Text>
                  <Text style={styles.itemMeta}>{`${formatServings(item.servings)} servings · ${item.recipe.calories_kcal} kcal`}</Text>
                  <Input
                    label="Servings"
                    value={servingDrafts[itemDraftKey(item)] ?? ''}
                    onChangeText={value => updateServingDraft(item, value)}
                    keyboardType="decimal-pad"
                    placeholder={formatServings(item.servings)}
                    testID={`meal-plan-serving-input-${dayOfWeek}-${item.meal_slot}`}
                  />
                </View>
              ))}
            </View>
          ))
        ) : mealPlan.grocery_items.length === 0 ? (
          <Text style={styles.helperText}>No grocery items available.</Text>
        ) : (
          mealPlan.grocery_items.map(item => (
            <View
              key={`${item.category}-${item.name}-${item.unit}`}
              style={styles.itemRow}
              testID={`grocery-item-${item.name}`}>
              <Text style={styles.itemTitle}>{item.name}</Text>
              <Text style={styles.itemMeta}>{`${item.quantity.toFixed(2)} ${item.unit} · ${item.category}`}</Text>
            </View>
          ))
        )}
        {mealPlan && activeView === 'plan' ? (
          <>
            <Button
              label="Save Meal Plan Edits"
              onPress={saveMealPlanEdits}
              loading={savePlanMutation.isPending}
              disabled={savePlanMutation.isPending || planItems.length === 0}
              testID="meal-plan-save-button"
            />
            {servingValidationError ? <Text style={styles.error}>{servingValidationError}</Text> : null}
            {savePlanMutation.isError ? (
              <Text style={styles.error}>Unable to save meal plan edits.</Text>
            ) : null}
          </>
        ) : null}
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
  helperText: {
    color: '#475569',
    fontSize: 13,
  },
  error: {
    color: '#b91c1c',
    fontSize: 13,
  },
  targetsRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    gap: 8,
    flexWrap: 'wrap',
  },
  targetValue: {
    color: '#0f172a',
    fontSize: 13,
    fontWeight: '700',
  },
  toggleRow: {
    flexDirection: 'row',
    gap: 8,
  },
  daySection: {
    borderTopWidth: 1,
    borderTopColor: '#e2e8f0',
    paddingTop: 10,
    gap: 6,
  },
  dayHeader: {
    color: '#0f172a',
    fontWeight: '700',
    fontSize: 14,
  },
  itemRow: {
    gap: 8,
  },
  itemTitle: {
    color: '#0f172a',
    fontSize: 14,
    fontWeight: '600',
  },
  itemMeta: {
    color: '#475569',
    fontSize: 12,
  },
});
