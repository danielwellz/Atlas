import type { operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';

type BillingVerifyRequest = operations['PostBillingVerify']['requestBody']['content']['application/json'];
type BillingVerifyResponse = operations['PostBillingVerify']['responses'][200]['content']['application/json'];

export type VerifyBillingReceiptInput = BillingVerifyRequest & {
  accessToken: string;
};

export async function verifyBillingReceipt(
  input: VerifyBillingReceiptInput,
): Promise<BillingVerifyResponse> {
  const body: BillingVerifyRequest = {
    platform: input.platform,
    productId: input.productId,
    receiptToken: input.receiptToken,
    restore: input.restore ?? false,
  };

  if (input.transactionId) {
    body.transactionId = input.transactionId;
  }
  if (input.originalTransactionId) {
    body.originalTransactionId = input.originalTransactionId;
  }
  if (input.expiresAt) {
    body.expiresAt = input.expiresAt;
  }

  const response = await atlasApiClient.POST('/api/v1/billing/verify', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to verify purchase.'));
  }

  return response.data;
}
