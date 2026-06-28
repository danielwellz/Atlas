import React, { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useNavigation, useRoute } from '@react-navigation/native';
import type { BottomTabNavigationProp } from '@react-navigation/bottom-tabs';
import type { RouteProp } from '@react-navigation/native';
import { verifyBillingReceipt } from '../../api/services/billingService';
import {
  SUBSCRIPTION_PRODUCT_IDS,
  closeIAPConnection,
  extractPurchaseExpiryDate,
  extractPurchaseOriginalTransactionId,
  extractPurchaseReceiptToken,
  extractPurchaseTransactionId,
  finalizeStorePurchase,
  inferBillingPlatform,
  initializeIAP,
  listSubscriptionProducts,
  purchaseSubscriptionBySku,
  restoreAvailablePurchases,
  type StoreSubscription,
} from '../../features/billing/iap';
import type { MainTabParamList } from '../../navigation/types';
import { useAuth } from '../../state/AuthContext';
import { Button, Card } from '../../ui';

type PaywallNavigation = BottomTabNavigationProp<MainTabParamList, 'Paywall'>;
type PaywallRoute = RouteProp<MainTabParamList, 'Paywall'>;

const PLAN_COPY: Record<string, { title: string; details: string }> = {
  'atlas.pro.monthly': {
    title: 'Atlas Pro Monthly',
    details: 'Unlock barcode scan, deep nutrition planning, biomechanics overlays, and upload form checks.',
  },
  'atlas.pro.yearly': {
    title: 'Atlas Pro Yearly',
    details: 'Yearly Pro access for nutrition depth, biomechanics overlays, and coach upload workflows.',
  },
  'atlas.elite.monthly': {
    title: 'Atlas Elite Monthly',
    details: 'Everything in Pro plus Elite coach session tier access.',
  },
  'atlas.elite.yearly': {
    title: 'Atlas Elite Yearly',
    details: 'Full Atlas Elite access with yearly billing.',
  },
};

function paywallHeading(feature: MainTabParamList['Paywall']['feature']): string {
  switch (feature) {
    case 'barcode_scan':
      return 'Barcode Scan Requires Pro';
    case 'deep_nutrition':
      return 'Deep Nutrition Requires Pro';
    case 'biomechanics_overlays':
      return 'Biomechanics Overlay Requires Pro';
    case 'form_check_upload':
      return 'Form Check Upload Requires Pro';
    case 'coach_tier_elite':
      return 'Elite Coach Tier Required';
    case 'coach_tier_pro':
    default:
      return 'Coach Tier Upgrade Required';
  }
}

function paywallSubcopy(feature: MainTabParamList['Paywall']['feature']): string {
  switch (feature) {
    case 'coach_tier_elite':
      return 'Upgrade to Elite to unlock this coach session tier.';
    case 'coach_tier_pro':
      return 'Upgrade to Pro to access coach sessions above free tier.';
    default:
      return 'Choose a subscription plan and restore prior purchases on this device if needed.';
  }
}

function productPriceLabel(product: StoreSubscription | undefined): string {
  if (!product) {
    return '';
  }

  const withDisplayPrice = product as StoreSubscription & {
    displayPrice?: string | null;
    localizedPrice?: string | null;
  };
  if (withDisplayPrice.displayPrice) {
    return withDisplayPrice.displayPrice;
  }
  if (withDisplayPrice.localizedPrice) {
    return withDisplayPrice.localizedPrice;
  }
  return '';
}

export function PaywallScreen(): React.JSX.Element {
  const route = useRoute<PaywallRoute>();
  const navigation = useNavigation<PaywallNavigation>();
  const { session, refreshCurrentUser } = useAuth();
  const accessToken = session?.tokens.accessToken ?? '';
  const [isInitializing, setIsInitializing] = useState(true);
  const [subscriptions, setSubscriptions] = useState<StoreSubscription[]>([]);
  const [statusMessage, setStatusMessage] = useState<string | undefined>();
  const [errorMessage, setErrorMessage] = useState<string | undefined>();
  const [activeSku, setActiveSku] = useState<string | null>(null);
  const [isRestoring, setIsRestoring] = useState(false);

  useEffect(() => {
    let isMounted = true;

    async function loadStoreContext() {
      try {
        await initializeIAP();
        const products = await listSubscriptionProducts(SUBSCRIPTION_PRODUCT_IDS);
        if (isMounted) {
          setSubscriptions(products);
        }
      } catch (error) {
        if (isMounted) {
          setErrorMessage(error instanceof Error ? error.message : 'Unable to initialize billing.');
        }
      } finally {
        if (isMounted) {
          setIsInitializing(false);
        }
      }
    }

    loadStoreContext().catch(() => {
      setIsInitializing(false);
    });

    return () => {
      isMounted = false;
      closeIAPConnection().catch(() => {
        // Ignore disconnect failures during teardown.
      });
    };
  }, []);

  const storeProductsById = useMemo(() => {
    const map = new Map<string, StoreSubscription>();
    for (const product of subscriptions) {
      map.set(product.id, product);
    }
    return map;
  }, [subscriptions]);

  async function verifyStorePurchase(
    purchase: {
      productId: string;
      receiptToken: string;
      transactionId: string;
      originalTransactionId?: string;
      expiresAt?: string;
    },
    restore: boolean,
  ): Promise<void> {
    if (!accessToken) {
      throw new Error('You must be logged in to verify purchases.');
    }

    await verifyBillingReceipt({
      accessToken,
      platform: inferBillingPlatform(),
      productId: purchase.productId,
      receiptToken: purchase.receiptToken,
      transactionId: purchase.transactionId,
      originalTransactionId: purchase.originalTransactionId,
      expiresAt: purchase.expiresAt,
      restore,
    });
  }

  async function handlePurchase(sku: string): Promise<void> {
    setErrorMessage(undefined);
    setStatusMessage(undefined);
    setActiveSku(sku);

    try {
      const purchase = await purchaseSubscriptionBySku(sku);
      await verifyStorePurchase(
        {
          productId: purchase.productId,
          receiptToken: extractPurchaseReceiptToken(purchase),
          transactionId: extractPurchaseTransactionId(purchase),
          originalTransactionId: extractPurchaseOriginalTransactionId(purchase),
          expiresAt: extractPurchaseExpiryDate(purchase),
        },
        false,
      );
      await finalizeStorePurchase(purchase);
      await refreshCurrentUser();
      setStatusMessage('Subscription activated. Your entitlements are now updated.');
      navigation.goBack();
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : 'Unable to complete purchase.');
    } finally {
      setActiveSku(null);
    }
  }

  async function handleRestore(): Promise<void> {
    setErrorMessage(undefined);
    setStatusMessage(undefined);
    setIsRestoring(true);

    try {
      const purchases = await restoreAvailablePurchases();
      const subscriptionPurchases = purchases.filter(item =>
        SUBSCRIPTION_PRODUCT_IDS.includes(item.productId as (typeof SUBSCRIPTION_PRODUCT_IDS)[number]),
      );
      if (subscriptionPurchases.length === 0) {
        setStatusMessage('No restorable subscriptions were found for this account.');
        return;
      }

      let restoredCount = 0;
      for (const purchase of subscriptionPurchases) {
        await verifyStorePurchase(
          {
            productId: purchase.productId,
            receiptToken: extractPurchaseReceiptToken(purchase),
            transactionId: extractPurchaseTransactionId(purchase),
            originalTransactionId: extractPurchaseOriginalTransactionId(purchase),
            expiresAt: extractPurchaseExpiryDate(purchase),
          },
          true,
        );
        await finalizeStorePurchase(purchase);
        restoredCount += 1;
      }

      await refreshCurrentUser();
      setStatusMessage(
        restoredCount === 1
          ? 'Restored 1 subscription purchase.'
          : `Restored ${restoredCount} subscription purchases.`,
      );
      navigation.goBack();
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : 'Unable to restore purchases.');
    } finally {
      setIsRestoring(false);
    }
  }

  return (
    <ScrollView contentContainerStyle={styles.container} testID="paywall-screen">
      <Text style={styles.title}>{paywallHeading(route.params.feature)}</Text>
      <Text style={styles.subtitle}>{paywallSubcopy(route.params.feature)}</Text>

      {isInitializing ? (
        <View style={styles.loadingRow} testID="paywall-loading">
          <ActivityIndicator size="small" color="#0f766e" />
          <Text style={styles.helper}>Loading subscription plans...</Text>
        </View>
      ) : (
        SUBSCRIPTION_PRODUCT_IDS.map(productId => {
          const product = storeProductsById.get(productId);
          const copy = PLAN_COPY[productId] ?? {
            title: productId,
            details: 'Subscription plan',
          };
          const price = productPriceLabel(product);
          const priceLabel = price ? ` • ${price}` : '';

          return (
            <Card key={productId} testID={`paywall-plan-${productId}`}>
              <Text style={styles.planTitle}>
                {copy.title}
                {priceLabel}
              </Text>
              <Text style={styles.helper}>{copy.details}</Text>
              <Button
                label="Subscribe"
                onPress={() => {
                  handlePurchase(productId).catch(() => {
                    setErrorMessage('Unable to complete purchase.');
                  });
                }}
                loading={activeSku === productId}
                disabled={Boolean(activeSku) || isRestoring}
                testID={`paywall-subscribe-${productId}`}
              />
            </Card>
          );
        })
      )}

      <Card testID="paywall-restore-card">
        <Text style={styles.planTitle}>Restore Purchases</Text>
        <Text style={styles.helper}>Recover previous subscriptions linked to your app-store account.</Text>
        <Button
          label="Restore Purchases"
          variant="secondary"
          onPress={() => {
            handleRestore().catch(() => {
              setErrorMessage('Unable to restore purchases.');
            });
          }}
          loading={isRestoring}
          disabled={Boolean(activeSku)}
          testID="paywall-restore-button"
        />
      </Card>

      {statusMessage ? <Text style={styles.success}>{statusMessage}</Text> : null}
      {errorMessage ? <Text style={styles.error}>{errorMessage}</Text> : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: 16,
    gap: 12,
    backgroundColor: '#f8fafc',
  },
  loadingRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  title: {
    fontSize: 24,
    fontWeight: '700',
    color: '#0f172a',
  },
  subtitle: {
    color: '#475569',
    fontSize: 14,
  },
  planTitle: {
    fontSize: 18,
    fontWeight: '700',
    color: '#0f172a',
  },
  helper: {
    color: '#475569',
    fontSize: 13,
  },
  success: {
    color: '#047857',
    fontSize: 13,
  },
  error: {
    color: '#b91c1c',
    fontSize: 13,
  },
});
