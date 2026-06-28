import { Platform } from 'react-native';
import {
  endConnection,
  fetchProducts,
  finishTransaction,
  getAvailablePurchases,
  initConnection,
  requestPurchase,
  type Product,
  type ProductSubscription,
  type Purchase,
} from 'react-native-iap';

export const SUBSCRIPTION_PRODUCT_IDS = [
  'atlas.pro.monthly',
  'atlas.pro.yearly',
  'atlas.elite.monthly',
  'atlas.elite.yearly',
] as const;

export type StoreSubscription = Product | ProductSubscription;
export type StorePurchase = Purchase;

export async function initializeIAP(): Promise<void> {
  await initConnection();
}

export async function closeIAPConnection(): Promise<void> {
  await endConnection();
}

export async function listSubscriptionProducts(
  skus: readonly string[] = SUBSCRIPTION_PRODUCT_IDS,
): Promise<StoreSubscription[]> {
  const products = (await fetchProducts({
    skus: [...skus],
    type: 'subs',
  })) ?? [];
  return products as StoreSubscription[];
}

export async function purchaseSubscriptionBySku(sku: string): Promise<StorePurchase> {
  const result = await requestPurchase({
    type: 'subs',
    request: {
      apple: {
        sku,
      },
      google: {
        skus: [sku],
      },
    },
  });

  const purchases = Array.isArray(result) ? result : result ? [result] : [];
  const matched = purchases.find(item => item.productId === sku) ?? purchases[0];
  if (!matched) {
    throw new Error('Store purchase did not return a transaction.');
  }
  return matched;
}

export async function restoreAvailablePurchases(): Promise<StorePurchase[]> {
  return getAvailablePurchases();
}

export async function finalizeStorePurchase(purchase: StorePurchase): Promise<void> {
  await finishTransaction({
    purchase,
    isConsumable: false,
  });
}

export function inferBillingPlatform(): 'ios' | 'android' {
  return Platform.OS === 'ios' ? 'ios' : 'android';
}

export function extractPurchaseReceiptToken(purchase: StorePurchase): string {
  const purchaseToken = purchase.purchaseToken?.trim();
  if (purchaseToken) {
    return purchaseToken;
  }
  return purchase.id;
}

export function extractPurchaseTransactionId(purchase: StorePurchase): string {
  const typedPurchase = purchase as StorePurchase & {
    transactionId?: string | null;
  };
  const transactionId = typedPurchase.transactionId?.trim();
  if (transactionId) {
    return transactionId;
  }
  return purchase.id;
}

export function extractPurchaseOriginalTransactionId(
  purchase: StorePurchase,
): string | undefined {
  const typedPurchase = purchase as StorePurchase & {
    originalTransactionIdentifierIOS?: string | null;
  };
  const value = typedPurchase.originalTransactionIdentifierIOS?.trim();
  return value ? value : undefined;
}

export function extractPurchaseExpiryDate(purchase: StorePurchase): string | undefined {
  const typedPurchase = purchase as StorePurchase & {
    expirationDateIOS?: number | null;
  };
  if (typedPurchase.expirationDateIOS && Number.isFinite(typedPurchase.expirationDateIOS)) {
    return new Date(typedPurchase.expirationDateIOS).toISOString();
  }
  return undefined;
}
