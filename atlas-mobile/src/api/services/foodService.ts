import type { components, operations } from '../generated/openapi';
import { normalizeUTCDateKey } from '../dateKey';
import { atlasApiClient, getApiErrorMessage } from '../client';

type FoodSearchResponse =
  operations['GetFoodsSearch']['responses'][200]['content']['application/json'];
type FoodDetailResponse =
  operations['GetFoodById']['responses'][200]['content']['application/json'];
type FoodUPCResponse =
  operations['GetFoodsUpcCode']['responses'][200]['content']['application/json'];
type CreateFoodLogRequest =
  operations['PostFoodLogs']['requestBody']['content']['application/json'];
type CreateFoodLogResponse =
  operations['PostFoodLogs']['responses'][201]['content']['application/json'];
type FoodLogsResponse =
  operations['GetFoodLogs']['responses'][200]['content']['application/json'];

export type Food = components['schemas']['Food'];
export type FoodLog = components['schemas']['FoodLog'];
export type NutrientValues = components['schemas']['NutrientValues'];
export type MacroNutrientTotals = components['schemas']['MacroNutrientTotals'];
export type FoodLogsResult = FoodLogsResponse;

type FoodServiceContext = {
  accessToken: string;
};

export type SearchFoodsInput = FoodServiceContext & {
  query: string;
  limit?: number;
};

export type GetFoodByIDInput = FoodServiceContext & {
  foodId: string;
};

export type LookupFoodByUPCInput = FoodServiceContext & {
  code: string;
};

export type CreateFoodLogInput = FoodServiceContext & {
  foodId: string;
  quantity: number;
  unit?: string;
  datetime?: string;
};

export type ListFoodLogsInput = FoodServiceContext & {
  dateKey?: string;
};

function assertAccessToken(accessToken: string): void {
  if (!accessToken) {
    throw new Error('Missing authentication token.');
  }
}

export async function searchFoods(input: SearchFoodsInput): Promise<Food[]> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/foods/search', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
    params: {
      query: {
        q: input.query,
        limit: input.limit,
      },
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to search foods.'));
  }

  const payload: FoodSearchResponse = response.data;
  return payload.foods;
}

export async function getFoodById(input: GetFoodByIDInput): Promise<Food> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/foods/{id}', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
    params: {
      path: {
        id: input.foodId,
      },
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load food details.'));
  }

  const payload: FoodDetailResponse = response.data;
  return payload.food;
}

export async function lookupFoodByUpc(input: LookupFoodByUPCInput): Promise<Food> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/foods/upc/{code}', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
    params: {
      path: {
        code: input.code,
      },
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to lookup barcode.'));
  }

  const payload: FoodUPCResponse = response.data;
  return payload.food;
}

export async function createFoodLog(input: CreateFoodLogInput): Promise<FoodLog> {
  assertAccessToken(input.accessToken);

  const body: CreateFoodLogRequest = {
    foodId: input.foodId,
    quantity: input.quantity,
  };
  if (input.unit) {
    body.unit = input.unit;
  }
  if (input.datetime) {
    body.datetime = input.datetime;
  }

  const response = await atlasApiClient.POST('/api/v1/food-logs', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to log food intake.'));
  }

  const payload: CreateFoodLogResponse = response.data;
  return payload.log;
}

export async function listFoodLogs(input: ListFoodLogsInput): Promise<FoodLogsResponse> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/food-logs', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
    params: {
      query: {
        date: normalizeUTCDateKey(input.dateKey),
      },
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load food logs.'));
  }

  const payload: FoodLogsResponse = response.data;
  return payload;
}
