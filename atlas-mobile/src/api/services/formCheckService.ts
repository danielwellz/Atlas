import type { components, operations } from '../generated/openapi';
import { atlasApiClient, getApiErrorMessage } from '../client';

type CreateFormCheckUploadRequest =
  operations['PostFormCheckUploads']['requestBody']['content']['application/json'];
type CreateFormCheckUploadResponse =
  operations['PostFormCheckUploads']['responses'][201]['content']['application/json'];

export type FormCheckMovementType = components['schemas']['FormCheckMovementType'];
export type FormCheckResultSummary = components['schemas']['FormCheckResultSummary'];
export type FormCheckUpload = components['schemas']['FormCheckUpload'];

type FormCheckContext = {
  accessToken: string;
};

export type UploadFormCheckInput = FormCheckContext & {
  movementType: FormCheckMovementType;
  recordingStartedAt: string;
  recordingEndedAt: string;
  summary: FormCheckResultSummary;
  storageKey?: string;
  metadataJson?: Record<string, unknown>;
};

function assertAccessToken(accessToken: string): void {
  if (!accessToken) {
    throw new Error('Missing authentication token.');
  }
}

export async function uploadFormCheckResult(input: UploadFormCheckInput): Promise<FormCheckUpload> {
  assertAccessToken(input.accessToken);

  const body: CreateFormCheckUploadRequest = {
    movementType: input.movementType,
    recordingStartedAt: input.recordingStartedAt,
    recordingEndedAt: input.recordingEndedAt,
    summary: input.summary,
    metadataJson: input.metadataJson,
  };

  if (input.storageKey) {
    body.storageKey = input.storageKey;
  }

  const response = await atlasApiClient.POST('/api/v1/form-check/uploads', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to upload form-check clip.'));
  }

  const payload: CreateFormCheckUploadResponse = response.data;
  return payload.upload;
}
