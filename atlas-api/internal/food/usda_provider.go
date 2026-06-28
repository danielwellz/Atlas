package food

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultUSDAHTTPTimeout = 10 * time.Second

	nutrientCaloriesKcalID = int64(1008)
	nutrientProteinGID     = int64(1003)
	nutrientCarbsGID       = int64(1005)
	nutrientFatGID         = int64(1004)
)

type USDAProvider struct {
	baseURL     string
	apiKey      string
	client      *http.Client
	detailsTTL  time.Duration
	detailsLock sync.RWMutex
	detailsByID map[string]cachedUSDAFoodDetail
}

type cachedUSDAFoodDetail struct {
	detail    Detail
	expiresAt time.Time
}

type usdaSearchRequest struct {
	Query    string `json:"query"`
	PageSize int    `json:"pageSize"`
}

type usdaSearchResponse struct {
	Foods []usdaFoodRecord `json:"foods"`
}

type usdaFoodRecord struct {
	FdcID          int64              `json:"fdcId"`
	Description    string             `json:"description"`
	BrandName      string             `json:"brandName"`
	BrandOwner     string             `json:"brandOwner"`
	FoodNutrients  []usdaNutrient     `json:"foodNutrients"`
	LabelNutrients usdaLabelNutrients `json:"labelNutrients"`
}

type usdaNutrient struct {
	NutrientID int64             `json:"nutrientId"`
	Value      *float64          `json:"value"`
	Amount     *float64          `json:"amount"`
	Nutrient   *usdaNutrientInfo `json:"nutrient"`
}

type usdaNutrientInfo struct {
	ID int64 `json:"id"`
}

type usdaLabelNutrients struct {
	Calories usdaLabelValue `json:"calories"`
	Protein  usdaLabelValue `json:"protein"`
	Carbs    usdaLabelValue `json:"carbohydrates"`
	Fat      usdaLabelValue `json:"fat"`
}

type usdaLabelValue struct {
	Value *float64 `json:"value"`
}

func NewUSDAProvider(baseURL string, apiKey string, detailsTTL time.Duration) *USDAProvider {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if normalizedBaseURL == "" {
		normalizedBaseURL = "https://api.nal.usda.gov/fdc/v1"
	}

	return &USDAProvider{
		baseURL:     normalizedBaseURL,
		apiKey:      strings.TrimSpace(apiKey),
		client:      &http.Client{Timeout: defaultUSDAHTTPTimeout},
		detailsTTL:  detailsTTL,
		detailsByID: map[string]cachedUSDAFoodDetail{},
	}
}

func (p *USDAProvider) Name() string {
	return USDAProviderName
}

func (p *USDAProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return []SearchResult{}, nil
	}
	if strings.TrimSpace(p.apiKey) == "" {
		return nil, fmt.Errorf("usda api key is required")
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}

	requestPayload, err := json.Marshal(usdaSearchRequest{
		Query:    query,
		PageSize: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal usda search request: %w", err)
	}

	endpoint, err := p.withAPIKey("/foods/search")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestPayload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	responseBody, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}

	return parseUSDASearchResponse(responseBody)
}

func (p *USDAProvider) GetDetails(ctx context.Context, externalID string) (Detail, error) {
	normalizedExternalID := strings.TrimSpace(externalID)
	if normalizedExternalID == "" {
		return Detail{}, fmt.Errorf("external id is required")
	}
	if strings.TrimSpace(p.apiKey) == "" {
		return Detail{}, fmt.Errorf("usda api key is required")
	}

	if cached, ok := p.getCachedDetails(normalizedExternalID); ok {
		return cached, nil
	}

	endpoint, err := p.withAPIKey("/food/" + url.PathEscape(normalizedExternalID))
	if err != nil {
		return Detail{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Detail{}, err
	}

	responseBody, err := p.doRequest(req)
	if err != nil {
		return Detail{}, err
	}

	detail, err := parseUSDADetailResponse(responseBody)
	if err != nil {
		return Detail{}, err
	}

	p.setCachedDetails(normalizedExternalID, detail)
	return detail, nil
}

func (p *USDAProvider) doRequest(req *http.Request) ([]byte, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if len(message) > 250 {
			message = message[:250]
		}
		return nil, fmt.Errorf("usda request failed with status %d: %s", resp.StatusCode, message)
	}

	return body, nil
}

func (p *USDAProvider) withAPIKey(path string) (string, error) {
	parsedURL, err := url.Parse(p.baseURL + path)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()
	query.Set("api_key", p.apiKey)
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func (p *USDAProvider) getCachedDetails(externalID string) (Detail, bool) {
	if p.detailsTTL <= 0 {
		return Detail{}, false
	}

	p.detailsLock.RLock()
	cached, ok := p.detailsByID[externalID]
	p.detailsLock.RUnlock()
	if !ok {
		return Detail{}, false
	}
	if time.Now().After(cached.expiresAt) {
		p.detailsLock.Lock()
		delete(p.detailsByID, externalID)
		p.detailsLock.Unlock()
		return Detail{}, false
	}
	return cached.detail, true
}

func (p *USDAProvider) setCachedDetails(externalID string, detail Detail) {
	if p.detailsTTL <= 0 {
		return
	}

	p.detailsLock.Lock()
	p.detailsByID[externalID] = cachedUSDAFoodDetail{
		detail:    detail,
		expiresAt: time.Now().Add(p.detailsTTL),
	}
	p.detailsLock.Unlock()
}

func parseUSDASearchResponse(body []byte) ([]SearchResult, error) {
	var payload usdaSearchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal usda search response: %w", err)
	}

	results := make([]SearchResult, 0, len(payload.Foods))
	for _, record := range payload.Foods {
		result, ok := mapUSDAFoodRecordToSearchResult(record)
		if !ok {
			continue
		}
		results = append(results, result)
	}
	return results, nil
}

func parseUSDADetailResponse(body []byte) (Detail, error) {
	var payload usdaFoodRecord
	if err := json.Unmarshal(body, &payload); err != nil {
		return Detail{}, fmt.Errorf("unmarshal usda detail response: %w", err)
	}

	detail, ok := mapUSDAFoodRecordToDetail(payload)
	if !ok {
		return Detail{}, fmt.Errorf("missing fdc id in usda detail response")
	}
	return detail, nil
}

func mapUSDAFoodRecordToSearchResult(record usdaFoodRecord) (SearchResult, bool) {
	externalID := strings.TrimSpace(strconv.FormatInt(record.FdcID, 10))
	if externalID == "0" || externalID == "" {
		return SearchResult{}, false
	}

	return SearchResult{
		ExternalID: externalID,
		Label:      normalizeFoodLabel(record.Description),
		Brand:      normalizeFoodBrand(record.BrandName, record.BrandOwner),
		Nutrients:  extractNutrients(record),
	}, true
}

func mapUSDAFoodRecordToDetail(record usdaFoodRecord) (Detail, bool) {
	searchResult, ok := mapUSDAFoodRecordToSearchResult(record)
	if !ok {
		return Detail{}, false
	}

	return Detail{
		ExternalID: searchResult.ExternalID,
		Label:      searchResult.Label,
		Brand:      searchResult.Brand,
		Nutrients:  searchResult.Nutrients,
	}, true
}

func extractNutrients(record usdaFoodRecord) Nutrients {
	// Prefer USDA branded label nutrients when present; otherwise fallback to nutrient list.
	nutrients := Nutrients{
		CaloriesKcal: cloneFloatPointer(record.LabelNutrients.Calories.Value),
		ProteinG:     cloneFloatPointer(record.LabelNutrients.Protein.Value),
		CarbsG:       cloneFloatPointer(record.LabelNutrients.Carbs.Value),
		FatG:         cloneFloatPointer(record.LabelNutrients.Fat.Value),
	}

	for _, nutrient := range record.FoodNutrients {
		nutrientID := nutrient.NutrientID
		if nutrientID == 0 && nutrient.Nutrient != nil {
			nutrientID = nutrient.Nutrient.ID
		}

		value := nutrient.Value
		if value == nil {
			value = nutrient.Amount
		}
		if value == nil {
			continue
		}

		switch nutrientID {
		case nutrientCaloriesKcalID:
			if nutrients.CaloriesKcal == nil {
				nutrients.CaloriesKcal = cloneFloatPointer(value)
			}
		case nutrientProteinGID:
			if nutrients.ProteinG == nil {
				nutrients.ProteinG = cloneFloatPointer(value)
			}
		case nutrientCarbsGID:
			if nutrients.CarbsG == nil {
				nutrients.CarbsG = cloneFloatPointer(value)
			}
		case nutrientFatGID:
			if nutrients.FatG == nil {
				nutrients.FatG = cloneFloatPointer(value)
			}
		}
	}

	return nutrients
}

func normalizeFoodLabel(description string) string {
	return strings.TrimSpace(description)
}

func normalizeFoodBrand(brandName string, brandOwner string) string {
	brand := strings.TrimSpace(brandName)
	if brand != "" {
		return brand
	}
	return strings.TrimSpace(brandOwner)
}

func cloneFloatPointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
