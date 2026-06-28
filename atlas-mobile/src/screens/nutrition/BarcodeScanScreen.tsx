import React, { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Platform, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import { useNavigation } from '@react-navigation/native';
import { Camera } from 'react-native-camera-kit';
import { PERMISSIONS, RESULTS, check, openSettings, request, type Permission } from 'react-native-permissions';
import {
  createFoodLog,
  lookupFoodByUpc,
  searchFoods,
  type Food,
  type FoodLog,
  type FoodLogsResult,
  type NutrientValues,
} from '../../api/services/foodService';
import { toUTCDateKey } from '../../api/dateKey';
import { hasBarcodeScanEntitlement } from '../../features/entitlements';
import { isNetworkOnline } from '../../network/onlineManager';
import type { MainTabParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { enqueueFoodLogOutboxItem, flushOutbox } from '../../sync/outbox';
import { Button, Card, Input } from '../../ui';

const CAMERA_PERMISSION: Permission =
  Platform.OS === 'ios' ? PERMISSIONS.IOS.CAMERA : PERMISSIONS.ANDROID.CAMERA;

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

function normalizeScannedCode(rawValue: string | undefined): string | null {
  if (!rawValue) {
    return null;
  }
  const digitsOnly = rawValue.replace(/\D/g, '');
  if (digitsOnly.length < 8 || digitsOnly.length > 14) {
    return null;
  }
  return digitsOnly;
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

function mergeCandidates(primary: Food, results: Food[]): Food[] {
  const seen = new Set<string>();
  const merged: Food[] = [];
  for (const item of [primary, ...results]) {
    if (seen.has(item.id)) {
      continue;
    }
    seen.add(item.id);
    merged.push(item);
  }
  return merged;
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

export function BarcodeScanScreen(): React.JSX.Element {
  const navigation = useNavigation<BottomTabNavigationProp<MainTabParamList>>();
  const { session } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';
  const isPro = hasBarcodeScanEntitlement(session?.user);
  const queryClient = useQueryClient();
  const dateKey = toUTCDateKey();

  const [cameraPermission, setCameraPermission] = useState<string>('checking');
  const [scanLocked, setScanLocked] = useState(false);
  const [scannedCode, setScannedCode] = useState<string | null>(null);
  const [candidates, setCandidates] = useState<Food[]>([]);
  const [selectedFood, setSelectedFood] = useState<Food | null>(null);
  const [servingInput, setServingInput] = useState('1');
  const [statusMessage, setStatusMessage] = useState<string | undefined>();
  const [errorMessage, setErrorMessage] = useState<string | undefined>();

  const lookupMutation = useMutation({
    mutationFn: async (code: string) => {
      const primary = await lookupFoodByUpc({
        accessToken,
        code,
      });

      try {
        const similar = await searchFoods({
          accessToken,
          query: primary.label,
          limit: 5,
        });
        return mergeCandidates(primary, similar);
      } catch {
        return [primary];
      }
    },
    onSuccess: foods => {
      setErrorMessage(undefined);
      setStatusMessage('Pick the best match, then confirm serving.');
      setCandidates(foods);
      setSelectedFood(null);
      setServingInput('1');
    },
    onError: error => {
      setCandidates([]);
      setSelectedFood(null);
      setScanLocked(false);
      setStatusMessage(undefined);
      setErrorMessage(error instanceof Error ? error.message : 'Unable to lookup barcode.');
    },
  });

  const logMutation = useMutation({
    mutationFn: async (input: { food: Food; quantity: number }) => {
      const unit = 'serving';
      if (!isNetworkOnline()) {
        const datetime = new Date().toISOString();
        const idempotencyKey = `food-log-${dateKey}-${input.food.id}-${Date.now()}`;
        await enqueueFoodLogOutboxItem({
          idempotencyKey,
          foodId: input.food.id,
          quantity: input.quantity,
          unit,
          datetime,
        });

        return buildOfflineFoodLog({
          userId: session?.user.id,
          food: input.food,
          quantity: input.quantity,
          unit,
          datetime,
          idempotencyKey,
        });
      }

      return createFoodLog({
        accessToken,
        foodId: input.food.id,
        quantity: input.quantity,
        unit,
      });
    },
    onSuccess: async log => {
      setErrorMessage(undefined);
      queryClient.setQueryData<FoodLogsResult>(
        ['food-logs', session?.user.id, dateKey],
        current => appendLogResult(current, log, dateKey),
      );

      const queuedOffline = log.id.startsWith('offline-');
      setStatusMessage(
        queuedOffline ? `Queued ${log.food.label}. It will sync when online.` : `Logged ${log.food.label}.`,
      );
      setCandidates([]);
      setSelectedFood(null);
      setScannedCode(null);
      setScanLocked(false);

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
      setErrorMessage(error instanceof Error ? error.message : 'Unable to log scanned food.');
    },
  });

  function submitLogSelection() {
    if (!selectedFood) {
      setStatusMessage(undefined);
      setErrorMessage('Select a food before logging.');
      return;
    }

    const quantity = Number(servingInput.trim().replace(',', '.'));
    if (!Number.isFinite(quantity) || quantity <= 0) {
      setStatusMessage(undefined);
      setErrorMessage('Enter a serving quantity greater than 0.');
      return;
    }

    setErrorMessage(undefined);
    logMutation.mutate({
      food: selectedFood,
      quantity,
    });
  }

  const checkPermission = useCallback(async () => {
    const status = await check(CAMERA_PERMISSION);
    setCameraPermission(status);
  }, []);

  useEffect(() => {
    if (!isPro) {
      return;
    }
    checkPermission().catch(() => {
      setCameraPermission(RESULTS.UNAVAILABLE);
    });
  }, [checkPermission, isPro]);

  async function requestPermission() {
    const status = await request(CAMERA_PERMISSION);
    setCameraPermission(status);
  }

  async function handlePermissionAction() {
    if (cameraPermission === RESULTS.BLOCKED) {
      await openSettings();
      return;
    }
    await requestPermission();
  }

  function resetScanner() {
    setScanLocked(false);
    setScannedCode(null);
    setCandidates([]);
    setSelectedFood(null);
    setServingInput('1');
    setStatusMessage(undefined);
    setErrorMessage(undefined);
  }

  function handleReadCode(event: { nativeEvent: { codeStringValue: string } }) {
    if (scanLocked || lookupMutation.isPending || logMutation.isPending) {
      return;
    }

    const normalizedCode = normalizeScannedCode(event.nativeEvent.codeStringValue);
    if (!normalizedCode) {
      setStatusMessage(undefined);
      setErrorMessage('Scanned code is not a valid UPC/EAN barcode.');
      return;
    }

    setScanLocked(true);
    setScannedCode(normalizedCode);
    setCandidates([]);
    setSelectedFood(null);
    setServingInput('1');
    setStatusMessage(undefined);
    setErrorMessage(undefined);
    lookupMutation.mutate(normalizedCode);
  }

  const isPermissionGranted = cameraPermission === RESULTS.GRANTED || cameraPermission === RESULTS.LIMITED;

  return (
    <ScrollView contentContainerStyle={styles.container} testID="barcode-scan-screen">
      <Text style={styles.title}>Barcode Scan</Text>
      <Text style={styles.subtitle}>Scan a package UPC to lookup and log food quickly.</Text>

      {!isPro ? (
        <Card testID="barcode-paywall">
          <Text style={styles.sectionTitle}>Pro Feature</Text>
          <Text style={styles.helperText}>
            Barcode scanning is available on Pro. Upgrade to unlock instant UPC lookup and logging.
          </Text>
          <Button
            label="Upgrade to Pro"
            variant="secondary"
            onPress={() => {
              navigation.navigate('Paywall', {
                feature: 'barcode_scan',
              });
            }}
            testID="barcode-upsell-button"
          />
        </Card>
      ) : cameraPermission === 'checking' ? (
        <Card>
          <ActivityIndicator size="small" color="#0f766e" testID="barcode-permission-loading" />
          <Text style={styles.helperText}>Checking camera permission...</Text>
        </Card>
      ) : !isPermissionGranted ? (
        <Card testID="barcode-permission-card">
          <Text style={styles.sectionTitle}>Camera Permission Required</Text>
          <Text style={styles.helperText}>
            {cameraPermission === RESULTS.BLOCKED
              ? 'Camera access is blocked. Open settings to allow access.'
              : 'Allow camera access to scan barcodes.'}
          </Text>
          <Button
            label={cameraPermission === RESULTS.BLOCKED ? 'Open Settings' : 'Allow Camera'}
            onPress={() => {
              handlePermissionAction().catch(() => {
                setErrorMessage('Unable to update camera permission.');
              });
            }}
            testID="barcode-permission-button"
          />
        </Card>
      ) : (
        <Card>
          <View style={styles.cameraWrapper}>
            <Camera
              style={styles.camera}
              scanBarcode={!scanLocked}
              onReadCode={handleReadCode}
              allowedBarcodeTypes={['ean-13', 'ean-8', 'upc-a', 'upc-e']}
              scanThrottleDelay={1000}
              showFrame
              frameColor="#0f172a"
              laserColor="#0f766e"
              testID="barcode-camera"
            />
          </View>
          <Text style={styles.helperText}>
            {scanLocked
              ? 'Barcode captured. Reviewing result...'
              : 'Align the barcode inside the frame to scan.'}
          </Text>
          {scannedCode ? <Text style={styles.codeText}>Scanned: {scannedCode}</Text> : null}
        </Card>
      )}

      {lookupMutation.isPending ? (
        <Card>
          <ActivityIndicator size="small" color="#0f766e" testID="barcode-lookup-loading" />
          <Text style={styles.helperText}>Looking up barcode...</Text>
        </Card>
      ) : null}

      {candidates.length > 0 ? (
        <Card testID="barcode-candidates-card">
          <Text style={styles.sectionTitle}>Choose Food</Text>
          {candidates.map(food => (
            <View key={food.id} style={styles.candidateRow} testID={`barcode-candidate-${food.id}`}>
              <View style={styles.foodCopy}>
                <Text style={styles.foodLabel}>{food.label}</Text>
                {food.brand ? <Text style={styles.foodBrand}>{food.brand}</Text> : null}
                <Text style={styles.foodMacros}>{formatFoodMacroRow(food.nutrients)}</Text>
              </View>
              <Button
                label={selectedFood?.id === food.id ? 'Selected' : 'Select'}
                variant={selectedFood?.id === food.id ? 'primary' : 'secondary'}
                onPress={() => {
                  setSelectedFood(food);
                  setErrorMessage(undefined);
                }}
                testID={`barcode-candidate-select-${food.id}`}
              />
            </View>
          ))}
          <Button
            label="Scan another"
            variant="secondary"
            onPress={resetScanner}
            disabled={logMutation.isPending}
            testID="barcode-scan-again-button"
          />
        </Card>
      ) : null}

      {selectedFood ? (
        <Card testID="barcode-confirm-card">
          <Text style={styles.sectionTitle}>Confirm Serving</Text>
          <Text style={styles.foodLabel}>{selectedFood.label}</Text>
          <Text style={styles.foodMacros}>{formatFoodMacroRow(selectedFood.nutrients)}</Text>
          <Input
            label="Serving quantity"
            value={servingInput}
            onChangeText={setServingInput}
            keyboardType="decimal-pad"
            placeholder="1"
            testID="barcode-serving-input"
          />
          <Button
            label="Log serving"
            onPress={submitLogSelection}
            loading={logMutation.isPending}
            disabled={logMutation.isPending}
            testID="barcode-log-button"
          />
        </Card>
      ) : null}

      {errorMessage ? <Text style={styles.error}>{errorMessage}</Text> : null}
      {statusMessage ? <Text style={styles.success}>{statusMessage}</Text> : null}
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
  cameraWrapper: {
    borderRadius: 12,
    overflow: 'hidden',
    borderWidth: 1,
    borderColor: '#cbd5e1',
  },
  camera: {
    width: '100%',
    minHeight: 280,
  },
  helperText: {
    color: '#475569',
    fontSize: 13,
  },
  codeText: {
    color: '#334155',
    fontSize: 13,
    fontWeight: '600',
  },
  foodLabel: {
    color: '#0f172a',
    fontWeight: '700',
    fontSize: 16,
  },
  foodBrand: {
    color: '#475569',
    fontSize: 13,
  },
  foodMacros: {
    color: '#334155',
    fontSize: 12,
  },
  foodCopy: {
    gap: 2,
    flex: 1,
  },
  candidateRow: {
    gap: 10,
    paddingTop: 10,
    borderTopWidth: 1,
    borderTopColor: '#e2e8f0',
  },
  error: {
    color: '#b91c1c',
    fontSize: 13,
  },
  success: {
    color: '#0f766e',
    fontSize: 13,
  },
});
