import React, { useMemo, useState } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import { useNavigation } from '@react-navigation/native';
import {
  createFoodLog,
  type FoodLog,
  type FoodLogsResult,
  listFoodLogs,
  searchFoods,
  type Food,
  type NutrientValues,
} from '../../api/services/foodService';
import { toUTCDateKey } from '../../api/dateKey';
import { isNetworkOnline } from '../../network/onlineManager';
import { hasBarcodeScanEntitlement } from '../../features/entitlements';
import type { MainTabParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { enqueueFoodLogOutboxItem, flushOutbox } from '../../sync/outbox';
import { Button, Card, Input } from '../../ui';

function formatNutrient(value: number | null | undefined, unit: string): string {
  if (value == null) {
    return `-- ${unit}`;
  }
  return `${value.toFixed(1)} ${unit}`;
}

function formatFoodMacroRow(nutrients: NutrientValues): string {
  return `Cals ${formatNutrient(nutrients.calories_kcal, 'kcal')} · P ${formatNutrient(
    nutrients.protein_g,
    'g',
  )} · C ${formatNutrient(nutrients.carbs_g, 'g')} · F ${formatNutrient(nutrients.fat_g, 'g')}`;
}

function formatQuantity(value: number): string {
  if (Number.isInteger(value)) {
    return `${value}`;
  }
  return value.toFixed(2);
}

function scaleMacro(value: number | null | undefined, quantity: number): number | null {
  if (value == null) {
    return null;
  }
  return value * quantity;
}

function scaleNutrients(nutrients: NutrientValues, quantity: number): NutrientValues {
  return {
    calories_kcal: scaleMacro(nutrients.calories_kcal, quantity),
    protein_g: scaleMacro(nutrients.protein_g, quantity),
    carbs_g: scaleMacro(nutrients.carbs_g, quantity),
    fat_g: scaleMacro(nutrients.fat_g, quantity),
  };
}

function sumMacro(left: number | null | undefined, right: number | null | undefined): number {
  return (left ?? 0) + (right ?? 0);
}

function appendLogResult(
  current: FoodLogsResult | undefined,
  log: FoodLog,
  dateKey: string,
): FoodLogsResult {
  const existingLogs = current?.logs ?? [];
  return {
    date: current?.date ?? dateKey,
    logs: [log, ...existingLogs.filter(existing => existing.id !== log.id)],
    totals: {
      calories_kcal: sumMacro(current?.totals?.calories_kcal, log.nutrientsSnapshot.calories_kcal),
      protein_g: sumMacro(current?.totals?.protein_g, log.nutrientsSnapshot.protein_g),
      carbs_g: sumMacro(current?.totals?.carbs_g, log.nutrientsSnapshot.carbs_g),
      fat_g: sumMacro(current?.totals?.fat_g, log.nutrientsSnapshot.fat_g),
    },
  };
}

function buildOfflineFoodLog(params: {
  userId: string | undefined;
  food: Food;
  quantity: number;
  unit: string;
  datetime: string;
  idempotencyKey: string;
}): FoodLog {
  return {
    id: `offline-${params.idempotencyKey}`,
    userId: params.userId ?? 'offline-user',
    datetime: params.datetime,
    foodId: params.food.id,
    quantity: params.quantity,
    unit: params.unit,
    nutrientsSnapshot: scaleNutrients(params.food.nutrients, params.quantity),
    createdAt: params.datetime,
    food: params.food,
  };
}

export function FoodScreen(): React.JSX.Element {
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';
  const queryClient = useQueryClient();
  const navigation = useNavigation<BottomTabNavigationProp<MainTabParamList>>();
  const dateKey = toUTCDateKey();
  const canScanBarcode = hasBarcodeScanEntitlement(session?.user);

  const [searchInput, setSearchInput] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [quantityInput, setQuantityInput] = useState('1');
  const [validationError, setValidationError] = useState<string | undefined>();
  const [statusMessage, setStatusMessage] = useState<string | undefined>();

  const foodsQuery = useQuery({
    queryKey: ['foods', 'search', session?.user.id, searchQuery],
    queryFn: () =>
      searchFoods({
        accessToken,
        query: searchQuery,
      }),
    enabled: Boolean(accessToken) && searchQuery.length > 0,
  });

  const foodLogsQuery = useQuery({
    queryKey: ['food-logs', session?.user.id, dateKey],
    queryFn: () =>
      listFoodLogs({
        accessToken,
        dateKey,
      }),
    enabled: Boolean(accessToken),
  });

  const logFoodMutation = useMutation({
    mutationFn: async ({ food }: { food: Food }) => {
      const quantity = Number(quantityInput.trim().replace(',', '.'));
      if (!Number.isFinite(quantity) || quantity <= 0) {
        throw new Error('Enter a quantity greater than 0.');
      }

      const unit = 'serving';
      if (!isNetworkOnline()) {
        const datetime = new Date().toISOString();
        const idempotencyKey = `food-log-${dateKey}-${food.id}-${Date.now()}`;
        await enqueueFoodLogOutboxItem({
          idempotencyKey,
          foodId: food.id,
          quantity,
          unit,
          datetime,
        });

        return buildOfflineFoodLog({
          userId: session?.user.id,
          food,
          quantity,
          unit,
          datetime,
          idempotencyKey,
        });
      }

      return createFoodLog({
        accessToken,
        foodId: food.id,
        quantity,
        unit,
      });
    },
    onSuccess: async log => {
      setValidationError(undefined);
      queryClient.setQueryData<FoodLogsResult>(
        ['food-logs', session?.user.id, dateKey],
        current => appendLogResult(current, log, dateKey),
      );

      const queuedOffline = log.id.startsWith('offline-');
      setStatusMessage(
        queuedOffline ? `Queued ${log.food.label}. It will sync when online.` : `Logged ${log.food.label}.`,
      );

      if (!queuedOffline) {
        await Promise.all([
          queryClient.invalidateQueries({
            queryKey: ['food-logs', session?.user.id],
          }),
          queryClient.invalidateQueries({
            queryKey: ['dashboard', session?.user.id],
          }),
        ]);
      }

      if (isNetworkOnline()) {
        const flushResult = await flushOutbox(accessToken);
        if (flushResult.flushedCount > 0) {
          await Promise.all([
            queryClient.invalidateQueries({
              queryKey: ['food-logs', session?.user.id],
            }),
            queryClient.invalidateQueries({
              queryKey: ['dashboard', session?.user.id],
            }),
          ]);
        }
      }
    },
    onError: error => {
      setStatusMessage(undefined);
      setValidationError(error instanceof Error ? error.message : 'Unable to log food.');
    },
  });

  const foods = foodsQuery.data ?? [];
  const logRows = foodLogsQuery.data?.logs ?? [];
  const totals = foodLogsQuery.data?.totals;

  const hasSearchResults = useMemo(() => foods.length > 0, [foods.length]);

  function submitSearch() {
    const normalized = searchInput.trim();
    if (!normalized) {
      setValidationError('Enter a search term.');
      return;
    }
    setValidationError(undefined);
    setStatusMessage(undefined);
    setSearchQuery(normalized);
  }

  function handleLogFood(food: Food) {
    setStatusMessage(undefined);
    logFoodMutation.mutate({ food });
  }

  return (
    <ScrollView contentContainerStyle={styles.container} testID="food-screen">
      <Text style={styles.title}>Food</Text>
      <Text style={styles.subtitle}>Search foods and log meals for today.</Text>
      <Button
        label={canScanBarcode ? 'Scan Barcode' : 'Scan Barcode (Pro)'}
        variant={canScanBarcode ? 'primary' : 'secondary'}
        onPress={() => {
          if (canScanBarcode) {
            navigation.navigate('BarcodeScan');
            return;
          }
          navigation.navigate('Paywall', {
            feature: 'barcode_scan',
          });
        }}
        testID="food-scan-button"
      />

      <Card>
        <Text style={styles.sectionTitle}>Search</Text>
        <Input
          label="Food name"
          value={searchInput}
          onChangeText={setSearchInput}
          placeholder="e.g. greek yogurt"
          autoCapitalize="none"
          testID="food-search-input"
        />
        <Input
          label="Quantity (servings)"
          value={quantityInput}
          onChangeText={setQuantityInput}
          keyboardType="decimal-pad"
          placeholder="1"
          testID="food-quantity-input"
        />
        <Button
          label="Search"
          onPress={submitSearch}
          loading={foodsQuery.isFetching}
          disabled={foodsQuery.isFetching}
          testID="food-search-button"
        />

        {validationError ? <Text style={styles.error}>{validationError}</Text> : null}
        {statusMessage ? <Text style={styles.success}>{statusMessage}</Text> : null}

        {foodsQuery.isLoading ? (
          <ActivityIndicator size="small" color="#0f766e" testID="food-search-loading" />
        ) : foodsQuery.isError ? (
          <Text style={styles.error}>Unable to search foods right now.</Text>
        ) : searchQuery.length > 0 && !hasSearchResults ? (
          <Text style={styles.helperText}>No foods found for that search.</Text>
        ) : (
          foods.map(food => (
            <View key={food.id} style={styles.foodRow} testID={`food-result-${food.id}`}>
              <View style={styles.foodCopy}>
                <Text style={styles.foodLabel}>{food.label}</Text>
                {food.brand ? <Text style={styles.foodBrand}>{food.brand}</Text> : null}
                <Text style={styles.foodMacros}>{formatFoodMacroRow(food.nutrients)}</Text>
              </View>
              <Button
                label="Log"
                variant="secondary"
                onPress={() => handleLogFood(food)}
                loading={logFoodMutation.isPending && logFoodMutation.variables?.food.id === food.id}
                disabled={logFoodMutation.isPending && logFoodMutation.variables?.food.id === food.id}
                testID={`food-log-${food.id}`}
              />
            </View>
          ))
        )}
      </Card>

      <Card>
        <Text style={styles.sectionTitle}>Today</Text>
        {foodLogsQuery.isLoading ? (
          <ActivityIndicator size="small" color="#0f766e" testID="food-logs-loading" />
        ) : foodLogsQuery.isError ? (
          <Text style={styles.error}>Unable to load today&apos;s logs.</Text>
        ) : (
          <>
            <Text style={styles.totalRow}>
              {`Totals · ${formatNutrient(totals?.calories_kcal, 'kcal')} · P ${formatNutrient(
                totals?.protein_g,
                'g',
              )} · C ${formatNutrient(totals?.carbs_g, 'g')} · F ${formatNutrient(
                totals?.fat_g,
                'g',
              )}`}
            </Text>
            {logRows.length === 0 ? (
              <Text style={styles.helperText}>No foods logged yet today.</Text>
            ) : (
              logRows.map(log => (
                <View key={log.id} style={styles.logRow}>
                  <Text style={styles.logLabel}>
                    {`${log.food.label} · ${formatQuantity(log.quantity)} ${log.unit}`}
                  </Text>
                  <Text style={styles.logMacros}>{formatFoodMacroRow(log.nutrientsSnapshot)}</Text>
                </View>
              ))
            )}
          </>
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
  helperText: {
    color: '#475569',
    fontSize: 13,
  },
  error: {
    color: '#b91c1c',
    fontSize: 13,
  },
  success: {
    color: '#0f766e',
    fontSize: 13,
  },
  foodRow: {
    borderTopWidth: 1,
    borderTopColor: '#e2e8f0',
    paddingTop: 10,
    gap: 8,
  },
  foodCopy: {
    gap: 2,
  },
  foodLabel: {
    color: '#0f172a',
    fontWeight: '700',
    fontSize: 15,
  },
  foodBrand: {
    color: '#475569',
    fontSize: 13,
  },
  foodMacros: {
    color: '#334155',
    fontSize: 12,
  },
  totalRow: {
    color: '#0f172a',
    fontWeight: '700',
    fontSize: 13,
  },
  logRow: {
    borderTopWidth: 1,
    borderTopColor: '#e2e8f0',
    paddingTop: 10,
    gap: 2,
  },
  logLabel: {
    color: '#0f172a',
    fontSize: 14,
    fontWeight: '600',
  },
  logMacros: {
    color: '#334155',
    fontSize: 12,
  },
});
