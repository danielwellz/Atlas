package food

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	defaultSearchLimit         = 20
	maxSearchLimit             = 50
	defaultProviderTimeout     = 6 * time.Second
	defaultSearchResponseTTL   = 15 * time.Minute
	defaultUPCResponseCacheTTL = 24 * time.Hour
)

var ErrFoodNotFound = errors.New("food not found")

type Service struct {
	logger                 *zap.Logger
	queries                db.Querier
	providersByName        map[string]Provider
	providerOrder          []string
	defaultProvider        string
	upcProviders           []UPCProvider
	providerRequestTimeout time.Duration
	searchCacheTTL         time.Duration
	upcCacheTTL            time.Duration
	searchCacheLock        sync.RWMutex
	searchCache            map[string]cachedSearchResponse
	upcCacheLock           sync.RWMutex
	upcCache               map[string]cachedUPCResponse
	now                    func() time.Time
}

type cachedSearchResponse struct {
	foods     []db.Food
	expiresAt time.Time
}

type cachedUPCResponse struct {
	food      db.Food
	expiresAt time.Time
}

type LogInput struct {
	FoodID   uuid.UUID
	Datetime time.Time
	Quantity float64
	Unit     string
}

type LoggedFood struct {
	Log            db.FoodLog
	Food           db.Food
	NutrientValues Nutrients
}

type DailyTotals struct {
	CaloriesKcal float64
	ProteinG     float64
	CarbsG       float64
	FatG         float64
}

func NewService(
	logger *zap.Logger,
	queries db.Querier,
	defaultProvider string,
	upcProvider UPCProvider,
	providers ...Provider,
) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	providersByName := make(map[string]Provider, len(providers))
	providerOrder := make([]string, 0, len(providers))
	upcProviders := make([]UPCProvider, 0, len(providers)+1)
	seenUPCProviders := map[string]struct{}{}

	appendUPCProvider := func(provider UPCProvider) {
		if provider == nil {
			return
		}
		name := strings.ToLower(strings.TrimSpace(provider.Name()))
		if name == "" {
			return
		}
		if _, exists := seenUPCProviders[name]; exists {
			return
		}
		seenUPCProviders[name] = struct{}{}
		upcProviders = append(upcProviders, provider)
	}

	appendUPCProvider(upcProvider)

	for _, provider := range providers {
		if provider == nil {
			continue
		}
		normalizedName := strings.ToLower(strings.TrimSpace(provider.Name()))
		if normalizedName == "" {
			continue
		}
		providersByName[normalizedName] = provider
		providerOrder = append(providerOrder, normalizedName)
		if upcProviderCandidate, ok := provider.(UPCProvider); ok {
			appendUPCProvider(upcProviderCandidate)
		}
	}

	return &Service{
		logger:                 logger.Named("food"),
		queries:                queries,
		providersByName:        providersByName,
		providerOrder:          providerOrder,
		defaultProvider:        strings.ToLower(strings.TrimSpace(defaultProvider)),
		upcProviders:           upcProviders,
		providerRequestTimeout: defaultProviderTimeout,
		searchCacheTTL:         defaultSearchResponseTTL,
		upcCacheTTL:            defaultUPCResponseCacheTTL,
		searchCache:            map[string]cachedSearchResponse{},
		upcCache:               map[string]cachedUPCResponse{},
		now:                    time.Now,
	}
}

func (s *Service) SearchFoods(ctx context.Context, query string, limit int) ([]db.Food, error) {
	normalizedQuery := strings.TrimSpace(query)
	if normalizedQuery == "" {
		return nil, fmt.Errorf("query must not be empty")
	}

	normalizedLimit := normalizeSearchLimit(limit)
	if cachedFoods, ok := s.getCachedSearchResponse(normalizedQuery, normalizedLimit); ok {
		return cachedFoods, nil
	}

	providers, err := s.searchProvidersInPriorityOrder()
	if err != nil {
		return nil, err
	}

	var attemptErrors []error
	for _, provider := range providers {
		results, providerErr := s.searchWithTimeout(ctx, provider, normalizedQuery, normalizedLimit)
		if providerErr != nil {
			attemptErrors = append(attemptErrors, fmt.Errorf("%s: %w", provider.Name(), providerErr))
			continue
		}

		foods := make([]db.Food, 0, len(results))
		for _, result := range results {
			row, err := s.upsertFood(
				ctx,
				provider.Name(),
				result.ExternalID,
				result.Label,
				result.Brand,
				result.Nutrients,
			)
			if err != nil {
				return nil, err
			}
			foods = append(foods, row)
		}

		s.setCachedSearchResponse(normalizedQuery, normalizedLimit, foods)
		return foods, nil
	}

	if cachedFoods, ok := s.getCachedSearchResponse(normalizedQuery, normalizedLimit); ok {
		s.logger.Warn(
			"search providers failed, serving cached results",
			zap.String("query", normalizedQuery),
			zap.Int("limit", normalizedLimit),
		)
		return cachedFoods, nil
	}

	if len(attemptErrors) > 0 {
		return nil, fmt.Errorf("food search failed: %w", errors.Join(attemptErrors...))
	}

	return nil, fmt.Errorf("food search provider is not configured")
}

func (s *Service) GetFood(ctx context.Context, foodID uuid.UUID) (db.Food, error) {
	row, err := s.queries.GetFoodByID(ctx, foodID)
	if err != nil {
		return db.Food{}, err
	}

	provider, err := s.resolveProvider(row.Provider)
	if err != nil {
		return row, nil
	}

	detail, err := s.getDetailsWithTimeout(ctx, provider, row.ExternalID)
	if err != nil {
		s.logger.Warn(
			"failed refreshing food details from provider",
			zap.Error(err),
			zap.String("food_id", row.ID.String()),
			zap.String("provider", row.Provider),
			zap.String("external_id", row.ExternalID),
		)
		return row, nil
	}

	updatedRow, err := s.upsertFood(ctx, provider.Name(), detail.ExternalID, detail.Label, detail.Brand, detail.Nutrients)
	if err != nil {
		return db.Food{}, err
	}
	return updatedRow, nil
}

func (s *Service) LookupFoodByUPC(ctx context.Context, code string) (db.Food, error) {
	normalizedCode, err := NormalizeUPC(code)
	if err != nil {
		return db.Food{}, err
	}
	if len(s.upcProviders) == 0 {
		return db.Food{}, fmt.Errorf("upc provider is not configured")
	}

	if cachedFood, ok := s.getCachedUPCResponse(normalizedCode); ok {
		return cachedFood, nil
	}

	var attemptErrors []error
	seenNotFound := false

	for _, provider := range s.upcProviders {
		detail, providerErr := s.lookupUPCWithTimeout(ctx, provider, normalizedCode)
		if providerErr != nil {
			if errors.Is(providerErr, ErrFoodNotFound) {
				seenNotFound = true
				continue
			}
			attemptErrors = append(attemptErrors, fmt.Errorf("%s: %w", provider.Name(), providerErr))
			continue
		}

		externalID := strings.TrimSpace(detail.ExternalID)
		if externalID == "" {
			externalID = normalizedCode
		}

		row, err := s.upsertFood(
			ctx,
			provider.Name(),
			externalID,
			detail.Label,
			detail.Brand,
			detail.Nutrients,
		)
		if err != nil {
			return db.Food{}, err
		}

		s.setCachedUPCResponse(normalizedCode, row)
		return row, nil
	}

	if cachedFood, ok := s.getCachedUPCResponse(normalizedCode); ok {
		s.logger.Warn("upc providers failed, serving cached result", zap.String("code", normalizedCode))
		return cachedFood, nil
	}
	if seenNotFound {
		return db.Food{}, ErrFoodNotFound
	}
	if len(attemptErrors) > 0 {
		return db.Food{}, fmt.Errorf("food upc lookup failed: %w", errors.Join(attemptErrors...))
	}
	return db.Food{}, ErrFoodNotFound
}

func (s *Service) LogFood(ctx context.Context, userID uuid.UUID, input LogInput) (db.FoodLog, db.Food, Nutrients, error) {
	if input.Quantity <= 0 {
		return db.FoodLog{}, db.Food{}, Nutrients{}, fmt.Errorf("quantity must be greater than 0")
	}

	loggedAt := input.Datetime.UTC()
	if loggedAt.IsZero() {
		loggedAt = s.now().UTC()
	}

	unit := strings.TrimSpace(input.Unit)
	if unit == "" {
		unit = "serving"
	}

	foodRow, err := s.GetFood(ctx, input.FoodID)
	if err != nil {
		return db.FoodLog{}, db.Food{}, Nutrients{}, err
	}

	nutrients, err := ParseNutrientsJSON(foodRow.NutrientsJson)
	if err != nil {
		return db.FoodLog{}, db.Food{}, Nutrients{}, err
	}

	snapshot := nutrients.Scaled(input.Quantity)
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return db.FoodLog{}, db.Food{}, Nutrients{}, fmt.Errorf("marshal nutrients snapshot: %w", err)
	}

	logRow, err := s.queries.CreateFoodLog(ctx, db.CreateFoodLogParams{
		UserID:                userID,
		Datetime:              loggedAt,
		FoodID:                input.FoodID,
		Quantity:              input.Quantity,
		Unit:                  unit,
		NutrientsSnapshotJson: snapshotJSON,
	})
	if err != nil {
		return db.FoodLog{}, db.Food{}, Nutrients{}, err
	}

	return logRow, foodRow, snapshot, nil
}

func (s *Service) ListFoodLogsByDate(ctx context.Context, userID uuid.UUID, date time.Time) ([]LoggedFood, DailyTotals, error) {
	targetDate := time.Date(date.UTC().Year(), date.UTC().Month(), date.UTC().Day(), 0, 0, 0, 0, time.UTC)

	rows, err := s.queries.ListFoodLogsByUserIDAndDate(ctx, db.ListFoodLogsByUserIDAndDateParams{
		UserID: userID,
		Date:   targetDate,
	})
	if err != nil {
		return nil, DailyTotals{}, err
	}

	logs := make([]LoggedFood, 0, len(rows))
	for _, row := range rows {
		snapshot, err := ParseNutrientsJSON(row.NutrientsSnapshotJson)
		if err != nil {
			return nil, DailyTotals{}, err
		}

		logs = append(logs, LoggedFood{
			Log: db.FoodLog{
				ID:                    row.ID,
				UserID:                row.UserID,
				Datetime:              row.Datetime,
				FoodID:                row.FoodID,
				Quantity:              row.Quantity,
				Unit:                  row.Unit,
				NutrientsSnapshotJson: row.NutrientsSnapshotJson,
				CreatedAt:             row.CreatedAt,
			},
			Food: db.Food{
				ID:            row.FoodID,
				ExternalID:    row.ExternalID,
				Provider:      row.Provider,
				Label:         row.Label,
				Brand:         row.Brand,
				NutrientsJson: row.NutrientsJson,
				CreatedAt:     row.CreatedAt_2,
				UpdatedAt:     row.UpdatedAt,
			},
			NutrientValues: snapshot,
		})
	}

	totalsRow, err := s.queries.GetFoodLogDailyTotalsByUserIDAndDate(ctx, db.GetFoodLogDailyTotalsByUserIDAndDateParams{
		UserID: userID,
		Date:   targetDate,
	})
	if err != nil {
		return nil, DailyTotals{}, err
	}

	return logs, DailyTotals{
		CaloriesKcal: totalsRow.CaloriesKcal,
		ProteinG:     totalsRow.ProteinG,
		CarbsG:       totalsRow.CarbsG,
		FatG:         totalsRow.FatG,
	}, nil
}

func (s *Service) DailyTotalsByDate(ctx context.Context, userID uuid.UUID, date time.Time) (DailyTotals, error) {
	targetDate := time.Date(date.UTC().Year(), date.UTC().Month(), date.UTC().Day(), 0, 0, 0, 0, time.UTC)
	row, err := s.queries.GetFoodLogDailyTotalsByUserIDAndDate(ctx, db.GetFoodLogDailyTotalsByUserIDAndDateParams{
		UserID: userID,
		Date:   targetDate,
	})
	if err != nil {
		return DailyTotals{}, err
	}
	return DailyTotals{
		CaloriesKcal: row.CaloriesKcal,
		ProteinG:     row.ProteinG,
		CarbsG:       row.CarbsG,
		FatG:         row.FatG,
	}, nil
}

func (s *Service) upsertFood(
	ctx context.Context,
	provider string,
	externalID string,
	label string,
	brand string,
	nutrients Nutrients,
) (db.Food, error) {
	nutrientsJSON, err := json.Marshal(nutrients)
	if err != nil {
		return db.Food{}, fmt.Errorf("marshal nutrients json: %w", err)
	}

	row, err := s.queries.UpsertFood(ctx, db.UpsertFoodParams{
		ExternalID:    strings.TrimSpace(externalID),
		Provider:      strings.ToLower(strings.TrimSpace(provider)),
		Label:         strings.TrimSpace(label),
		Brand:         strings.TrimSpace(brand),
		NutrientsJson: nutrientsJSON,
	})
	if err != nil {
		return db.Food{}, err
	}
	return row, nil
}

func (s *Service) resolveProvider(providerName string) (Provider, error) {
	normalized := strings.ToLower(strings.TrimSpace(providerName))
	if normalized == "" {
		return nil, fmt.Errorf("provider is not configured")
	}

	provider, ok := s.providersByName[normalized]
	if !ok || provider == nil {
		return nil, fmt.Errorf("provider %q is not registered", normalized)
	}
	return provider, nil
}

func (s *Service) searchProvidersInPriorityOrder() ([]Provider, error) {
	if len(s.providersByName) == 0 {
		return nil, fmt.Errorf("provider is not configured")
	}

	providers := make([]Provider, 0, len(s.providersByName))
	seen := map[string]struct{}{}

	if s.defaultProvider != "" {
		defaultProvider, err := s.resolveProvider(s.defaultProvider)
		if err != nil {
			s.logger.Warn("default food provider is unavailable", zap.String("provider", s.defaultProvider), zap.Error(err))
		} else {
			providers = append(providers, defaultProvider)
			seen[s.defaultProvider] = struct{}{}
		}
	}

	for _, providerName := range s.providerOrder {
		normalizedProviderName := strings.ToLower(strings.TrimSpace(providerName))
		if normalizedProviderName == "" {
			continue
		}
		if _, alreadyIncluded := seen[normalizedProviderName]; alreadyIncluded {
			continue
		}
		provider, ok := s.providersByName[normalizedProviderName]
		if !ok || provider == nil {
			continue
		}
		providers = append(providers, provider)
		seen[normalizedProviderName] = struct{}{}
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("provider is not configured")
	}

	return providers, nil
}

func (s *Service) searchWithTimeout(ctx context.Context, provider Provider, query string, limit int) ([]SearchResult, error) {
	if s.providerRequestTimeout <= 0 {
		return provider.Search(ctx, query, limit)
	}
	requestCtx, cancel := context.WithTimeout(ctx, s.providerRequestTimeout)
	defer cancel()
	return provider.Search(requestCtx, query, limit)
}

func (s *Service) getDetailsWithTimeout(ctx context.Context, provider Provider, externalID string) (Detail, error) {
	if s.providerRequestTimeout <= 0 {
		return provider.GetDetails(ctx, externalID)
	}
	requestCtx, cancel := context.WithTimeout(ctx, s.providerRequestTimeout)
	defer cancel()
	return provider.GetDetails(requestCtx, externalID)
}

func (s *Service) lookupUPCWithTimeout(ctx context.Context, provider UPCProvider, code string) (Detail, error) {
	if s.providerRequestTimeout <= 0 {
		return provider.LookupUPC(ctx, code)
	}
	requestCtx, cancel := context.WithTimeout(ctx, s.providerRequestTimeout)
	defer cancel()
	return provider.LookupUPC(requestCtx, code)
}

func (s *Service) getCachedSearchResponse(query string, limit int) ([]db.Food, bool) {
	if s.searchCacheTTL <= 0 {
		return nil, false
	}
	cacheKey := searchCacheKey(query, limit)
	now := s.now()

	s.searchCacheLock.RLock()
	cached, ok := s.searchCache[cacheKey]
	s.searchCacheLock.RUnlock()
	if !ok {
		return nil, false
	}
	if now.After(cached.expiresAt) {
		s.searchCacheLock.Lock()
		delete(s.searchCache, cacheKey)
		s.searchCacheLock.Unlock()
		return nil, false
	}

	cloned := make([]db.Food, len(cached.foods))
	copy(cloned, cached.foods)
	return cloned, true
}

func (s *Service) setCachedSearchResponse(query string, limit int, foods []db.Food) {
	if s.searchCacheTTL <= 0 {
		return
	}
	cacheKey := searchCacheKey(query, limit)
	cloned := make([]db.Food, len(foods))
	copy(cloned, foods)

	s.searchCacheLock.Lock()
	s.searchCache[cacheKey] = cachedSearchResponse{
		foods:     cloned,
		expiresAt: s.now().Add(s.searchCacheTTL),
	}
	s.searchCacheLock.Unlock()
}

func (s *Service) getCachedUPCResponse(code string) (db.Food, bool) {
	if s.upcCacheTTL <= 0 {
		return db.Food{}, false
	}
	now := s.now()

	s.upcCacheLock.RLock()
	cached, ok := s.upcCache[code]
	s.upcCacheLock.RUnlock()
	if !ok {
		return db.Food{}, false
	}
	if now.After(cached.expiresAt) {
		s.upcCacheLock.Lock()
		delete(s.upcCache, code)
		s.upcCacheLock.Unlock()
		return db.Food{}, false
	}
	return cached.food, true
}

func (s *Service) setCachedUPCResponse(code string, food db.Food) {
	if s.upcCacheTTL <= 0 {
		return
	}
	s.upcCacheLock.Lock()
	s.upcCache[code] = cachedUPCResponse{
		food:      food,
		expiresAt: s.now().Add(s.upcCacheTTL),
	}
	s.upcCacheLock.Unlock()
}

func normalizeSearchLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchLimit
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

func searchCacheKey(query string, limit int) string {
	return strings.ToLower(strings.TrimSpace(query)) + "|" + strconv.Itoa(limit)
}
