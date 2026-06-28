import type { components, operations } from '../generated/openapi';
import { normalizeUTCDateKey } from '../dateKey';
import { atlasApiClient, getApiErrorMessage } from '../client';

type NutritionTodayResponse =
  operations['GetNutritionToday']['responses'][200]['content']['application/json'];
type NutritionCheckinRequest =
  operations['PostNutritionCheckin']['requestBody']['content']['application/json'];
type NutritionCheckinResponse =
  operations['PostNutritionCheckin']['responses'][200]['content']['application/json'];
type UpsertWeightEntryRequest =
  operations['PutNutritionWeightEntry']['requestBody']['content']['application/json'];
type UpsertWeightEntryResponse =
  operations['PutNutritionWeightEntry']['responses'][200]['content']['application/json'];
type NutritionWeightTrendResponse =
  operations['GetNutritionWeightTrend']['responses'][200]['content']['application/json'];
type NutritionWeeklyCheckinRequest =
  NonNullable<operations['PostNutritionWeeklyCheckin']['requestBody']>['content']['application/json'];
type NutritionWeeklyCheckinResponse =
  operations['PostNutritionWeeklyCheckin']['responses'][200]['content']['application/json'];
type NutritionMealPlanGenerateRequest =
  NonNullable<operations['PostNutritionMealPlanGenerate']['requestBody']>['content']['application/json'];
type NutritionMealPlanResponse =
  operations['PostNutritionMealPlanGenerate']['responses'][200]['content']['application/json'];
type NutritionMealPlanUpsertRequest =
  operations['PutNutritionMealPlan']['requestBody']['content']['application/json'];
type NutritionRecipeImportRequest =
  operations['PostNutritionRecipeImport']['requestBody']['content']['application/json'];
type NutritionRecipeImportResponse =
  operations['PostNutritionRecipeImport']['responses'][200]['content']['application/json'];

export type NutritionToday = components['schemas']['NutritionTodayResponse'];
export type NutritionDailyCheckin = components['schemas']['NutritionDailyCheckin'];
export type WeightUnit = components['schemas']['WeightUnit'];
export type NutritionWeightEntry = components['schemas']['NutritionWeightEntry'];
export type NutritionWeightTrendPoint = components['schemas']['NutritionWeightTrendPoint'];
export type WeeklyMacroTargets = components['schemas']['WeeklyMacroTargets'];
export type NutritionWeeklyCheckin = components['schemas']['NutritionWeeklyCheckin'];
export type NutritionMealPlan = components['schemas']['NutritionMealPlan'];
export type NutritionMealPlanItem = components['schemas']['NutritionMealPlanItem'];
export type NutritionGroceryItem = components['schemas']['NutritionGroceryItem'];
export type NutritionMealPlanEditableItem = components['schemas']['NutritionMealPlanEditableItem'];
export type NutritionRecipeImportDraft = components['schemas']['NutritionRecipeImportDraft'];
export type NutritionRecipeImportResult = components['schemas']['NutritionRecipeImportResponse'];

type NutritionServiceContext = {
  accessToken: string;
};

export type UpsertDailyNutritionCheckinInput = NutritionServiceContext & {
  dateKey: string;
  caloriesEstimate?: number | null;
  proteinGEstimate?: number | null;
  notes?: string;
};

export type UpsertWeightEntryInput = NutritionServiceContext & {
  dateKey?: string;
  weight: number;
  unit: WeightUnit;
};

export type RunWeeklyNutritionCheckinInput = NutritionServiceContext & {
  weekStartDateKey?: string;
};

export type GenerateMealPlanInput = NutritionServiceContext & {
  weekStartDateKey?: string;
};

export type GetMealPlanByWeekInput = NutritionServiceContext & {
  weekStartDateKey: string;
};

export type UpsertMealPlanInput = NutritionServiceContext & {
  weekStartDateKey: string;
  items: Array<{
    dayOfWeek: number;
    mealSlot: string;
    recipeId: string;
    servings: number;
  }>;
};

export type DeleteMealPlanByWeekInput = NutritionServiceContext & {
  weekStartDateKey: string;
};

export type ImportNutritionRecipeInput = NutritionServiceContext & {
  sourceUrl: string;
  confirm?: boolean;
  draft?: NutritionRecipeImportDraft;
};

function assertAccessToken(accessToken: string): void {
  if (!accessToken) {
    throw new Error('Missing authentication token.');
  }
}

export async function getNutritionTargets(input: NutritionServiceContext): Promise<NutritionToday> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/nutrition/today', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load nutrition status.'));
  }

  const payload: NutritionTodayResponse = response.data;
  return payload;
}

export async function upsertDailyNutritionCheckin(
  input: UpsertDailyNutritionCheckinInput,
): Promise<NutritionDailyCheckin> {
  assertAccessToken(input.accessToken);

  const body: NutritionCheckinRequest = {
    date: normalizeUTCDateKey(input.dateKey),
  };

  if (input.caloriesEstimate !== undefined) {
    body.calories_estimate = input.caloriesEstimate;
  }
  if (input.proteinGEstimate !== undefined) {
    body.protein_g_estimate = input.proteinGEstimate;
  }
  if (input.notes !== undefined) {
    body.notes = input.notes;
  }

  const response = await atlasApiClient.POST('/api/v1/nutrition/checkin', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to submit nutrition check-in.'));
  }

  const payload: NutritionCheckinResponse = response.data;
  return payload.checkin;
}

export async function upsertNutritionWeightEntry(
  input: UpsertWeightEntryInput,
): Promise<NutritionWeightEntry> {
  assertAccessToken(input.accessToken);

  const body: UpsertWeightEntryRequest = {
    date: normalizeUTCDateKey(input.dateKey),
    weight: input.weight,
    unit: input.unit,
  };

  const response = await atlasApiClient.PUT('/api/v1/nutrition/weight', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to submit weight entry.'));
  }

  const payload: UpsertWeightEntryResponse = response.data;
  return payload.entry;
}

export async function getNutritionWeightTrend(
  input: NutritionServiceContext,
): Promise<NutritionWeightTrendPoint[]> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/nutrition/weight/trend', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load weight trend.'));
  }

  const payload: NutritionWeightTrendResponse = response.data;
  return payload.points;
}

export async function runWeeklyNutritionCheckin(
  input: RunWeeklyNutritionCheckinInput,
): Promise<NutritionWeeklyCheckin> {
  assertAccessToken(input.accessToken);

  const body: NutritionWeeklyCheckinRequest = {};
  if (input.weekStartDateKey) {
    body.week_start = normalizeUTCDateKey(input.weekStartDateKey);
  }

  const response = await atlasApiClient.POST('/api/v1/nutrition/weekly-checkin', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to run weekly check-in.'));
  }

  const payload: NutritionWeeklyCheckinResponse = response.data;
  return payload.checkin;
}

export async function getLatestNutritionWeeklyCheckin(
  input: NutritionServiceContext,
): Promise<NutritionWeeklyCheckin> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/nutrition/weekly-checkin/latest', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load weekly check-in.'));
  }

  const payload: NutritionWeeklyCheckinResponse = response.data;
  return payload.checkin;
}

export async function generateNutritionMealPlan(input: GenerateMealPlanInput): Promise<NutritionMealPlan> {
  assertAccessToken(input.accessToken);

  const body: NutritionMealPlanGenerateRequest = {};
  if (input.weekStartDateKey) {
    body.week_start = normalizeUTCDateKey(input.weekStartDateKey);
  }

  const response = await atlasApiClient.POST('/api/v1/nutrition/meal-plan/generate', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to generate meal plan.'));
  }

  const payload: NutritionMealPlanResponse = response.data;
  return payload.meal_plan;
}

export async function getLatestNutritionMealPlan(
  input: NutritionServiceContext,
): Promise<NutritionMealPlan> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/nutrition/meal-plan/latest', {
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load meal plan.'));
  }

  const payload: NutritionMealPlanResponse = response.data;
  return payload.meal_plan;
}

export async function getNutritionMealPlanByWeek(
  input: GetMealPlanByWeekInput,
): Promise<NutritionMealPlan> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.GET('/api/v1/nutrition/meal-plan', {
    params: {
      query: {
        week_start: normalizeUTCDateKey(input.weekStartDateKey),
      },
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to load meal plan.'));
  }

  const payload: NutritionMealPlanResponse = response.data;
  return payload.meal_plan;
}

export async function upsertNutritionMealPlan(input: UpsertMealPlanInput): Promise<NutritionMealPlan> {
  assertAccessToken(input.accessToken);

  const body: NutritionMealPlanUpsertRequest = {
    week_start: normalizeUTCDateKey(input.weekStartDateKey),
    items: input.items.map(item => ({
      day_of_week: item.dayOfWeek,
      meal_slot: item.mealSlot,
      recipe_id: item.recipeId,
      servings: item.servings,
    })),
  };

  const response = await atlasApiClient.PUT('/api/v1/nutrition/meal-plan', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to save meal plan.'));
  }

  const payload: NutritionMealPlanResponse = response.data;
  return payload.meal_plan;
}

export async function deleteNutritionMealPlanByWeek(input: DeleteMealPlanByWeekInput): Promise<void> {
  assertAccessToken(input.accessToken);

  const response = await atlasApiClient.DELETE('/api/v1/nutrition/meal-plan', {
    params: {
      query: {
        week_start: normalizeUTCDateKey(input.weekStartDateKey),
      },
    },
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (response.error) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to delete meal plan.'));
  }
}

export async function importNutritionRecipe(
  input: ImportNutritionRecipeInput,
): Promise<NutritionRecipeImportResult> {
  assertAccessToken(input.accessToken);

  const body: NutritionRecipeImportRequest = {
    source_url: input.sourceUrl,
  };
  if (input.confirm !== undefined) {
    body.confirm = input.confirm;
  }
  if (input.draft !== undefined) {
    body.draft = input.draft;
  }

  const response = await atlasApiClient.POST('/api/v1/nutrition/recipes/import', {
    body,
    headers: {
      Authorization: `Bearer ${input.accessToken}`,
    },
  });

  if (!response.data) {
    throw new Error(getApiErrorMessage(response.error, 'Unable to import recipe.'));
  }

  const payload: NutritionRecipeImportResponse = response.data;
  return payload;
}
