import type { operations } from '../api/generated/openapi';

type PostEventRequestBody = operations['PostEvents']['requestBody']['content']['application/json'];

export type ProductEventName = PostEventRequestBody['eventName'];

export type ProductEventProperties = NonNullable<PostEventRequestBody['properties']>;

export type QueueableProductEvent = {
  eventName: ProductEventName;
  eventTime: string;
  consentGranted: boolean;
  properties?: ProductEventProperties;
};
