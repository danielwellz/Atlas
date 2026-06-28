package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/atlas/atlas-api/internal/auth"
	"github.com/atlas/atlas-api/internal/config"
	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAuthLifecycleIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	registerBody := map[string]string{
		"email":    "athlete@atlas.local",
		"password": "strongpass123",
	}
	registerResp, registerRespBody := doRequest(t, server, http.MethodPost, "/api/v1/auth/register", registerBody, nil)
	require.Equal(t, http.StatusCreated, registerResp.StatusCode)

	var registerPayload generated.AuthResponse
	require.NoError(t, json.Unmarshal(registerRespBody, &registerPayload))
	require.NotEmpty(t, registerPayload.Tokens.AccessToken)
	require.NotEmpty(t, registerPayload.Tokens.RefreshToken)
	require.Equal(t, "athlete@atlas.local", string(registerPayload.User.Email))

	loginBody := map[string]string{
		"email":    "athlete@atlas.local",
		"password": "strongpass123",
	}
	loginResp, loginRespBody := doRequest(t, server, http.MethodPost, "/api/v1/auth/login", loginBody, nil)
	require.Equal(t, http.StatusOK, loginResp.StatusCode)

	var loginPayload generated.AuthResponse
	require.NoError(t, json.Unmarshal(loginRespBody, &loginPayload))
	require.NotEmpty(t, loginPayload.Tokens.AccessToken)
	require.NotEmpty(t, loginPayload.Tokens.RefreshToken)

	meResp, meBody := doRequest(t, server, http.MethodGet, "/api/v1/me", nil, map[string]string{
		"Authorization": "Bearer " + loginPayload.Tokens.AccessToken,
	})
	require.Equal(t, http.StatusOK, meResp.StatusCode)

	var mePayload generated.UserResponse
	require.NoError(t, json.Unmarshal(meBody, &mePayload))
	require.Equal(t, loginPayload.User.Id, mePayload.User.Id)

	refreshResp, refreshBody := doRequest(t, server, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refreshToken": loginPayload.Tokens.RefreshToken,
	}, nil)
	require.Equal(t, http.StatusOK, refreshResp.StatusCode)

	var refreshPayload generated.TokenResponse
	require.NoError(t, json.Unmarshal(refreshBody, &refreshPayload))
	require.NotEqual(t, loginPayload.Tokens.AccessToken, refreshPayload.AccessToken)
	require.NotEqual(t, loginPayload.Tokens.RefreshToken, refreshPayload.RefreshToken)

	oldAccessResp, oldAccessBody := doRequest(t, server, http.MethodGet, "/api/v1/me", nil, map[string]string{
		"Authorization": "Bearer " + loginPayload.Tokens.AccessToken,
	})
	require.Equal(t, http.StatusOK, oldAccessResp.StatusCode)

	var oldAccessPayload generated.UserResponse
	require.NoError(t, json.Unmarshal(oldAccessBody, &oldAccessPayload))
	require.Equal(t, loginPayload.User.Id, oldAccessPayload.User.Id)

	logoutResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/auth/logout", map[string]string{
		"refreshToken": refreshPayload.RefreshToken,
	}, nil)
	require.Equal(t, http.StatusNoContent, logoutResp.StatusCode)

	refreshAfterLogoutResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refreshToken": refreshPayload.RefreshToken,
	}, nil)
	require.Equal(t, http.StatusUnauthorized, refreshAfterLogoutResp.StatusCode)
}

func TestConsentEndpointsIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	registerResp, registerRespBody := doRequest(t, server, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email":    "consent-user@atlas.local",
		"password": "strongpass123",
	}, nil)
	require.Equal(t, http.StatusCreated, registerResp.StatusCode)

	var registerPayload generated.AuthResponse
	require.NoError(t, json.Unmarshal(registerRespBody, &registerPayload))
	authHeader := map[string]string{"Authorization": "Bearer " + registerPayload.Tokens.AccessToken}

	consentTypes := []string{
		"movement_screen_camera",
		"form_check_local",
		"form_check_upload",
	}

	for _, consentType := range consentTypes {
		if consentType == "form_check_upload" {
			activateSubscription(t, server, authHeader, map[string]any{
				"platform":      "ios",
				"productId":     "atlas.pro.monthly",
				"receiptToken":  "rcpt-consent-pro-1",
				"transactionId": "tx-consent-pro-1",
			})
		}

		grantResp, grantBody := doRequest(t, server, http.MethodPost, "/api/v1/consents/grant", map[string]any{
			"consentType": consentType,
			"metadataJson": map[string]any{
				"source": "integration-test",
			},
		}, authHeader)
		require.Equal(t, http.StatusOK, grantResp.StatusCode)

		var grantPayload generated.ConsentResponse
		require.NoError(t, json.Unmarshal(grantBody, &grantPayload))
		require.Equal(t, generated.ConsentType(consentType), grantPayload.Consent.ConsentType)
		require.Nil(t, grantPayload.Consent.RevokedAt)
	}

	listResp, listBody := doRequest(t, server, http.MethodGet, "/api/v1/consents", nil, authHeader)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listPayload generated.ConsentsResponse
	require.NoError(t, json.Unmarshal(listBody, &listPayload))
	require.Len(t, listPayload.Consents, len(consentTypes))

	listedTypes := make(map[generated.ConsentType]struct{}, len(listPayload.Consents))
	for _, consentRecord := range listPayload.Consents {
		listedTypes[consentRecord.ConsentType] = struct{}{}
	}
	for _, consentType := range consentTypes {
		_, exists := listedTypes[generated.ConsentType(consentType)]
		require.Truef(t, exists, "expected %s to be listed", consentType)
	}

	revokeResp, revokeBody := doRequest(t, server, http.MethodPost, "/api/v1/consents/revoke", map[string]any{
		"consentType": "form_check_local",
	}, authHeader)
	require.Equal(t, http.StatusOK, revokeResp.StatusCode)

	var revokePayload generated.ConsentResponse
	require.NoError(t, json.Unmarshal(revokeBody, &revokePayload))
	require.Equal(t, generated.ConsentType("form_check_local"), revokePayload.Consent.ConsentType)
	require.NotNil(t, revokePayload.Consent.RevokedAt)

	revokeAgainResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/consents/revoke", map[string]any{
		"consentType": "form_check_local",
	}, authHeader)
	require.Equal(t, http.StatusNotFound, revokeAgainResp.StatusCode)
}

func TestFormCheckUploadRequiresEntitlementAndConsentIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	authHeader := registerAndAuthHeader(t, server, "form-check-upload-user@atlas.local")

	now := time.Now().UTC()
	uploadRequest := map[string]any{
		"movementType":      "squat",
		"recordingStartedAt": now.Add(-8 * time.Second).Format(time.RFC3339),
		"recordingEndedAt":   now.Format(time.RFC3339),
		"summary": map[string]any{
			"overallScore":        88,
			"rangeOfMotionScore":  90,
			"kneeTrackingScore":   86,
			"symmetryScore":       84,
			"rangeOfMotionDegrees": 83.5,
			"repetitionCount":     4,
			"feedback":            []string{"Solid rep quality across depth, tracking, and symmetry."},
		},
		"metadataJson": map[string]any{
			"source": "integration-test",
		},
	}

	withoutEntitlementResp, withoutEntitlementBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/form-check/uploads",
		uploadRequest,
		authHeader,
	)
	require.Equal(t, http.StatusForbidden, withoutEntitlementResp.StatusCode)

	var withoutEntitlementError generated.ErrorResponse
	require.NoError(t, json.Unmarshal(withoutEntitlementBody, &withoutEntitlementError))
	require.Equal(t, "subscription entitlement required: form_check_upload", withoutEntitlementError.Message)

	activateSubscription(t, server, authHeader, map[string]any{
		"platform":      "ios",
		"productId":     "atlas.pro.monthly",
		"receiptToken":  "rcpt-form-check-upload-1",
		"transactionId": "tx-form-check-upload-1",
	})

	withoutConsentResp, withoutConsentBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/form-check/uploads",
		uploadRequest,
		authHeader,
	)
	require.Equal(t, http.StatusForbidden, withoutConsentResp.StatusCode)

	var withoutConsentError generated.ErrorResponse
	require.NoError(t, json.Unmarshal(withoutConsentBody, &withoutConsentError))
	require.Equal(t, "form-check upload consent is not granted", withoutConsentError.Message)

	grantConsentResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/consents/grant", map[string]any{
		"consentType": "form_check_upload",
		"metadataJson": map[string]any{
			"source": "integration-test",
		},
	}, authHeader)
	require.Equal(t, http.StatusOK, grantConsentResp.StatusCode)

	successResp, successBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/form-check/uploads",
		uploadRequest,
		authHeader,
	)
	require.Equal(t, http.StatusCreated, successResp.StatusCode)

	var successPayload generated.FormCheckUploadResponse
	require.NoError(t, json.Unmarshal(successBody, &successPayload))
	require.Equal(t, generated.FormCheckMovementType("squat"), successPayload.Upload.MovementType)
	require.Equal(t, int32(88), successPayload.Upload.Summary.OverallScore)
	require.Equal(t, int32(4), successPayload.Upload.Summary.RepetitionCount)
	require.NotEmpty(t, successPayload.Upload.StorageKey)
	require.Equal(t, "integration-test", successPayload.Upload.MetadataJson["source"])

	database := openTestDatabase(t)
	defer func() {
		require.NoError(t, database.Close())
	}()

	var storedStorageKey string
	var storedOverallScore int
	err := database.QueryRow(
		`SELECT storage_key, (summary_json ->> 'overallScore')::int FROM form_check_uploads WHERE id = $1`,
		uuid.UUID(successPayload.Upload.Id),
	).Scan(&storedStorageKey, &storedOverallScore)
	require.NoError(t, err)
	require.NotEmpty(t, storedStorageKey)
	require.Equal(t, 88, storedOverallScore)
}

func TestEventsEndpointIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	anonymousResp, anonymousBody := doRequest(t, server, http.MethodPost, "/api/v1/events", map[string]any{
		"eventName":      "onboarding_started",
		"eventTime":      time.Now().UTC().Format(time.RFC3339),
		"consentGranted": true,
		"properties": map[string]any{
			"entry_point": "welcome_screen",
			"platform":    "ios",
			"app_version": "0.0.1",
		},
	}, nil)
	require.Equal(t, http.StatusAccepted, anonymousResp.StatusCode)

	var anonymousPayload generated.EventIngestResponse
	require.NoError(t, json.Unmarshal(anonymousBody, &anonymousPayload))
	require.True(t, anonymousPayload.Accepted)

	registerResp, registerBody := doRequest(t, server, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email":    "events-user@atlas.local",
		"password": "strongpass123",
	}, nil)
	require.Equal(t, http.StatusCreated, registerResp.StatusCode)

	var registerPayload generated.AuthResponse
	require.NoError(t, json.Unmarshal(registerBody, &registerPayload))
	authHeader := map[string]string{"Authorization": "Bearer " + registerPayload.Tokens.AccessToken}

	missingConsentResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/events", map[string]any{
		"eventName":      "workout_completed",
		"eventTime":      time.Now().UTC().Format(time.RFC3339),
		"consentGranted": true,
		"properties": map[string]any{
			"workout_id":        uuid.New().String(),
			"duration_minutes":  38,
			"exercise_count":    6,
			"set_count":         20,
			"completion_source": "workout_runner",
			"platform":          "ios",
			"app_version":       "0.0.1",
		},
	}, authHeader)
	require.Equal(t, http.StatusForbidden, missingConsentResp.StatusCode)

	grantConsentResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/consents/grant", map[string]any{
		"consentType": "product_analytics",
		"metadataJson": map[string]any{
			"source": "integration-test",
		},
	}, authHeader)
	require.Equal(t, http.StatusOK, grantConsentResp.StatusCode)

	withConsentResp, withConsentBody := doRequest(t, server, http.MethodPost, "/api/v1/events", map[string]any{
		"eventName":      "workout_completed",
		"eventTime":      time.Now().UTC().Format(time.RFC3339),
		"consentGranted": true,
		"properties": map[string]any{
			"workout_id":        uuid.New().String(),
			"duration_minutes":  42,
			"exercise_count":    7,
			"set_count":         24,
			"completion_source": "workout_runner",
			"platform":          "ios",
			"app_version":       "0.0.1",
		},
	}, authHeader)
	require.Equal(t, http.StatusAccepted, withConsentResp.StatusCode)

	var withConsentPayload generated.EventIngestResponse
	require.NoError(t, json.Unmarshal(withConsentBody, &withConsentPayload))
	require.True(t, withConsentPayload.Accepted)

	sensitivePayloadResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/events", map[string]any{
		"eventName":      "workout_completed",
		"eventTime":      time.Now().UTC().Format(time.RFC3339),
		"consentGranted": true,
		"properties": map[string]any{
			"email": "athlete@atlas.local",
		},
	}, authHeader)
	require.Equal(t, http.StatusBadRequest, sensitivePayloadResp.StatusCode)

	database := openTestDatabase(t)
	defer func() {
		require.NoError(t, database.Close())
	}()

	var totalEvents int
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM app_events`).Scan(&totalEvents))
	require.Equal(t, 2, totalEvents)

	var userEvents int
	require.NoError(
		t,
		database.QueryRow(
			`SELECT COUNT(*) FROM app_events WHERE event_name = 'workout_completed' AND user_id = $1`,
			registerPayload.User.Id,
		).Scan(&userEvents),
	)
	require.Equal(t, 1, userEvents)
}

func TestMeIncludesProEntitlementFlagIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	authHeader := registerAndAuthHeader(t, server, "pro-athlete@atlas.local")

	beforeResp, beforeBody := doRequest(t, server, http.MethodGet, "/api/v1/me", nil, authHeader)
	require.Equal(t, http.StatusOK, beforeResp.StatusCode)

	var beforePayload generated.UserResponse
	require.NoError(t, json.Unmarshal(beforeBody, &beforePayload))
	require.False(t, beforePayload.User.IsPro)

	activateSubscription(t, server, authHeader, map[string]any{
		"platform":      "ios",
		"productId":     "atlas.pro.monthly",
		"receiptToken":  "rcpt-me-pro-1",
		"transactionId": "tx-me-pro-1",
	})

	afterResp, afterBody := doRequest(t, server, http.MethodGet, "/api/v1/me", nil, authHeader)
	require.Equal(t, http.StatusOK, afterResp.StatusCode)

	var afterPayload generated.UserResponse
	require.NoError(t, json.Unmarshal(afterBody, &afterPayload))
	require.True(t, afterPayload.User.IsPro)
	require.Contains(t, afterPayload.User.Entitlements, generated.EntitlementKey("barcode_scan"))
	require.Equal(t, generated.CoachTier("pro"), afterPayload.User.CoachTier)
}

func TestExerciseCatalogEndpointsIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	fixtureIDs := seedExerciseFixtures(t)

	filterResp, filterBody := doRequest(t, server, http.MethodGet, "/api/v1/exercises?equipment=barbell&pattern=squat", nil, nil)
	require.Equal(t, http.StatusOK, filterResp.StatusCode)

	var filtered generated.ExercisesResponse
	require.NoError(t, json.Unmarshal(filterBody, &filtered))
	require.Len(t, filtered.Exercises, 1)
	require.Equal(t, "back-squat", filtered.Exercises[0].Slug)
	require.Equal(t, []string{"quads"}, filtered.Exercises[0].PrimaryMuscles)
	require.Equal(t, []string{"glutes", "core"}, filtered.Exercises[0].SecondaryMuscles)
	require.Equal(t, []string{"acute_knee_injury"}, filtered.Exercises[0].Contraindications)

	searchResp, searchBody := doRequest(t, server, http.MethodGet, "/api/v1/exercises?query=plank", nil, nil)
	require.Equal(t, http.StatusOK, searchResp.StatusCode)

	var searched generated.ExercisesResponse
	require.NoError(t, json.Unmarshal(searchBody, &searched))
	require.Len(t, searched.Exercises, 1)
	require.Equal(t, "plank", searched.Exercises[0].Slug)

	exerciseID := fixtureIDs["back-squat"]
	detailResp, detailBody := doRequest(t, server, http.MethodGet, "/api/v1/exercises/"+exerciseID.String(), nil, nil)
	require.Equal(t, http.StatusOK, detailResp.StatusCode)

	var detail generated.ExerciseResponse
	require.NoError(t, json.Unmarshal(detailBody, &detail))
	require.Equal(t, "back-squat", detail.Exercise.Slug)
	require.Equal(t, "squat", detail.Exercise.MovementPattern)
	require.Equal(t, []string{"quads"}, detail.Exercise.PrimaryMuscles)
	require.Equal(t, []string{"glutes", "core"}, detail.Exercise.SecondaryMuscles)
	require.Equal(t, []string{"acute_knee_injury"}, detail.Exercise.Contraindications)

	notFoundResp, _ := doRequest(t, server, http.MethodGet, "/api/v1/exercises/"+uuid.New().String(), nil, nil)
	require.Equal(t, http.StatusNotFound, notFoundResp.StatusCode)
}

func TestExerciseBiomechanicsEndpointIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	fixtureIDs := seedExerciseBiomechanicsFixtures(t)
	authHeader := registerAndAuthHeader(t, server, "biomechanics-entitlement-user@atlas.local")

	previewPath := fmt.Sprintf("/api/v1/exercises/%s/biomechanics", fixtureIDs["back-squat"].String())
	forbiddenResp, _ := doRequest(t, server, http.MethodGet, previewPath, nil, authHeader)
	require.Equal(t, http.StatusForbidden, forbiddenResp.StatusCode)

	activateSubscription(t, server, authHeader, map[string]any{
		"platform":     "ios",
		"productId":    "atlas.pro.monthly",
		"receiptToken": "rcpt-biomechanics-1",
		"transactionId": "tx-biomechanics-1",
	})

	previewResp, previewBody := doRequest(t, server, http.MethodGet, previewPath, nil, authHeader)
	require.Equal(t, http.StatusOK, previewResp.StatusCode)

	var payload struct {
		Biomechanics struct {
			ExerciseID        string `json:"exerciseId"`
			ExerciseSlug      string `json:"exerciseSlug"`
			AnimationAssetKey string `json:"animationAssetKey"`
			RigVersion        string `json:"rigVersion"`
			MuscleHighlights  []struct {
				MuscleGroup     string  `json:"muscleGroup"`
				ActivationLevel float64 `json:"activationLevel"`
				Role            string  `json:"role"`
			} `json:"muscleHighlights"`
			JointAngles []struct {
				Joint         string  `json:"joint"`
				MinDegrees    float64 `json:"minDegrees"`
				MaxDegrees    float64 `json:"maxDegrees"`
				TargetDegrees float64 `json:"targetDegrees"`
				Unit          string  `json:"unit"`
			} `json:"jointAngles"`
		} `json:"biomechanics"`
	}

	require.NoError(t, json.Unmarshal(previewBody, &payload))
	require.Equal(t, fixtureIDs["back-squat"].String(), payload.Biomechanics.ExerciseID)
	require.Equal(t, "back-squat", payload.Biomechanics.ExerciseSlug)
	require.Equal(t, "atlas-humanoid-v1", payload.Biomechanics.RigVersion)
	require.NotEmpty(t, payload.Biomechanics.AnimationAssetKey)
	require.NotEmpty(t, payload.Biomechanics.MuscleHighlights)
	require.NotEmpty(t, payload.Biomechanics.JointAngles)

	missingExerciseResp, _ := doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/exercises/"+uuid.New().String()+"/biomechanics",
		nil,
		authHeader,
	)
	require.Equal(t, http.StatusNotFound, missingExerciseResp.StatusCode)
}

func TestExerciseSubstitutesEndpointIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	fixtureIDs := seedExerciseSubstituteFixtures(t)

	constraints := url.QueryEscape(`{"equipment":["dumbbell","barbell","rack"]}`)
	substitutesPath := fmt.Sprintf(
		"/api/v1/exercises/%s/substitutes?constraints=%s&limit=3",
		fixtureIDs["back-squat"].String(),
		constraints,
	)
	substitutesResp, substitutesBody := doRequest(t, server, http.MethodGet, substitutesPath, nil, nil)
	require.Equal(t, http.StatusOK, substitutesResp.StatusCode)

	var substitutesPayload generated.ExerciseSubstitutesResponse
	require.NoError(t, json.Unmarshal(substitutesBody, &substitutesPayload))
	require.Len(t, substitutesPayload.Substitutes, 3)
	require.Equal(t, "goblet-squat", substitutesPayload.Substitutes[0].Exercise.Slug)
	require.Equal(t, "front-squat", substitutesPayload.Substitutes[1].Exercise.Slug)
	require.Equal(t, "split-squat", substitutesPayload.Substitutes[2].Exercise.Slug)
	require.NotEmpty(t, substitutesPayload.Substitutes[0].Why.MatchedPattern)
	require.NotEmpty(t, substitutesPayload.Substitutes[0].Why.MatchedMuscles)
	require.Equal(
		t,
		"partial",
		string(substitutesPayload.Substitutes[0].Why.EquipmentFit),
	)

	repeatResp, repeatBody := doRequest(t, server, http.MethodGet, substitutesPath, nil, nil)
	require.Equal(t, http.StatusOK, repeatResp.StatusCode)
	var repeatPayload generated.ExerciseSubstitutesResponse
	require.NoError(t, json.Unmarshal(repeatBody, &repeatPayload))
	require.Equal(t, substitutesPayload.Substitutes, repeatPayload.Substitutes)

	excludeContraPath := fmt.Sprintf(
		"/api/v1/exercises/%s/substitutes?equipment=dumbbell,barbell,rack&injuryFlags=acute_knee_injury&limit=3",
		fixtureIDs["back-squat"].String(),
	)
	excludeContraResp, excludeContraBody := doRequest(t, server, http.MethodGet, excludeContraPath, nil, nil)
	require.Equal(t, http.StatusOK, excludeContraResp.StatusCode)

	var excludeContraPayload generated.ExerciseSubstitutesResponse
	require.NoError(t, json.Unmarshal(excludeContraBody, &excludeContraPayload))
	require.Len(t, excludeContraPayload.Substitutes, 2)
	require.Equal(t, "goblet-squat", excludeContraPayload.Substitutes[0].Exercise.Slug)
	require.Equal(t, "split-squat", excludeContraPayload.Substitutes[1].Exercise.Slug)

	equipmentPath := fmt.Sprintf(
		"/api/v1/exercises/%s/substitutes?equipment=barbell&limit=3",
		fixtureIDs["back-squat"].String(),
	)
	equipmentResp, equipmentBody := doRequest(t, server, http.MethodGet, equipmentPath, nil, nil)
	require.Equal(t, http.StatusOK, equipmentResp.StatusCode)

	var equipmentPayload generated.ExerciseSubstitutesResponse
	require.NoError(t, json.Unmarshal(equipmentBody, &equipmentPayload))
	require.Len(t, equipmentPayload.Substitutes, 1)
	require.Equal(t, "front-squat", equipmentPayload.Substitutes[0].Exercise.Slug)

	notFoundPath := fmt.Sprintf("/api/v1/exercises/%s/substitutes", uuid.New().String())
	notFoundResp, _ := doRequest(t, server, http.MethodGet, notFoundPath, nil, nil)
	require.Equal(t, http.StatusNotFound, notFoundResp.StatusCode)

	invalidConstraintsPath := fmt.Sprintf(
		"/api/v1/exercises/%s/substitutes?constraints=not-json",
		fixtureIDs["back-squat"].String(),
	)
	invalidConstraintsResp, _ := doRequest(t, server, http.MethodGet, invalidConstraintsPath, nil, nil)
	require.Equal(t, http.StatusBadRequest, invalidConstraintsResp.StatusCode)
}

func TestOnboardingProfileGoalsIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	registerResp, registerRespBody := doRequest(t, server, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email":    "onboarding-user@atlas.local",
		"password": "strongpass123",
	}, nil)
	require.Equal(t, http.StatusCreated, registerResp.StatusCode)

	var registerPayload generated.AuthResponse
	require.NoError(t, json.Unmarshal(registerRespBody, &registerPayload))
	authHeader := map[string]string{"Authorization": "Bearer " + registerPayload.Tokens.AccessToken}

	statusResp, statusBody := doRequest(t, server, http.MethodGet, "/api/v1/onboarding/status", nil, authHeader)
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	var initialStatus generated.OnboardingStatusResponse
	require.NoError(t, json.Unmarshal(statusBody, &initialStatus))
	require.False(t, initialStatus.ProfileCompleted)
	require.False(t, initialStatus.GoalsCompleted)
	require.False(t, initialStatus.OnboardingCompleted)
	require.Nil(t, initialStatus.FirstWeekPlan)
	require.Nil(t, initialStatus.TrainingProfile)
	require.Nil(t, initialStatus.PlanExplanation)

	putProfileResp, putProfileBody := doRequest(t, server, http.MethodPut, "/api/v1/profile", map[string]any{
		"displayName":     "Athlete One",
		"sex":             "female",
		"heightCm":        168,
		"weightKg":        63.5,
		"experienceLevel": "beginner",
	}, authHeader)
	require.Equal(t, http.StatusOK, putProfileResp.StatusCode)

	var profilePayload generated.ProfileResponse
	require.NoError(t, json.Unmarshal(putProfileBody, &profilePayload))
	require.Equal(t, "Athlete One", profilePayload.Profile.DisplayName)
	require.Equal(t, int32(168), profilePayload.Profile.HeightCm)
	require.Equal(t, float32(63.5), profilePayload.Profile.WeightKg)

	invalidDaysResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/onboarding/profile", map[string]any{
		"primaryGoal":            "build_strength",
		"secondaryGoal":          "improve_mobility",
		"daysPerWeek":            0,
		"sessionDurationMinutes": 60,
		"equipmentAccessJson":    []string{"barbell"},
		"injuriesLimitationsFlags": []string{
			"none",
		},
		"modalityPreferences": []string{
			"barbell_strength",
		},
		"constraintsJson": map[string]any{
			"kneePain":     false,
			"scheduleDays": []string{"Monday", "Wednesday"},
		},
	}, authHeader)
	require.Equal(t, http.StatusBadRequest, invalidDaysResp.StatusCode)

	invalidDurationResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/onboarding/profile", map[string]any{
		"primaryGoal":            "build_strength",
		"secondaryGoal":          "improve_mobility",
		"daysPerWeek":            4,
		"sessionDurationMinutes": 10,
		"equipmentAccessJson":    []string{"barbell"},
		"injuriesLimitationsFlags": []string{
			"none",
		},
		"modalityPreferences": []string{
			"barbell_strength",
		},
		"constraintsJson": map[string]any{
			"kneePain":     false,
			"scheduleDays": []string{"Monday", "Wednesday"},
		},
	}, authHeader)
	require.Equal(t, http.StatusBadRequest, invalidDurationResp.StatusCode)

	missingInjuriesResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/onboarding/profile", map[string]any{
		"primaryGoal":              "build_strength",
		"secondaryGoal":            "improve_mobility",
		"daysPerWeek":              4,
		"sessionDurationMinutes":   60,
		"equipmentAccessJson":      []string{"barbell", "dumbbell"},
		"injuriesLimitationsFlags": []string{},
		"modalityPreferences": []string{
			"barbell_strength",
		},
		"constraintsJson": map[string]any{
			"kneePain":     false,
			"scheduleDays": []string{"Monday", "Wednesday", "Friday", "Saturday"},
		},
	}, authHeader)
	require.Equal(t, http.StatusBadRequest, missingInjuriesResp.StatusCode)

	putOnboardingProfileResp, putOnboardingProfileBody := doRequest(t, server, http.MethodPut, "/api/v1/onboarding/profile", map[string]any{
		"primaryGoal":            "build_strength",
		"secondaryGoal":          "improve_mobility",
		"daysPerWeek":            4,
		"sessionDurationMinutes": 60,
		"equipmentAccessJson":    []string{"barbell", "dumbbell"},
		"injuriesLimitationsFlags": []string{
			"none",
		},
		"modalityPreferences": []string{
			"barbell_strength",
			"hypertrophy_accessory",
		},
		"priorTrainingHistory": map[string]any{
			"consistentYears": 2,
			"lastProgram":     "novice_linear",
		},
		"readinessSignals": map[string]any{
			"sleepQuality": "good",
			"energy":       4,
		},
		"constraintsJson": map[string]any{
			"kneePain":     false,
			"scheduleDays": []string{"Monday", "Wednesday", "Friday", "Saturday"},
		},
	}, authHeader)
	require.Equal(t, http.StatusOK, putOnboardingProfileResp.StatusCode)

	var onboardingProfilePayload generated.OnboardingProfileResponse
	require.NoError(t, json.Unmarshal(putOnboardingProfileBody, &onboardingProfilePayload))
	require.Equal(t, "build_strength", onboardingProfilePayload.TrainingProfile.PrimaryGoal)
	require.Equal(t, int32(4), onboardingProfilePayload.TrainingProfile.DaysPerWeek)
	require.Equal(t, int32(60), onboardingProfilePayload.TrainingProfile.SessionDurationMinutes)
	require.ElementsMatch(t, []string{"barbell", "dumbbell"}, onboardingProfilePayload.TrainingProfile.EquipmentAccess)
	require.ElementsMatch(t, []string{"none"}, onboardingProfilePayload.TrainingProfile.InjuriesLimitationsFlags)
	require.ElementsMatch(
		t,
		[]string{"barbell_strength", "hypertrophy_accessory"},
		onboardingProfilePayload.TrainingProfile.ModalityPreferences,
	)
	require.NotNil(t, onboardingProfilePayload.TrainingProfile.PriorTrainingHistory)
	require.NotNil(t, onboardingProfilePayload.TrainingProfile.ReadinessSignals)

	planResp, planBody := doRequest(t, server, http.MethodGet, "/api/v1/onboarding/plan", nil, authHeader)
	require.Equal(t, http.StatusOK, planResp.StatusCode)

	var planPayload generated.OnboardingPlanResponse
	require.NoError(t, json.Unmarshal(planBody, &planPayload))
	require.NotEmpty(t, planPayload.Explanation)
	require.Len(t, planPayload.FirstWeekPlan.Days, 4)
	require.Equal(t, "Monday", planPayload.FirstWeekPlan.Days[0].Day)
	require.Equal(t, "Strength Builder 1", planPayload.FirstWeekPlan.Days[0].SessionName)
	require.Equal(t, "Wednesday", planPayload.FirstWeekPlan.Days[1].Day)
	require.Equal(t, "Strength Builder 2", planPayload.FirstWeekPlan.Days[1].SessionName)
	require.Equal(t, int32(4), planPayload.TrainingProfile.DaysPerWeek)

	statusRespAfter, statusBodyAfter := doRequest(t, server, http.MethodGet, "/api/v1/onboarding/status", nil, authHeader)
	require.Equal(t, http.StatusOK, statusRespAfter.StatusCode)

	var completedStatus generated.OnboardingStatusResponse
	require.NoError(t, json.Unmarshal(statusBodyAfter, &completedStatus))
	require.True(t, completedStatus.ProfileCompleted)
	require.True(t, completedStatus.GoalsCompleted)
	require.True(t, completedStatus.OnboardingCompleted)
	require.NotNil(t, completedStatus.FirstWeekPlan)
	require.NotNil(t, completedStatus.TrainingProfile)
	require.NotNil(t, completedStatus.PlanExplanation)
	require.Len(t, completedStatus.FirstWeekPlan.Days, 4)
	require.Equal(t, "Monday", completedStatus.FirstWeekPlan.Days[0].Day)
	require.Equal(t, "Strength Builder 1", completedStatus.FirstWeekPlan.Days[0].SessionName)
	require.Equal(t, "Wednesday", completedStatus.FirstWeekPlan.Days[1].Day)
	require.Equal(t, "Strength Builder 2", completedStatus.FirstWeekPlan.Days[1].SessionName)

	updateProfileResp, updateProfileBody := doRequest(t, server, http.MethodPut, "/api/v1/profile", map[string]any{
		"displayName":     "Athlete Updated",
		"sex":             "female",
		"heightCm":        168,
		"weightKg":        64.0,
		"experienceLevel": "intermediate",
	}, authHeader)
	require.Equal(t, http.StatusOK, updateProfileResp.StatusCode)

	var updatedProfile generated.ProfileResponse
	require.NoError(t, json.Unmarshal(updateProfileBody, &updatedProfile))
	require.Equal(t, "Athlete Updated", updatedProfile.Profile.DisplayName)
	require.Equal(t, "intermediate", updatedProfile.Profile.ExperienceLevel)

	updateOnboardingProfileResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/onboarding/profile", map[string]any{
		"primaryGoal":            "build_strength",
		"secondaryGoal":          "run_faster",
		"daysPerWeek":            5,
		"sessionDurationMinutes": 75,
		"equipmentAccessJson":    []string{"barbell", "dumbbell", "bench"},
		"injuriesLimitationsFlags": []string{
			"knee_sensitivity",
		},
		"modalityPreferences": []string{
			"barbell_strength",
			"low_impact_conditioning",
		},
		"constraintsJson": map[string]any{
			"kneePain":     false,
			"timeCap":      75,
			"scheduleDays": []string{"Monday", "Tuesday", "Thursday", "Saturday", "Sunday"},
		},
	}, authHeader)
	require.Equal(t, http.StatusOK, updateOnboardingProfileResp.StatusCode)

	statusRespFinal, statusBodyFinal := doRequest(t, server, http.MethodGet, "/api/v1/onboarding/status", nil, authHeader)
	require.Equal(t, http.StatusOK, statusRespFinal.StatusCode)

	var finalStatus generated.OnboardingStatusResponse
	require.NoError(t, json.Unmarshal(statusBodyFinal, &finalStatus))
	require.NotNil(t, finalStatus.FirstWeekPlan)
	require.NotNil(t, finalStatus.TrainingProfile)
	require.Len(t, finalStatus.FirstWeekPlan.Days, 5)
	require.Equal(t, "Sunday", finalStatus.FirstWeekPlan.Days[4].Day)
	require.Equal(t, "Strength Builder 5", finalStatus.FirstWeekPlan.Days[4].SessionName)
	require.Equal(t, int32(5), finalStatus.TrainingProfile.DaysPerWeek)
	require.ElementsMatch(t, []string{"knee_sensitivity"}, finalStatus.TrainingProfile.InjuriesLimitationsFlags)
}

func TestProgramEnrollmentSetsStartDateAndWeekOneIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)

	authHeader := registerAndAuthHeader(t, server, "program-enroll-user@atlas.local")

	enrollResp, enrollBody := doRequest(t, server, http.MethodPost, "/api/v1/programs/enroll", map[string]any{
		"program_id": programID.String(),
	}, authHeader)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	var enrollPayload generated.ProgramEnrollmentResponse
	require.NoError(t, json.Unmarshal(enrollBody, &enrollPayload))
	require.Equal(t, programID, uuid.UUID(enrollPayload.Enrollment.ProgramId))
	require.Equal(t, int32(1), enrollPayload.Enrollment.CurrentWeek)
	require.Equal(t, time.Now().UTC().Format("2006-01-02"), enrollPayload.Enrollment.StartDate.String())
}

func TestProgramsCurrentScheduleReturnsStructureIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)

	listResp, listBody := doRequest(t, server, http.MethodGet, "/api/v1/programs", nil, nil)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listPayload generated.ProgramsResponse
	require.NoError(t, json.Unmarshal(listBody, &listPayload))
	require.NotEmpty(t, listPayload.Programs)

	found := false
	for _, program := range listPayload.Programs {
		if uuid.UUID(program.Id) == programID {
			found = true
		}
	}
	require.True(t, found)

	authHeader := registerAndAuthHeader(t, server, "program-schedule-user@atlas.local")

	enrollResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/programs/enroll", map[string]any{
		"program_id": programID.String(),
	}, authHeader)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	currentResp, currentBody := doRequest(t, server, http.MethodGet, "/api/v1/programs/current", nil, authHeader)
	require.Equal(t, http.StatusOK, currentResp.StatusCode)

	var currentPayload generated.CurrentProgramScheduleResponse
	require.NoError(t, json.Unmarshal(currentBody, &currentPayload))
	require.Equal(t, int32(1), currentPayload.Enrollment.CurrentWeek)
	require.Equal(t, int32(1), currentPayload.Week.WeekIndex)
	require.Equal(t, int32(3), currentPayload.Program.WeeklyFrequency)
	require.Len(t, currentPayload.Program.Blocks, int(currentPayload.Program.WeeksLength))
	require.Equal(t, int32(1), currentPayload.Context.BlockWeekIndex)
	require.Equal(t, int32(1), currentPayload.Context.TemplateWeekIndex)
	require.Len(t, currentPayload.Week.Sessions, 3)

	for _, session := range currentPayload.Week.Sessions {
		require.NotZero(t, session.DayOfWeek)
		require.NotEmpty(t, session.Name)
		require.NotEmpty(t, session.Exercises)
		for _, item := range session.Exercises {
			require.NotEmpty(t, item.ExerciseSlug)
			require.NotEmpty(t, item.ExerciseName)
			require.Greater(t, item.Prescription.Sets, int32(0))
			require.NotEmpty(t, item.Prescription.RepsRange)
			require.Greater(t, item.Prescription.RestSeconds, int32(0))
			require.NotNil(t, item.SubstitutionCandidates)
		}
	}

	lowerDayExercise := findProgramSessionExerciseByDayAndOrder(t, currentPayload, 3, 1)
	require.NotEmpty(t, lowerDayExercise.SubstitutionCandidates)
	require.Equal(t, "front-squat", lowerDayExercise.SubstitutionCandidates[0].ExerciseSlug)
}

func TestProgramsCurrentSessionsGenerateDeterministicRangeIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)

	email := "program-session-range-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	goalsResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/goals", map[string]any{
		"primaryGoal":            "hypertrophy",
		"daysPerWeek":            3,
		"sessionDurationMinutes": 60,
		"equipmentAccessJson":    []string{"barbell", "rack"},
		"constraintsJson": map[string]any{
			"scheduleDays": []string{"Tuesday", "Thursday", "Saturday"},
		},
	}, authHeader)
	require.Equal(t, http.StatusOK, goalsResp.StatusCode)

	enrollResp, enrollBody := doRequest(t, server, http.MethodPost, "/api/v1/programs/enroll", map[string]any{
		"program_id": programID.String(),
	}, authHeader)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	var enrollPayload generated.ProgramEnrollmentResponse
	require.NoError(t, json.Unmarshal(enrollBody, &enrollPayload))
	userID := uuid.UUID(enrollPayload.Enrollment.UserId)

	database := openTestDatabase(t)
	defer database.Close()

	_, err := database.Exec(
		`UPDATE user_program_enrollments SET start_date = $1 WHERE user_id = $2`,
		"2026-02-23",
		userID,
	)
	require.NoError(t, err)

	resp, body := doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/programs/current/sessions?from=2026-02-23&to=2026-03-08",
		nil,
		authHeader,
	)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload generated.CurrentProgramSessionsResponse
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, int32(3), payload.Program.WeeklyFrequency)
	require.Len(t, payload.Sessions, 6)

	require.Equal(t, "2026-02-24", payload.Sessions[0].ScheduledDate.String())
	require.Equal(t, "2026-02-26", payload.Sessions[1].ScheduledDate.String())
	require.Equal(t, "2026-02-28", payload.Sessions[2].ScheduledDate.String())
	require.Equal(t, "2026-03-03", payload.Sessions[3].ScheduledDate.String())
	require.Equal(t, "2026-03-05", payload.Sessions[4].ScheduledDate.String())
	require.Equal(t, "2026-03-07", payload.Sessions[5].ScheduledDate.String())

	require.Equal(t, int32(1), payload.Sessions[0].BlockWeekIndex)
	require.Equal(t, int32(1), payload.Sessions[1].BlockWeekIndex)
	require.Equal(t, int32(1), payload.Sessions[2].BlockWeekIndex)
	require.Equal(t, int32(2), payload.Sessions[3].BlockWeekIndex)
	require.Equal(t, int32(2), payload.Sessions[4].BlockWeekIndex)
	require.Equal(t, int32(2), payload.Sessions[5].BlockWeekIndex)

	require.Equal(t, int32(2), payload.Sessions[0].DayOfWeek)
	require.Equal(t, int32(4), payload.Sessions[1].DayOfWeek)
	require.Equal(t, int32(6), payload.Sessions[2].DayOfWeek)
}

func TestProgramsCurrentProgressionLoadIncreaseIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)
	programSessionID := getProgramSessionIDForProgramDay(t, programID, 3)

	authHeader := registerAndAuthHeader(t, server, "program-progression-increase@atlas.local")

	enrollResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/programs/enroll", map[string]any{
		"program_id": programID.String(),
	}, authHeader)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	startResp, startBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/start", map[string]any{
		"program_session_id": programSessionID.String(),
	}, authHeader)
	require.Equal(t, http.StatusCreated, startResp.StatusCode)

	var startPayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(startBody, &startPayload))
	require.NotEmpty(t, startPayload.Workout.Exercises)
	mainLift := startPayload.Workout.Exercises[0]

	addSetResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+startPayload.Workout.Id.String()+"/add_set", map[string]any{
		"idempotency_key":     "progression-increase-set-1",
		"workout_exercise_id": mainLift.Id.String(),
		"reps":                10,
		"weight_kg":           100.0,
		"rpe":                 7.0,
	}, authHeader)
	require.Equal(t, http.StatusCreated, addSetResp.StatusCode)

	completeResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+startPayload.Workout.Id.String()+"/complete", map[string]any{
		"notes": "progression increase complete",
	}, authHeader)
	require.Equal(t, http.StatusOK, completeResp.StatusCode)

	currentResp, currentBody := doRequest(t, server, http.MethodGet, "/api/v1/programs/current", nil, authHeader)
	require.Equal(t, http.StatusOK, currentResp.StatusCode)

	var currentPayload generated.CurrentProgramScheduleResponse
	require.NoError(t, json.Unmarshal(currentBody, &currentPayload))

	sessionExercise := findProgramSessionExerciseByDayAndOrder(t, currentPayload, 3, 1)
	require.NotNil(t, sessionExercise.RecommendedLoadKg)
	require.InDelta(t, 105.0, *sessionExercise.RecommendedLoadKg, 0.001)
	require.NotNil(t, sessionExercise.ProgressionWhy)
	require.Contains(t, strings.ToLower(*sessionExercise.ProgressionWhy), "increasing load")
}

func TestProgramsCurrentProgressionDeloadTriggerIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)
	programSessionID := getProgramSessionIDForProgramDay(t, programID, 3)

	authHeader := registerAndAuthHeader(t, server, "program-progression-deload@atlas.local")

	enrollResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/programs/enroll", map[string]any{
		"program_id": programID.String(),
	}, authHeader)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	startResp, startBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/start", map[string]any{
		"program_session_id": programSessionID.String(),
	}, authHeader)
	require.Equal(t, http.StatusCreated, startResp.StatusCode)

	var startPayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(startBody, &startPayload))
	require.NotEmpty(t, startPayload.Workout.Exercises)
	mainLift := startPayload.Workout.Exercises[0]

	addSetResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+startPayload.Workout.Id.String()+"/add_set", map[string]any{
		"idempotency_key":     "progression-deload-set-1",
		"workout_exercise_id": mainLift.Id.String(),
		"reps":                3,
		"weight_kg":           100.0,
		"rpe":                 9.5,
	}, authHeader)
	require.Equal(t, http.StatusCreated, addSetResp.StatusCode)

	completeResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+startPayload.Workout.Id.String()+"/complete", map[string]any{
		"notes": "progression deload complete",
	}, authHeader)
	require.Equal(t, http.StatusOK, completeResp.StatusCode)

	currentResp, currentBody := doRequest(t, server, http.MethodGet, "/api/v1/programs/current", nil, authHeader)
	require.Equal(t, http.StatusOK, currentResp.StatusCode)

	var currentPayload generated.CurrentProgramScheduleResponse
	require.NoError(t, json.Unmarshal(currentBody, &currentPayload))

	sessionExercise := findProgramSessionExerciseByDayAndOrder(t, currentPayload, 3, 1)
	require.NotNil(t, sessionExercise.RecommendedLoadKg)
	require.InDelta(t, 95.0, *sessionExercise.RecommendedLoadKg, 0.001)
	require.NotNil(t, sessionExercise.ProgressionWhy)
	require.Contains(t, strings.ToLower(*sessionExercise.ProgressionWhy), "slight deload")
}

func TestProgramsCurrentProgressionMissedSessionAdaptationIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)
	programSessionID := getProgramSessionIDForProgramDay(t, programID, 3)

	email := "program-progression-missed@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	enrollResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/programs/enroll", map[string]any{
		"program_id": programID.String(),
	}, authHeader)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	userID := lookupUserIDByEmail(t, email)
	database := openTestDatabase(t)
	defer database.Close()

	previousWeekTime := time.Now().UTC().AddDate(0, 0, -8)
	_, err := database.Exec(`
		INSERT INTO workouts (id, user_id, program_session_id, started_at, completed_at, notes, created_at)
		VALUES ($1, $2, $3, $4, $4, '', $4)
	`, uuid.New(), userID, programSessionID, previousWeekTime)
	require.NoError(t, err)

	startResp, startBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/start", map[string]any{
		"program_session_id": programSessionID.String(),
	}, authHeader)
	require.Equal(t, http.StatusCreated, startResp.StatusCode)

	var startPayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(startBody, &startPayload))

	completeResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+startPayload.Workout.Id.String()+"/complete", map[string]any{
		"notes": "progression missed-session complete",
	}, authHeader)
	require.Equal(t, http.StatusOK, completeResp.StatusCode)

	currentResp, currentBody := doRequest(t, server, http.MethodGet, "/api/v1/programs/current", nil, authHeader)
	require.Equal(t, http.StatusOK, currentResp.StatusCode)

	var currentPayload generated.CurrentProgramScheduleResponse
	require.NoError(t, json.Unmarshal(currentBody, &currentPayload))

	sessionExercise := findProgramSessionExerciseByDayAndOrder(t, currentPayload, 3, 1)
	require.Equal(t, int32(3), sessionExercise.Prescription.Sets)
	require.NotNil(t, sessionExercise.ProgressionWhy)
	require.Contains(t, strings.ToLower(*sessionExercise.ProgressionWhy), "catch-up")
}

func TestProgramsCurrentProgressionWeeklySequenceUpdatesNextWeekIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)
	programSessionID := getProgramSessionIDForProgramDay(t, programID, 3)

	email := "program-progression-weekly-sequence@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	enrollResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/programs/enroll", map[string]any{
		"program_id": programID.String(),
	}, authHeader)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	userID := lookupUserIDByEmail(t, email)
	database := openTestDatabase(t)
	defer database.Close()

	now := time.Now().UTC()
	weekOneStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -8)
	_, err := database.Exec(`
		UPDATE user_program_enrollments
		SET start_date = $1, current_week = 2
		WHERE user_id = $2
	`, weekOneStart, userID)
	require.NoError(t, err)

	insertHistoryWorkout := func(completedAt time.Time, reps int32, weight float64, rpe float64) {
		workoutID := uuid.New()
		workoutExerciseID := uuid.New()

		_, err := database.Exec(`
			INSERT INTO workouts (id, user_id, program_session_id, started_at, completed_at, notes, created_at)
			VALUES ($1, $2, $3, $4, $4, '', $4)
		`, workoutID, userID, programSessionID, completedAt)
		require.NoError(t, err)

		_, err = database.Exec(`
			INSERT INTO workout_exercises (id, workout_id, exercise_id, order_index, planned_json, actual_json, created_at)
			SELECT
				$1,
				$2,
				pse.exercise_id,
				pse.order_index,
				pse.prescription_json,
				'{}'::jsonb,
				$3
			FROM program_session_exercises pse
			WHERE pse.program_session_id = $4
			  AND pse.order_index = 1
		`, workoutExerciseID, workoutID, completedAt, programSessionID)
		require.NoError(t, err)

		_, err = database.Exec(`
			INSERT INTO workout_sets (id, workout_exercise_id, set_index, reps, weight_kg, rpe, completed_at, created_at, idempotency_key)
			VALUES ($1, $2, 1, $3, $4, $5, $6, $6, $7)
		`, uuid.New(), workoutExerciseID, reps, weight, rpe, completedAt, uuid.NewString())
		require.NoError(t, err)
	}

	insertHistoryWorkout(weekOneStart.AddDate(0, 0, 1).Add(2*time.Hour), 8, 95, 8.0)
	insertHistoryWorkout(weekOneStart.AddDate(0, 0, 3).Add(3*time.Hour), 8, 97.5, 8.2)

	startResp, startBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/start", map[string]any{
		"program_session_id": programSessionID.String(),
	}, authHeader)
	require.Equal(t, http.StatusCreated, startResp.StatusCode)

	var startPayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(startBody, &startPayload))
	require.NotEmpty(t, startPayload.Workout.Exercises)
	mainLift := startPayload.Workout.Exercises[0]

	addSetResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+startPayload.Workout.Id.String()+"/add_set", map[string]any{
		"idempotency_key":     "progression-weekly-sequence-set-1",
		"workout_exercise_id": mainLift.Id.String(),
		"reps":                8,
		"weight_kg":           100.0,
		"rpe":                 8.0,
	}, authHeader)
	require.Equal(t, http.StatusCreated, addSetResp.StatusCode)

	completeResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+startPayload.Workout.Id.String()+"/complete", map[string]any{
		"notes": "progression weekly sequence complete",
	}, authHeader)
	require.Equal(t, http.StatusOK, completeResp.StatusCode)

	currentResp, currentBody := doRequest(t, server, http.MethodGet, "/api/v1/programs/current", nil, authHeader)
	require.Equal(t, http.StatusOK, currentResp.StatusCode)

	var currentPayload generated.CurrentProgramScheduleResponse
	require.NoError(t, json.Unmarshal(currentBody, &currentPayload))

	sessionExercise := findProgramSessionExerciseByDayAndOrder(t, currentPayload, 3, 1)
	require.Equal(t, int32(4), sessionExercise.Prescription.Sets)
	require.NotNil(t, sessionExercise.AdjustmentReasons)
	reasons := strings.ToLower(strings.Join(*sessionExercise.AdjustmentReasons, " "))
	require.Contains(t, reasons, "completed 2/3 sessions")
	require.Contains(t, reasons, "partial adherence")

	var (
		currentWeek                  int32
		deloadFlag                   bool
		lastWeekAdherence            float64
		lastWeekScheduledSessions    int32
		lastWeekCompletedSessions    int32
		consecutiveLowAdherenceWeeks int32
	)
	err = database.QueryRow(`
		SELECT
			current_week,
			deload_flag,
			last_week_adherence,
			last_week_scheduled_sessions,
			last_week_completed_sessions,
			consecutive_low_adherence_weeks
		FROM user_program_state
		WHERE user_id = $1
		  AND program_id = $2
	`, userID, programID).Scan(
		&currentWeek,
		&deloadFlag,
		&lastWeekAdherence,
		&lastWeekScheduledSessions,
		&lastWeekCompletedSessions,
		&consecutiveLowAdherenceWeeks,
	)
	require.NoError(t, err)
	require.Equal(t, int32(2), currentWeek)
	require.False(t, deloadFlag)
	require.InDelta(t, 0.6667, lastWeekAdherence, 0.001)
	require.Equal(t, int32(3), lastWeekScheduledSessions)
	require.Equal(t, int32(2), lastWeekCompletedSessions)
	require.Equal(t, int32(1), consecutiveLowAdherenceWeeks)
}

func TestWorkoutsStartFromProgramSessionCreatesWorkoutExercisesIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)
	programSessionID := getProgramSessionIDForProgramDay(t, programID, 1)

	authHeader := registerAndAuthHeader(t, server, "workout-start-user@atlas.local")

	startResp, startBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/start", map[string]any{
		"program_session_id": programSessionID.String(),
	}, authHeader)
	require.Equal(t, http.StatusCreated, startResp.StatusCode)

	var startPayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(startBody, &startPayload))
	require.NotNil(t, startPayload.Workout.ProgramSessionId)
	require.Equal(t, programSessionID, uuid.UUID(*startPayload.Workout.ProgramSessionId))
	require.NotEmpty(t, startPayload.Workout.Exercises)

	database := openTestDatabase(t)
	defer database.Close()

	var count int
	err := database.QueryRow(
		`SELECT COUNT(*) FROM workout_exercises WHERE workout_id = $1`,
		uuid.UUID(startPayload.Workout.Id),
	).Scan(&count)
	require.NoError(t, err)
	require.Greater(t, count, 0)
}

func TestWorkoutsStartIncludesPreviousPerformanceByExerciseIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)
	programSessionID := getProgramSessionIDForProgramDay(t, programID, 3)

	email := "workout-prev-performance-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)
	userID := lookupUserIDByEmail(t, email)

	database := openTestDatabase(t)
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var backSquatExerciseID uuid.UUID
	err := database.QueryRowContext(ctx, `SELECT id FROM exercises WHERE slug = $1 LIMIT 1`, "back-squat").
		Scan(&backSquatExerciseID)
	require.NoError(t, err)

	previousWorkoutID := uuid.New()
	previousWorkoutExerciseID := uuid.New()
	previousCompletedAt := time.Now().UTC().AddDate(0, 0, -3)

	_, err = database.ExecContext(ctx, `
		INSERT INTO workouts (id, user_id, program_session_id, started_at, completed_at, notes, created_at)
		VALUES ($1, $2, $3, $4, $4, '', $4)
	`, previousWorkoutID, userID, programSessionID, previousCompletedAt)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO workout_exercises (id, workout_id, exercise_id, order_index, planned_json, actual_json, created_at)
		VALUES ($1, $2, $3, 1, '{}'::jsonb, '{}'::jsonb, $4)
	`, previousWorkoutExerciseID, previousWorkoutID, backSquatExerciseID, previousCompletedAt)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO workout_sets (id, workout_exercise_id, set_index, reps, weight_kg, rpe, completed_at, created_at)
		VALUES
			($1, $2, 1, 8, 100.0, 8.0, $3, $3),
			($4, $2, 2, 8, 102.5, NULL, $3, $3)
	`, uuid.New(), previousWorkoutExerciseID, previousCompletedAt, uuid.New())
	require.NoError(t, err)

	startResp, startBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/start", map[string]any{
		"program_session_id": programSessionID.String(),
	}, authHeader)
	require.Equal(t, http.StatusCreated, startResp.StatusCode)

	var startPayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(startBody, &startPayload))

	var backSquatWorkoutExercise generated.WorkoutExercise
	foundExercise := false
	for _, exercise := range startPayload.Workout.Exercises {
		if exercise.ExerciseSlug == "back-squat" {
			backSquatWorkoutExercise = exercise
			foundExercise = true
			break
		}
	}
	require.True(t, foundExercise)

	previousRaw, hasPrevious := backSquatWorkoutExercise.PlannedJson["previous_performance"]
	require.True(t, hasPrevious)

	previousPerformance, ok := previousRaw.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, previousWorkoutID.String(), previousPerformance["workout_id"])

	completedAtRaw, hasCompletedAt := previousPerformance["completed_at"]
	require.True(t, hasCompletedAt)
	require.NotEmpty(t, completedAtRaw)

	setsRaw, ok := previousPerformance["sets"].([]interface{})
	require.True(t, ok)
	require.Len(t, setsRaw, 2)

	firstSet, ok := setsRaw[0].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(1), firstSet["set_index"])
	require.Equal(t, float64(8), firstSet["reps"])
	require.Equal(t, 100.0, firstSet["weight_kg"])
	require.Equal(t, 8.0, firstSet["rpe"])

	secondSet, ok := setsRaw[1].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(2), secondSet["set_index"])
	require.Equal(t, float64(8), secondSet["reps"])
	require.Equal(t, 102.5, secondSet["weight_kg"])
	require.Nil(t, secondSet["rpe"])
}

func TestWorkoutsAddSetPersistsAndAutoIncrementsSetIndexIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)
	programSessionID := getProgramSessionIDForProgramDay(t, programID, 1)

	authHeader := registerAndAuthHeader(t, server, "workout-add-set-user@atlas.local")

	startResp, startBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/start", map[string]any{
		"program_session_id": programSessionID.String(),
	}, authHeader)
	require.Equal(t, http.StatusCreated, startResp.StatusCode)

	var startPayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(startBody, &startPayload))
	require.NotEmpty(t, startPayload.Workout.Exercises)

	workoutID := startPayload.Workout.Id
	workoutExerciseID := startPayload.Workout.Exercises[0].Id

	addSetRespOne, addSetBodyOne := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+workoutID.String()+"/add_set", map[string]any{
		"idempotency_key":     "set-1",
		"workout_exercise_id": workoutExerciseID.String(),
		"reps":                8,
		"weight_kg":           80.0,
		"rpe":                 8.0,
	}, authHeader)
	require.Equal(t, http.StatusCreated, addSetRespOne.StatusCode)

	var firstSetPayload generated.WorkoutSetResponse
	require.NoError(t, json.Unmarshal(addSetBodyOne, &firstSetPayload))
	require.Equal(t, int32(1), firstSetPayload.Set.SetIndex)
	require.Equal(t, int32(8), firstSetPayload.Set.Reps)
	require.Equal(t, float32(80.0), firstSetPayload.Set.WeightKg)

	addSetRespTwo, addSetBodyTwo := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+workoutID.String()+"/add_set", map[string]any{
		"idempotency_key":     "set-2",
		"workout_exercise_id": workoutExerciseID.String(),
		"reps":                6,
		"weight_kg":           85.0,
	}, authHeader)
	require.Equal(t, http.StatusCreated, addSetRespTwo.StatusCode)

	var secondSetPayload generated.WorkoutSetResponse
	require.NoError(t, json.Unmarshal(addSetBodyTwo, &secondSetPayload))
	require.Equal(t, int32(2), secondSetPayload.Set.SetIndex)
	require.Equal(t, int32(6), secondSetPayload.Set.Reps)
	require.Equal(t, float32(85.0), secondSetPayload.Set.WeightKg)

	database := openTestDatabase(t)
	defer database.Close()

	rows, err := database.Query(`
		SELECT set_index, reps, weight_kg
		FROM workout_sets
		WHERE workout_exercise_id = $1
		ORDER BY set_index ASC
	`, uuid.UUID(workoutExerciseID))
	require.NoError(t, err)
	defer rows.Close()

	type persistedSet struct {
		SetIndex int32
		Reps     int32
		WeightKg float64
	}

	sets := []persistedSet{}
	for rows.Next() {
		var item persistedSet
		require.NoError(t, rows.Scan(&item.SetIndex, &item.Reps, &item.WeightKg))
		sets = append(sets, item)
	}
	require.NoError(t, rows.Err())

	require.Len(t, sets, 2)
	require.Equal(t, int32(1), sets[0].SetIndex)
	require.Equal(t, int32(8), sets[0].Reps)
	require.Equal(t, 80.0, sets[0].WeightKg)
	require.Equal(t, int32(2), sets[1].SetIndex)
	require.Equal(t, int32(6), sets[1].Reps)
	require.Equal(t, 85.0, sets[1].WeightKg)
}

func TestWorkoutsAddSetIsIdempotentByClientKeyIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)
	programSessionID := getProgramSessionIDForProgramDay(t, programID, 1)

	authHeader := registerAndAuthHeader(t, server, "workout-idempotency-user@atlas.local")

	startResp, startBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/start", map[string]any{
		"program_session_id": programSessionID.String(),
	}, authHeader)
	require.Equal(t, http.StatusCreated, startResp.StatusCode)

	var startPayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(startBody, &startPayload))
	require.NotEmpty(t, startPayload.Workout.Exercises)

	workoutID := startPayload.Workout.Id
	workoutExerciseID := startPayload.Workout.Exercises[0].Id

	idempotencyKey := "set-duplicate-key"
	addSetRespOne, addSetBodyOne := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+workoutID.String()+"/add_set", map[string]any{
		"idempotency_key":     idempotencyKey,
		"workout_exercise_id": workoutExerciseID.String(),
		"reps":                8,
		"weight_kg":           80.0,
		"rpe":                 8.0,
	}, authHeader)
	require.Equal(t, http.StatusCreated, addSetRespOne.StatusCode)

	var firstSetPayload generated.WorkoutSetResponse
	require.NoError(t, json.Unmarshal(addSetBodyOne, &firstSetPayload))

	addSetRespTwo, addSetBodyTwo := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+workoutID.String()+"/add_set", map[string]any{
		"idempotency_key":     idempotencyKey,
		"workout_exercise_id": workoutExerciseID.String(),
		"reps":                10,
		"weight_kg":           95.0,
	}, authHeader)
	require.Equal(t, http.StatusCreated, addSetRespTwo.StatusCode)

	var secondSetPayload generated.WorkoutSetResponse
	require.NoError(t, json.Unmarshal(addSetBodyTwo, &secondSetPayload))
	require.Equal(t, firstSetPayload.Set.Id, secondSetPayload.Set.Id)
	require.Equal(t, firstSetPayload.Set.SetIndex, secondSetPayload.Set.SetIndex)
	require.Equal(t, firstSetPayload.Set.Reps, secondSetPayload.Set.Reps)
	require.Equal(t, firstSetPayload.Set.WeightKg, secondSetPayload.Set.WeightKg)

	database := openTestDatabase(t)
	defer database.Close()

	var setCount int
	err := database.QueryRow(`
		SELECT COUNT(*)
		FROM workout_sets
		WHERE workout_exercise_id = $1
	`, uuid.UUID(workoutExerciseID)).Scan(&setCount)
	require.NoError(t, err)
	require.Equal(t, 1, setCount)
}

func TestWorkoutCompleteLocksFurtherSetWritesIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	programID := seedProgramFixtures(t)
	programSessionID := getProgramSessionIDForProgramDay(t, programID, 1)

	authHeader := registerAndAuthHeader(t, server, "workout-complete-user@atlas.local")

	startResp, startBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/start", map[string]any{
		"program_session_id": programSessionID.String(),
	}, authHeader)
	require.Equal(t, http.StatusCreated, startResp.StatusCode)

	var startPayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(startBody, &startPayload))
	require.NotEmpty(t, startPayload.Workout.Exercises)

	workoutID := startPayload.Workout.Id
	workoutExerciseID := startPayload.Workout.Exercises[0].Id

	completeResp, completeBody := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+workoutID.String()+"/complete", map[string]any{
		"notes": "Session completed",
	}, authHeader)
	require.Equal(t, http.StatusOK, completeResp.StatusCode)

	var completePayload generated.WorkoutResponse
	require.NoError(t, json.Unmarshal(completeBody, &completePayload))
	require.NotNil(t, completePayload.Workout.CompletedAt)
	require.Equal(t, "Session completed", completePayload.Workout.Notes)

	addSetAfterCompleteResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/workouts/"+workoutID.String()+"/add_set", map[string]any{
		"idempotency_key":     "set-after-complete",
		"workout_exercise_id": workoutExerciseID.String(),
		"reps":                5,
		"weight_kg":           90.0,
	}, authHeader)
	require.Equal(t, http.StatusConflict, addSetAfterCompleteResp.StatusCode)
}

func TestDashboardSummaryIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	email := "dashboard-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)
	userID := lookupUserIDByEmail(t, email)
	nowUTC := time.Now().UTC()
	now := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 12, 0, 0, 0, time.UTC)
	seedAnalyticsWorkoutData(t, userID, now)

	resp, body := doRequest(t, server, http.MethodGet, "/api/v1/dashboard/summary", nil, authHeader)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload generated.DashboardSummaryResponse
	require.NoError(t, json.Unmarshal(body, &payload))

	require.Equal(t, int32(2), payload.Summary.WorkoutsCompletedLast7Days)
	require.Equal(t, int32(3), payload.Summary.TotalSetsLast7Days)
	require.InDelta(t, 860.0, payload.Summary.VolumeByMovementPatternLast7Days["squat"], 0.001)
	require.InDelta(t, 400.0, payload.Summary.VolumeByMovementPatternLast7Days["push"], 0.001)

	require.NotNil(t, payload.Summary.Prs.Squat.Best5RmEstimateKg)
	require.InDelta(t, 113.1429, *payload.Summary.Prs.Squat.Best5RmEstimateKg, 0.001)
	require.Equal(t, int32(3), *payload.Summary.Prs.Squat.BestSetReps)
	require.InDelta(t, 120.0, *payload.Summary.Prs.Squat.BestSetWeightKg, 0.001)

	require.NotNil(t, payload.Summary.Prs.Bench.Best5RmEstimateKg)
	require.InDelta(t, 80.0, *payload.Summary.Prs.Bench.Best5RmEstimateKg, 0.001)

	require.NotNil(t, payload.Summary.Prs.Deadlift.Best5RmEstimateKg)
	require.InDelta(t, 155.4286, *payload.Summary.Prs.Deadlift.Best5RmEstimateKg, 0.001)

	require.NotNil(t, payload.Summary.EstimatedOneRmByLift.Squat.EstimatedOneRmKg)
	require.InDelta(t, 132.0, *payload.Summary.EstimatedOneRmByLift.Squat.EstimatedOneRmKg, 0.001)
	require.Equal(t, int32(3), *payload.Summary.EstimatedOneRmByLift.Squat.BestSetReps)
	require.InDelta(t, 120.0, *payload.Summary.EstimatedOneRmByLift.Squat.BestSetWeightKg, 0.001)

	require.NotNil(t, payload.Summary.EstimatedOneRmByLift.Bench.EstimatedOneRmKg)
	require.InDelta(t, 93.3333, *payload.Summary.EstimatedOneRmByLift.Bench.EstimatedOneRmKg, 0.001)

	require.NotNil(t, payload.Summary.EstimatedOneRmByLift.Deadlift.EstimatedOneRmKg)
	require.InDelta(t, 181.3333, *payload.Summary.EstimatedOneRmByLift.Deadlift.EstimatedOneRmKg, 0.001)

	require.Len(t, payload.Summary.PrEvents, 4)
	require.Equal(t, generated.DashboardPREventLift("squat"), payload.Summary.PrEvents[0].Lift)
	require.NotNil(t, payload.Summary.PrEvents[0].ImprovementKg)
	require.InDelta(t, 15.3333, *payload.Summary.PrEvents[0].ImprovementKg, 0.001)

	require.Len(t, payload.Summary.WeeklyVolumeTrend, 6)
	require.InDelta(t, 1260.0, payload.Summary.WeeklyVolumeTrend[0].TotalVolumeKg, 0.001)
	require.InDelta(t, 340.0, payload.Summary.WeeklyVolumeTrend[3].TotalVolumeKg, 0.001)

	require.Len(t, payload.Summary.WeeklyMuscleGroupVolume, 6)
	currentWeekVolume := payload.Summary.WeeklyMuscleGroupVolume[0].VolumeByMuscleGroup
	require.InDelta(t, 430.0, currentWeekVolume["quads"], 0.001)
	require.InDelta(t, 215.0, currentWeekVolume["glutes"], 0.001)
	require.InDelta(t, 215.0, currentWeekVolume["core"], 0.001)
	require.InDelta(t, 266.6667, currentWeekVolume["chest"], 0.001)
	require.InDelta(t, 133.3333, currentWeekVolume["triceps"], 0.001)

	olderWeekVolume := payload.Summary.WeeklyMuscleGroupVolume[3].VolumeByMuscleGroup
	require.InDelta(t, 170.0, olderWeekVolume["glutes"], 0.001)
	require.InDelta(t, 85.0, olderWeekVolume["hamstrings"], 0.001)
	require.InDelta(t, 85.0, olderWeekVolume["erectors"], 0.001)

	require.Equal(t, int32(2), payload.Summary.AdherenceStreaks.Training.CurrentDays)
	require.Equal(t, int32(2), payload.Summary.AdherenceStreaks.Training.LongestDays)
	require.Equal(t, int32(1), payload.Summary.AdherenceStreaks.Protein.CurrentDays)
	require.Equal(t, int32(1), payload.Summary.AdherenceStreaks.Protein.LongestDays)

	require.Len(t, payload.Summary.WeightTrendPoints, 4)
	require.Equal(t, now.AddDate(0, 0, -21).Format("2006-01-02"), payload.Summary.WeightTrendPoints[0].Date.Time.UTC().Format("2006-01-02"))
	require.InDelta(t, 84.6, payload.Summary.WeightTrendPoints[0].WeightKg, 0.001)
	require.Equal(t, now.Format("2006-01-02"), payload.Summary.WeightTrendPoints[3].Date.Time.UTC().Format("2006-01-02"))

	require.Len(t, payload.Summary.ReadinessSelfReportHistory, 2)
	require.Equal(t, now.AddDate(0, 0, -1).Format("2006-01-02"), payload.Summary.ReadinessSelfReportHistory[1].Date.Time.UTC().Format("2006-01-02"))
	require.InDelta(t, 2.0, payload.Summary.ReadinessSelfReportHistory[1].ReadinessScore, 0.001)
	require.Equal(t, int32(2), payload.Summary.ReadinessSelfReportHistory[1].StressLevel)

	require.InDelta(t, 66.6666, payload.Summary.ProteinAdherenceLast7DaysPercent, 0.01)
}

func TestDashboardReadinessCheckinIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	email := "dashboard-readiness@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)
	userID := lookupUserIDByEmail(t, email)

	createResp, createBody := doRequest(t, server, http.MethodPost, "/api/v1/dashboard/readiness-checkin", map[string]any{
		"date":         "2026-02-27",
		"energyLevel":  3,
		"sleepQuality": 2,
		"stressLevel":  1,
	}, authHeader)
	require.Equalf(t, http.StatusOK, createResp.StatusCode, "body=%s", string(createBody))

	var createPayload generated.DashboardReadinessCheckinResponse
	require.NoError(t, json.Unmarshal(createBody, &createPayload))
	require.Equal(t, int32(3), createPayload.Checkin.EnergyLevel)
	require.InDelta(t, 2.6667, createPayload.Checkin.ReadinessScore, 0.001)

	updateResp, updateBody := doRequest(t, server, http.MethodPost, "/api/v1/dashboard/readiness-checkin", map[string]any{
		"date":         "2026-02-27",
		"energyLevel":  2,
		"sleepQuality": 3,
		"stressLevel":  2,
	}, authHeader)
	require.Equalf(t, http.StatusOK, updateResp.StatusCode, "body=%s", string(updateBody))

	var updatePayload generated.DashboardReadinessCheckinResponse
	require.NoError(t, json.Unmarshal(updateBody, &updatePayload))
	require.Equal(t, int32(2), updatePayload.Checkin.EnergyLevel)
	require.Equal(t, int32(3), updatePayload.Checkin.SleepQuality)
	require.InDelta(t, 2.3333, updatePayload.Checkin.ReadinessScore, 0.001)

	database := openTestDatabase(t)
	defer database.Close()

	var totalRows int
	err := database.QueryRow(`SELECT COUNT(*) FROM readiness_checkins WHERE user_id = $1`, userID).Scan(&totalRows)
	require.NoError(t, err)
	require.Equal(t, 1, totalRows)
}

func TestHabitsCreateEnforcesMaxThreeActiveIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	authHeader := registerAndAuthHeader(t, server, "habits-limit-user@atlas.local")

	for i := 1; i <= 3; i++ {
		createResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/habits", map[string]any{
			"name":        fmt.Sprintf("Habit %d", i),
			"type":        "daily",
			"target_json": map[string]any{"count": 1},
			"active":      true,
		}, authHeader)
		require.Equal(t, http.StatusCreated, createResp.StatusCode)
	}

	createFourthActiveResp, createFourthActiveBody := doRequest(t, server, http.MethodPost, "/api/v1/habits", map[string]any{
		"name":        "Habit 4",
		"type":        "daily",
		"target_json": map[string]any{"count": 1},
		"active":      true,
	}, authHeader)
	require.Equal(t, http.StatusConflict, createFourthActiveResp.StatusCode)

	var errorPayload generated.ErrorResponse
	require.NoError(t, json.Unmarshal(createFourthActiveBody, &errorPayload))
	require.Equal(t, "max 3 active habits allowed", errorPayload.Message)

	createInactiveResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/habits", map[string]any{
		"name":        "Habit inactive",
		"type":        "daily",
		"target_json": map[string]any{"count": 1},
		"active":      false,
	}, authHeader)
	require.Equal(t, http.StatusCreated, createInactiveResp.StatusCode)
}

func TestHabitsListResolvesCompletionByUTCDateKeyIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "habits-date-key-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	createResp, createBody := doRequest(t, server, http.MethodPost, "/api/v1/habits", map[string]any{
		"name":        "Hydration",
		"type":        "daily",
		"target_json": map[string]any{"count": 1},
		"active":      true,
	}, authHeader)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createdHabit generated.HabitResponse
	require.NoError(t, json.Unmarshal(createBody, &createdHabit))

	todayKey := time.Now().UTC().Format("2006-01-02")
	yesterdayKey := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	beforeResp, beforeBody := doRequest(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/api/v1/habits?date=%s", todayKey),
		nil,
		authHeader,
	)
	require.Equal(t, http.StatusOK, beforeResp.StatusCode)

	var beforePayload generated.HabitsResponse
	require.NoError(t, json.Unmarshal(beforeBody, &beforePayload))
	require.Len(t, beforePayload.Habits, 1)
	require.False(t, beforePayload.Habits[0].Completed)

	toggleResp, toggleBody := doRequest(
		t,
		server,
		http.MethodPost,
		fmt.Sprintf("/api/v1/habits/%s/toggle_today", createdHabit.Habit.Id),
		nil,
		authHeader,
	)
	require.Equal(t, http.StatusOK, toggleResp.StatusCode)

	var togglePayload generated.HabitToggleTodayResponse
	require.NoError(t, json.Unmarshal(toggleBody, &togglePayload))
	require.Equal(t, todayKey, togglePayload.Log.Date.String())
	require.True(t, togglePayload.Log.Completed)

	todayResp, todayBody := doRequest(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/api/v1/habits?date=%s", todayKey),
		nil,
		authHeader,
	)
	require.Equal(t, http.StatusOK, todayResp.StatusCode)

	var todayPayload generated.HabitsResponse
	require.NoError(t, json.Unmarshal(todayBody, &todayPayload))
	require.Len(t, todayPayload.Habits, 1)
	require.True(t, todayPayload.Habits[0].Completed)

	yesterdayResp, yesterdayBody := doRequest(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/api/v1/habits?date=%s", yesterdayKey),
		nil,
		authHeader,
	)
	require.Equal(t, http.StatusOK, yesterdayResp.StatusCode)

	var yesterdayPayload generated.HabitsResponse
	require.NoError(t, json.Unmarshal(yesterdayBody, &yesterdayPayload))
	require.Len(t, yesterdayPayload.Habits, 1)
	require.False(t, yesterdayPayload.Habits[0].Completed)

	userID := lookupUserIDByEmail(t, email)
	database := openTestDatabase(t)
	defer database.Close()

	var rowCount int
	err := database.QueryRow(`
		SELECT COUNT(*)
		FROM habit_daily_logs hdl
		JOIN habits h ON h.id = hdl.habit_id
		WHERE h.user_id = $1
		  AND hdl.date = $2::date
	`, userID, todayKey).Scan(&rowCount)
	require.NoError(t, err)
	require.Equal(t, 1, rowCount)
}

func TestMomentumSprintEnrollAndStatusIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "momentum-sprint-status-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	enrollResp, enrollBody := doRequest(t, server, http.MethodPost, "/api/v1/momentum-sprint/enroll", map[string]any{
		"goal": "Build strength",
	}, authHeader)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	var enrollPayload generated.MomentumSprintStatusResponse
	require.NoError(t, json.Unmarshal(enrollBody, &enrollPayload))
	require.True(t, enrollPayload.Enrolled)
	require.NotNil(t, enrollPayload.Enrollment)
	require.NotNil(t, enrollPayload.Progress)
	require.Equal(t, "build_strength", enrollPayload.Enrollment.Goal)
	require.Len(t, enrollPayload.TodayChecklist, 3)
	require.Len(t, enrollPayload.Milestones, 3)
	require.Equal(t, int32(14), enrollPayload.Progress.TotalDays)
	require.Equal(t, int32(0), enrollPayload.Progress.CompletedDays)

	getResp, getBody := doRequest(t, server, http.MethodGet, "/api/v1/momentum-sprint/status", nil, authHeader)
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var statusPayload generated.MomentumSprintStatusResponse
	require.NoError(t, json.Unmarshal(getBody, &statusPayload))
	require.True(t, statusPayload.Enrolled)
	require.NotNil(t, statusPayload.Enrollment)
	require.NotNil(t, statusPayload.Progress)
	require.Equal(t, enrollPayload.Enrollment.Id, statusPayload.Enrollment.Id)
	require.Len(t, statusPayload.TodayChecklist, 3)
	require.Len(t, statusPayload.Milestones, 3)

	userID := lookupUserIDByEmail(t, email)
	database := openTestDatabase(t)
	defer database.Close()

	var checklistEntryCount int
	require.NoError(t, database.QueryRow(`
		SELECT COUNT(*)
		FROM momentum_sprint_daily_checklist_entries entries
		JOIN momentum_sprint_enrollments enrollments ON enrollments.id = entries.enrollment_id
		WHERE enrollments.user_id = $1
	`, userID).Scan(&checklistEntryCount))
	require.Equal(t, 42, checklistEntryCount)
}

func TestMomentumSprintDayCompleteUpdatesProgressIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	authHeader := registerAndAuthHeader(t, server, "momentum-sprint-complete-user@atlas.local")

	enrollResp, enrollBody := doRequest(t, server, http.MethodPost, "/api/v1/momentum-sprint/enroll", map[string]any{
		"goal": "Build strength",
	}, authHeader)
	require.Equal(t, http.StatusOK, enrollResp.StatusCode)

	var enrollPayload generated.MomentumSprintStatusResponse
	require.NoError(t, json.Unmarshal(enrollBody, &enrollPayload))
	require.Len(t, enrollPayload.TodayChecklist, 3)

	for _, entry := range enrollPayload.TodayChecklist {
		completeResp, completeBody := doRequest(t, server, http.MethodPost, "/api/v1/momentum-sprint/day-complete", map[string]any{
			"habitKey":  entry.HabitKey,
			"completed": true,
		}, authHeader)
		require.Equal(t, http.StatusOK, completeResp.StatusCode)

		var payload generated.MomentumSprintStatusResponse
		require.NoError(t, json.Unmarshal(completeBody, &payload))
		require.True(t, payload.Enrolled)
		require.NotNil(t, payload.Progress)
		require.Len(t, payload.TodayChecklist, 3)
	}

	statusResp, statusBody := doRequest(t, server, http.MethodGet, "/api/v1/momentum-sprint/status", nil, authHeader)
	require.Equal(t, http.StatusOK, statusResp.StatusCode)

	var statusPayload generated.MomentumSprintStatusResponse
	require.NoError(t, json.Unmarshal(statusBody, &statusPayload))
	require.NotNil(t, statusPayload.Progress)
	require.True(t, statusPayload.Progress.CompletedToday)
	require.Equal(t, int32(1), statusPayload.Progress.CompletedDays)
	require.Equal(t, int32(1), statusPayload.Progress.CurrentStreak)
	require.Equal(t, int32(1), statusPayload.Progress.LongestStreak)

	for _, entry := range statusPayload.TodayChecklist {
		require.True(t, entry.Completed)
	}
	for _, milestone := range statusPayload.Milestones {
		require.False(t, milestone.Unlocked)
	}
}

func TestNutritionTargetsAndCheckinCRUDIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "nutrition-crud-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	targetsResp, targetsBody := doRequest(t, server, http.MethodPut, "/api/v1/nutrition/targets", map[string]any{
		"calories_target":  2200,
		"protein_g_target": 160,
	}, authHeader)
	require.Equal(t, http.StatusOK, targetsResp.StatusCode)

	var targetsPayload generated.NutritionTargetsResponse
	require.NoError(t, json.Unmarshal(targetsBody, &targetsPayload))
	require.Equal(t, int32(2200), targetsPayload.Targets.CaloriesTarget)
	require.Equal(t, int32(160), targetsPayload.Targets.ProteinGTarget)

	todayRespBefore, todayBodyBefore := doRequest(t, server, http.MethodGet, "/api/v1/nutrition/today", nil, authHeader)
	require.Equal(t, http.StatusOK, todayRespBefore.StatusCode)

	var todayBefore generated.NutritionTodayResponse
	require.NoError(t, json.Unmarshal(todayBodyBefore, &todayBefore))
	require.True(t, todayBefore.TargetsConfigured)
	require.NotNil(t, todayBefore.Targets)
	require.Nil(t, todayBefore.Checkin)

	checkinRespOne, checkinBodyOne := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/checkin", map[string]any{
		"calories_estimate":  2100,
		"protein_g_estimate": 170,
		"notes":              "first pass",
	}, authHeader)
	require.Equal(t, http.StatusOK, checkinRespOne.StatusCode)

	var checkinPayloadOne generated.NutritionCheckinResponse
	require.NoError(t, json.Unmarshal(checkinBodyOne, &checkinPayloadOne))
	require.True(t, checkinPayloadOne.Checkin.HitCalories)
	require.True(t, checkinPayloadOne.Checkin.HitProtein)
	require.Equal(t, "first pass", checkinPayloadOne.Checkin.Notes)

	checkinRespTwo, checkinBodyTwo := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/checkin", map[string]any{
		"calories_estimate":  2300,
		"protein_g_estimate": 140,
		"notes":              "updated",
	}, authHeader)
	require.Equal(t, http.StatusOK, checkinRespTwo.StatusCode)

	var checkinPayloadTwo generated.NutritionCheckinResponse
	require.NoError(t, json.Unmarshal(checkinBodyTwo, &checkinPayloadTwo))
	require.Equal(t, checkinPayloadOne.Checkin.Id, checkinPayloadTwo.Checkin.Id)
	require.False(t, checkinPayloadTwo.Checkin.HitCalories)
	require.False(t, checkinPayloadTwo.Checkin.HitProtein)
	require.Equal(t, "updated", checkinPayloadTwo.Checkin.Notes)

	userID := lookupUserIDByEmail(t, email)
	database := openTestDatabase(t)
	defer database.Close()

	today := time.Now().UTC().Format("2006-01-02")
	var rowCount int
	err := database.QueryRow(`
		SELECT COUNT(*)
		FROM nutrition_daily_checkins
		WHERE user_id = $1
		  AND date = $2::date
	`, userID, today).Scan(&rowCount)
	require.NoError(t, err)
	require.Equal(t, 1, rowCount)

	todayRespAfter, todayBodyAfter := doRequest(t, server, http.MethodGet, "/api/v1/nutrition/today", nil, authHeader)
	require.Equal(t, http.StatusOK, todayRespAfter.StatusCode)

	var todayAfter generated.NutritionTodayResponse
	require.NoError(t, json.Unmarshal(todayBodyAfter, &todayAfter))
	require.True(t, todayAfter.TargetsConfigured)
	require.NotNil(t, todayAfter.Checkin)
	require.False(t, todayAfter.Checkin.HitCalories)
	require.False(t, todayAfter.Checkin.HitProtein)
	require.Equal(t, "updated", todayAfter.Checkin.Notes)
}

func TestNutritionCheckinDateBehaviorIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "nutrition-date-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	targetsResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/nutrition/targets", map[string]any{
		"calories_target":  2400,
		"protein_g_target": 170,
	}, authHeader)
	require.Equal(t, http.StatusOK, targetsResp.StatusCode)

	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	yesterdayCheckinResp, yesterdayCheckinBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/nutrition/checkin",
		map[string]any{
			"date":               yesterday,
			"calories_estimate":  2300,
			"protein_g_estimate": 180,
		},
		authHeader,
	)
	require.Equal(t, http.StatusOK, yesterdayCheckinResp.StatusCode)

	var yesterdayCheckinPayload generated.NutritionCheckinResponse
	require.NoError(t, json.Unmarshal(yesterdayCheckinBody, &yesterdayCheckinPayload))
	require.Equal(t, yesterday, yesterdayCheckinPayload.Checkin.Date.String())

	todayResp, todayBody := doRequest(t, server, http.MethodGet, "/api/v1/nutrition/today", nil, authHeader)
	require.Equal(t, http.StatusOK, todayResp.StatusCode)

	var todayPayload generated.NutritionTodayResponse
	require.NoError(t, json.Unmarshal(todayBody, &todayPayload))
	require.Nil(t, todayPayload.Checkin)

	todayCheckinResp, todayCheckinBody := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/checkin", map[string]any{
		"calories_estimate":  2600,
		"protein_g_estimate": 150,
	}, authHeader)
	require.Equal(t, http.StatusOK, todayCheckinResp.StatusCode)

	var todayCheckinPayload generated.NutritionCheckinResponse
	require.NoError(t, json.Unmarshal(todayCheckinBody, &todayCheckinPayload))
	require.Equal(t, time.Now().UTC().Format("2006-01-02"), todayCheckinPayload.Checkin.Date.String())

	userID := lookupUserIDByEmail(t, email)
	database := openTestDatabase(t)
	defer database.Close()

	var totalRows int
	err := database.QueryRow(`SELECT COUNT(*) FROM nutrition_daily_checkins WHERE user_id = $1`, userID).Scan(&totalRows)
	require.NoError(t, err)
	require.Equal(t, 2, totalRows)
}

func TestNutritionWeightEntryUpsertAndTrendIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "nutrition-weight-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	startOfUTCWeek := func(value time.Time) time.Time {
		day := time.Date(value.UTC().Year(), value.UTC().Month(), value.UTC().Day(), 0, 0, 0, 0, time.UTC)
		weekday := int(day.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return day.AddDate(0, 0, -(weekday - 1))
	}

	now := time.Now().UTC()
	currentWeekStart := startOfUTCWeek(now)
	currentEntryDate := currentWeekStart.AddDate(0, 0, 3).Format("2006-01-02")
	currentLatestDate := currentWeekStart.AddDate(0, 0, 4).Format("2006-01-02")
	twoWeeksAgoDate := currentWeekStart.AddDate(0, 0, -12).Format("2006-01-02")

	postResp, postBody := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/weight", map[string]any{
		"date":   currentEntryDate,
		"weight": 180.0,
		"unit":   "lb",
	}, authHeader)
	require.Equal(t, http.StatusOK, postResp.StatusCode)

	var postPayload generated.NutritionWeightEntryResponse
	require.NoError(t, json.Unmarshal(postBody, &postPayload))
	require.Equal(t, generated.WeightUnit("lb"), postPayload.Entry.Unit)
	require.Equal(t, currentEntryDate, postPayload.Entry.Date.String())
	require.InDelta(t, 180.0, postPayload.Entry.Weight, 0.01)
	require.InDelta(t, 81.6466, postPayload.Entry.WeightKg, 0.01)

	putResp, putBody := doRequest(t, server, http.MethodPut, "/api/v1/nutrition/weight", map[string]any{
		"date":   currentEntryDate,
		"weight": 182.0,
		"unit":   "lb",
	}, authHeader)
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	var putPayload generated.NutritionWeightEntryResponse
	require.NoError(t, json.Unmarshal(putBody, &putPayload))
	require.Equal(t, generated.WeightUnit("lb"), putPayload.Entry.Unit)
	require.Equal(t, currentEntryDate, putPayload.Entry.Date.String())
	require.InDelta(t, 182.0, putPayload.Entry.Weight, 0.01)
	require.InDelta(t, 82.5538, putPayload.Entry.WeightKg, 0.01)

	latestResp, latestBody := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/weight", map[string]any{
		"date":   currentLatestDate,
		"weight": 81.0,
		"unit":   "kg",
	}, authHeader)
	require.Equal(t, http.StatusOK, latestResp.StatusCode)

	var latestPayload generated.NutritionWeightEntryResponse
	require.NoError(t, json.Unmarshal(latestBody, &latestPayload))
	require.Equal(t, generated.WeightUnit("kg"), latestPayload.Entry.Unit)
	require.Equal(t, currentLatestDate, latestPayload.Entry.Date.String())
	require.InDelta(t, 81.0, latestPayload.Entry.Weight, 0.01)

	olderResp, olderBody := doRequest(t, server, http.MethodPut, "/api/v1/nutrition/weight", map[string]any{
		"date":   twoWeeksAgoDate,
		"weight": 176.0,
		"unit":   "lb",
	}, authHeader)
	require.Equal(t, http.StatusOK, olderResp.StatusCode)

	var olderPayload generated.NutritionWeightEntryResponse
	require.NoError(t, json.Unmarshal(olderBody, &olderPayload))
	require.Equal(t, generated.WeightUnit("lb"), olderPayload.Entry.Unit)
	require.Equal(t, twoWeeksAgoDate, olderPayload.Entry.Date.String())

	userID := lookupUserIDByEmail(t, email)
	database := openTestDatabase(t)
	defer database.Close()

	var rowCount int
	err := database.QueryRow(`
		SELECT COUNT(*)
		FROM user_weight_entries
		WHERE user_id = $1
	`, userID).Scan(&rowCount)
	require.NoError(t, err)
	require.Equal(t, 3, rowCount)

	trendResp, trendBody := doRequest(t, server, http.MethodGet, "/api/v1/nutrition/weight/trend", nil, authHeader)
	require.Equal(t, http.StatusOK, trendResp.StatusCode)

	var trendPayload generated.NutritionWeightTrendResponse
	require.NoError(t, json.Unmarshal(trendBody, &trendPayload))
	require.Len(t, trendPayload.Points, 8)

	pointsByWeek := make(map[string]generated.NutritionWeightTrendPoint, len(trendPayload.Points))
	for _, point := range trendPayload.Points {
		pointsByWeek[point.WeekStartDate.String()] = point
	}

	currentWeekKey := currentWeekStart.Format("2006-01-02")
	currentWeekPoint, ok := pointsByWeek[currentWeekKey]
	require.True(t, ok)
	require.NotNil(t, currentWeekPoint.EntryDate)
	require.Equal(t, currentLatestDate, currentWeekPoint.EntryDate.String())
	require.NotNil(t, currentWeekPoint.Unit)
	require.Equal(t, generated.WeightUnit("kg"), *currentWeekPoint.Unit)
	require.NotNil(t, currentWeekPoint.Weight)
	require.InDelta(t, 81.0, *currentWeekPoint.Weight, 0.01)

	twoWeeksAgoKey := currentWeekStart.AddDate(0, 0, -14).Format("2006-01-02")
	twoWeeksAgoPoint, ok := pointsByWeek[twoWeeksAgoKey]
	require.True(t, ok)
	require.NotNil(t, twoWeeksAgoPoint.EntryDate)
	require.Equal(t, twoWeeksAgoDate, twoWeeksAgoPoint.EntryDate.String())
	require.NotNil(t, twoWeeksAgoPoint.Unit)
	require.Equal(t, generated.WeightUnit("lb"), *twoWeeksAgoPoint.Unit)
	require.NotNil(t, twoWeeksAgoPoint.Weight)
	require.InDelta(t, 176.0, *twoWeeksAgoPoint.Weight, 0.01)

	oneWeekAgoKey := currentWeekStart.AddDate(0, 0, -7).Format("2006-01-02")
	oneWeekAgoPoint, ok := pointsByWeek[oneWeekAgoKey]
	require.True(t, ok)
	require.Nil(t, oneWeekAgoPoint.EntryDate)
	require.Nil(t, oneWeekAgoPoint.Unit)
	require.Nil(t, oneWeekAgoPoint.Weight)
	require.Nil(t, oneWeekAgoPoint.WeightKg)
}

func TestNutritionWeeklyCheckinAdjustmentIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "nutrition-weekly-checkin-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)
	activateSubscription(t, server, authHeader, map[string]any{
		"platform":      "ios",
		"productId":     "atlas.pro.monthly",
		"receiptToken":  "rcpt-weekly-checkin-pro-1",
		"transactionId": "tx-weekly-checkin-pro-1",
	})

	goalsResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/goals", map[string]any{
		"primaryGoal":            "fat_loss",
		"daysPerWeek":            4,
		"sessionDurationMinutes": 60,
		"equipmentAccessJson":    []string{"barbell"},
		"constraintsJson":        map[string]any{},
	}, authHeader)
	require.Equal(t, http.StatusOK, goalsResp.StatusCode)

	targetsResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/nutrition/targets", map[string]any{
		"calories_target":  2300,
		"protein_g_target": 165,
	}, authHeader)
	require.Equal(t, http.StatusOK, targetsResp.StatusCode)

	startOfUTCWeek := func(value time.Time) time.Time {
		day := time.Date(value.UTC().Year(), value.UTC().Month(), value.UTC().Day(), 0, 0, 0, 0, time.UTC)
		weekday := int(day.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return day.AddDate(0, 0, -(weekday - 1))
	}

	now := time.Now().UTC()
	weekStart := startOfUTCWeek(now)
	previousWeekDate := weekStart.AddDate(0, 0, -2).Format("2006-01-02")
	currentWeekDate := weekStart.AddDate(0, 0, 2).Format("2006-01-02")

	for dayOffset := 0; dayOffset < 5; dayOffset++ {
		checkinDate := weekStart.AddDate(0, 0, dayOffset).Format("2006-01-02")
		dailyResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/checkin", map[string]any{
			"date":               checkinDate,
			"calories_estimate":  2100,
			"protein_g_estimate": 165,
		}, authHeader)
		require.Equal(t, http.StatusOK, dailyResp.StatusCode)
	}

	previousWeightResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/weight", map[string]any{
		"date":   previousWeekDate,
		"weight": 80.0,
		"unit":   "kg",
	}, authHeader)
	require.Equal(t, http.StatusOK, previousWeightResp.StatusCode)

	currentWeightResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/weight", map[string]any{
		"date":   currentWeekDate,
		"weight": 80.3,
		"unit":   "kg",
	}, authHeader)
	require.Equal(t, http.StatusOK, currentWeightResp.StatusCode)

	weeklyResp, weeklyBody := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/weekly-checkin", map[string]any{
		"week_start": weekStart.Format("2006-01-02"),
	}, authHeader)
	require.Equal(t, http.StatusOK, weeklyResp.StatusCode)

	var weeklyPayload generated.NutritionWeeklyCheckinResponse
	require.NoError(t, json.Unmarshal(weeklyBody, &weeklyPayload))
	require.Equal(t, weekStart.Format("2006-01-02"), weeklyPayload.Checkin.WeekStart.String())
	require.Equal(t, int32(2150), weeklyPayload.Checkin.NewTargets.CaloriesTarget)
	require.Equal(t, int32(175), weeklyPayload.Checkin.NewTargets.ProteinGTarget)
	require.Equal(t, int32(2300), weeklyPayload.Checkin.PreviousTargets.CaloriesTarget)
	require.Equal(t, int32(165), weeklyPayload.Checkin.PreviousTargets.ProteinGTarget)
	require.Equal(t, int32(-150), weeklyPayload.Checkin.CalorieDelta)
	require.InDelta(t, -0.4, weeklyPayload.Checkin.GoalPaceKgPerWeek, 0.0001)
	require.NotEmpty(t, strings.TrimSpace(weeklyPayload.Checkin.Explanation))

	latestResp, latestBody := doRequest(t, server, http.MethodGet, "/api/v1/nutrition/weekly-checkin/latest", nil, authHeader)
	require.Equal(t, http.StatusOK, latestResp.StatusCode)

	var latestPayload generated.NutritionWeeklyCheckinResponse
	require.NoError(t, json.Unmarshal(latestBody, &latestPayload))
	require.Equal(t, weeklyPayload.Checkin.WeekStart.String(), latestPayload.Checkin.WeekStart.String())
	require.Equal(t, weeklyPayload.Checkin.NewTargets.CaloriesTarget, latestPayload.Checkin.NewTargets.CaloriesTarget)
	require.Equal(t, weeklyPayload.Checkin.PreviousTargets.CaloriesTarget, latestPayload.Checkin.PreviousTargets.CaloriesTarget)
}

func TestNutritionMealPlanGenerationFlowIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "nutrition-meal-plan-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)
	activateSubscription(t, server, authHeader, map[string]any{
		"platform":      "ios",
		"productId":     "atlas.pro.monthly",
		"receiptToken":  "rcpt-meal-plan-generate-pro-1",
		"transactionId": "tx-meal-plan-generate-pro-1",
	})

	targetsResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/nutrition/targets", map[string]any{
		"calories_target":  2400,
		"protein_g_target": 170,
	}, authHeader)
	require.Equal(t, http.StatusOK, targetsResp.StatusCode)

	seedMealPlanRecipes(t)

	generateResp, generateBody := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/meal-plan/generate", map[string]any{}, authHeader)
	require.Equal(t, http.StatusOK, generateResp.StatusCode)

	var generatePayload generated.NutritionMealPlanResponse
	require.NoError(t, json.Unmarshal(generateBody, &generatePayload))
	require.NotEqual(t, uuid.Nil, uuid.UUID(generatePayload.MealPlan.Id))
	require.Len(t, generatePayload.MealPlan.Items, 21)
	require.NotEmpty(t, generatePayload.MealPlan.GroceryItems)
	require.Equal(t, int32(2400), generatePayload.MealPlan.Targets.CaloriesTarget)
	require.Equal(t, int32(160), generatePayload.MealPlan.Targets.ProteinGTarget)

	latestResp, latestBody := doRequest(t, server, http.MethodGet, "/api/v1/nutrition/meal-plan/latest", nil, authHeader)
	require.Equal(t, http.StatusOK, latestResp.StatusCode)

	var latestPayload generated.NutritionMealPlanResponse
	require.NoError(t, json.Unmarshal(latestBody, &latestPayload))
	require.Equal(t, generatePayload.MealPlan.Id, latestPayload.MealPlan.Id)
	require.Len(t, latestPayload.MealPlan.Items, 21)
	require.NotEmpty(t, latestPayload.MealPlan.GroceryItems)

	foundOats := false
	for _, groceryItem := range latestPayload.MealPlan.GroceryItems {
		if strings.EqualFold(groceryItem.Name, "rolled oats") {
			foundOats = true
			require.Greater(t, groceryItem.Quantity, float32(0))
			break
		}
	}
	require.True(t, foundOats, "expected rolled oats in grocery list")
}

func TestNutritionMealPlanCrudAndGroceryRegenerationIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "nutrition-meal-plan-crud-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)
	activateSubscription(t, server, authHeader, map[string]any{
		"platform":      "ios",
		"productId":     "atlas.pro.monthly",
		"receiptToken":  "rcpt-meal-plan-crud-pro-1",
		"transactionId": "tx-meal-plan-crud-pro-1",
	})

	targetsResp, _ := doRequest(t, server, http.MethodPut, "/api/v1/nutrition/targets", map[string]any{
		"calories_target":  2400,
		"protein_g_target": 170,
	}, authHeader)
	require.Equal(t, http.StatusOK, targetsResp.StatusCode)

	seedMealPlanRecipes(t)

	generateResp, generateBody := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/meal-plan/generate", map[string]any{}, authHeader)
	require.Equal(t, http.StatusOK, generateResp.StatusCode)

	var generatePayload generated.NutritionMealPlanResponse
	require.NoError(t, json.Unmarshal(generateBody, &generatePayload))

	weekStart := generatePayload.MealPlan.WeekStart.String()
	require.NotEmpty(t, weekStart)

	oatsRecipeID := ""
	for _, item := range generatePayload.MealPlan.Items {
		if item.Recipe.Slug == "overnight-oats-protein" {
			oatsRecipeID = uuid.UUID(item.Recipe.Id).String()
			break
		}
	}
	require.NotEmpty(t, oatsRecipeID)

	upsertResp, upsertBody := doRequest(t, server, http.MethodPut, "/api/v1/nutrition/meal-plan", map[string]any{
		"week_start": weekStart,
		"items": []map[string]any{
			{
				"day_of_week": 1,
				"meal_slot":   "breakfast",
				"recipe_id":   oatsRecipeID,
				"servings":    2.0,
			},
		},
	}, authHeader)
	require.Equal(t, http.StatusOK, upsertResp.StatusCode)

	var upsertPayload generated.NutritionMealPlanResponse
	require.NoError(t, json.Unmarshal(upsertBody, &upsertPayload))
	require.Len(t, upsertPayload.MealPlan.Items, 1)

	foundOats := false
	for _, groceryItem := range upsertPayload.MealPlan.GroceryItems {
		if strings.EqualFold(groceryItem.Name, "rolled oats") {
			foundOats = true
			require.InDelta(t, 120.0, groceryItem.Quantity, 0.05)
		}
	}
	require.True(t, foundOats, "expected rolled oats grocery aggregation after meal plan upsert")

	getResp, getBody := doRequest(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/api/v1/nutrition/meal-plan?week_start=%s", weekStart),
		nil,
		authHeader,
	)
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var getPayload generated.NutritionMealPlanResponse
	require.NoError(t, json.Unmarshal(getBody, &getPayload))
	require.Len(t, getPayload.MealPlan.Items, 1)
	require.Equal(t, upsertPayload.MealPlan.Id, getPayload.MealPlan.Id)

	deleteResp, _ := doRequest(
		t,
		server,
		http.MethodDelete,
		fmt.Sprintf("/api/v1/nutrition/meal-plan?week_start=%s", weekStart),
		nil,
		authHeader,
	)
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	missingResp, _ := doRequest(
		t,
		server,
		http.MethodGet,
		fmt.Sprintf("/api/v1/nutrition/meal-plan?week_start=%s", weekStart),
		nil,
		authHeader,
	)
	require.Equal(t, http.StatusNotFound, missingResp.StatusCode)
}

func TestNutritionRecipeImportParseAndConfirmIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "nutrition-recipe-import-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	sourceURL := "https://example.com/recipes/high-protein-oats"
	previewResp, previewBody := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/recipes/import", map[string]any{
		"source_url": sourceURL,
	}, authHeader)
	require.Equal(t, http.StatusOK, previewResp.StatusCode)

	var previewPayload generated.NutritionRecipeImportResponse
	require.NoError(t, json.Unmarshal(previewBody, &previewPayload))
	require.False(t, previewPayload.Confirmed)
	require.Nil(t, previewPayload.Recipe)
	require.Equal(t, "high-protein-oats", previewPayload.Draft.Slug)
	require.Equal(t, "High Protein Oats", previewPayload.Draft.Name)

	confirmResp, confirmBody := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/recipes/import", map[string]any{
		"source_url": sourceURL,
		"confirm":    true,
		"draft": map[string]any{
			"slug":          "high-protein-oats",
			"name":          "High Protein Oats",
			"meal_type":     "breakfast",
			"description":   "Atlas import integration fixture.",
			"servings":      2,
			"calories_kcal": 520,
			"protein_g":     35,
			"carbs_g":       62,
			"fat_g":         14,
			"ingredients": []map[string]any{
				{
					"name":     "rolled oats",
					"quantity": 80,
					"unit":     "g",
					"category": "grains",
				},
				{
					"name":     "greek yogurt",
					"quantity": 150,
					"unit":     "g",
					"category": "dairy",
				},
			},
			"instructions": []string{
				"Mix ingredients in a bowl.",
				"Chill for 10 minutes before serving.",
			},
		},
	}, authHeader)
	require.Equal(t, http.StatusOK, confirmResp.StatusCode)

	var confirmPayload generated.NutritionRecipeImportResponse
	require.NoError(t, json.Unmarshal(confirmBody, &confirmPayload))
	require.True(t, confirmPayload.Confirmed)
	require.NotNil(t, confirmPayload.Recipe)
	require.Equal(t, "high-protein-oats", confirmPayload.Recipe.Slug)
	require.Equal(t, int32(520), confirmPayload.Recipe.CaloriesKcal)
	require.Equal(t, "breakfast", confirmPayload.Recipe.MealType)

	database := openTestDatabase(t)
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var persistedCalories int32
	err := database.QueryRowContext(ctx, `
		SELECT calories_kcal
		FROM recipes
		WHERE slug = $1
	`, "high-protein-oats").Scan(&persistedCalories)
	require.NoError(t, err)
	require.Equal(t, int32(520), persistedCalories)
}

func TestFoodLogsCreateAndListIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "food-logs-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)
	userID := lookupUserIDByEmail(t, email)

	database := openTestDatabase(t)
	defer database.Close()

	queries := db.New(database)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	foodNutrients := `{"calories_kcal":100,"protein_g":8.5,"carbs_g":12.0}`
	foodRow, err := queries.UpsertFood(ctx, db.UpsertFoodParams{
		ExternalID:    "fixture-food-1",
		Provider:      "fixture",
		Label:         "Fixture Oatmeal",
		Brand:         "Atlas Test",
		NutrientsJson: []byte(foodNutrients),
	})
	require.NoError(t, err)

	createResp, createBody := doRequest(t, server, http.MethodPost, "/api/v1/food-logs", map[string]any{
		"foodId":   foodRow.ID.String(),
		"quantity": 1.5,
		"unit":     "serving",
	}, authHeader)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createPayload generated.FoodLogResponse
	require.NoError(t, json.Unmarshal(createBody, &createPayload))
	require.Equal(t, uuid.UUID(createPayload.Log.FoodId), foodRow.ID)
	require.InDelta(t, 1.5, createPayload.Log.Quantity, 0.001)
	require.NotNil(t, createPayload.Log.NutrientsSnapshot.CaloriesKcal)
	require.InDelta(t, 150.0, *createPayload.Log.NutrientsSnapshot.CaloriesKcal, 0.001)
	require.NotNil(t, createPayload.Log.NutrientsSnapshot.ProteinG)
	require.InDelta(t, 12.75, *createPayload.Log.NutrientsSnapshot.ProteinG, 0.001)
	require.Nil(t, createPayload.Log.NutrientsSnapshot.FatG)

	var rowCount int
	err = database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM food_logs
		WHERE user_id = $1
	`, userID).Scan(&rowCount)
	require.NoError(t, err)
	require.Equal(t, 1, rowCount)

	listResp, listBody := doRequest(t, server, http.MethodGet, "/api/v1/food-logs?date="+time.Now().UTC().Format("2006-01-02"), nil, authHeader)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listPayload generated.FoodLogsResponse
	require.NoError(t, json.Unmarshal(listBody, &listPayload))
	require.Equal(t, time.Now().UTC().Format("2006-01-02"), listPayload.Date.String())
	require.Len(t, listPayload.Logs, 1)
	require.Equal(t, "Fixture Oatmeal", listPayload.Logs[0].Food.Label)
	require.InDelta(t, 150.0, listPayload.Totals.CaloriesKcal, 0.001)
	require.InDelta(t, 12.75, listPayload.Totals.ProteinG, 0.001)
	require.InDelta(t, 18.0, listPayload.Totals.CarbsG, 0.001)
	require.InDelta(t, 0.0, listPayload.Totals.FatG, 0.001)
}

func TestFoodLogsDailyTotalsUseNutrientSnapshotsIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	email := "food-logs-snapshot-user@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)

	database := openTestDatabase(t)
	defer database.Close()

	queries := db.New(database)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	foodRow, err := queries.UpsertFood(ctx, db.UpsertFoodParams{
		ExternalID:    "snapshot-food-1",
		Provider:      "fixture",
		Label:         "Snapshot Yogurt",
		Brand:         "Atlas Test",
		NutrientsJson: []byte(`{"calories_kcal":100,"protein_g":10,"carbs_g":5,"fat_g":1}`),
	})
	require.NoError(t, err)

	createResp, createBody := doRequest(t, server, http.MethodPost, "/api/v1/food-logs", map[string]any{
		"foodId":   foodRow.ID.String(),
		"quantity": 1.0,
		"unit":     "serving",
	}, authHeader)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createPayload generated.FoodLogResponse
	require.NoError(t, json.Unmarshal(createBody, &createPayload))
	require.InDelta(t, 100.0, *createPayload.Log.NutrientsSnapshot.CaloriesKcal, 0.001)
	require.InDelta(t, 10.0, *createPayload.Log.NutrientsSnapshot.ProteinG, 0.001)

	_, err = queries.UpsertFood(ctx, db.UpsertFoodParams{
		ExternalID:    "snapshot-food-1",
		Provider:      "fixture",
		Label:         "Snapshot Yogurt Updated",
		Brand:         "Atlas Test",
		NutrientsJson: []byte(`{"calories_kcal":300,"protein_g":30,"carbs_g":20,"fat_g":8}`),
	})
	require.NoError(t, err)

	listResp, listBody := doRequest(t, server, http.MethodGet, "/api/v1/food-logs?date="+time.Now().UTC().Format("2006-01-02"), nil, authHeader)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listPayload generated.FoodLogsResponse
	require.NoError(t, json.Unmarshal(listBody, &listPayload))
	require.Len(t, listPayload.Logs, 1)
	require.InDelta(t, 100.0, listPayload.Totals.CaloriesKcal, 0.001)
	require.InDelta(t, 10.0, listPayload.Totals.ProteinG, 0.001)
	require.InDelta(t, 5.0, listPayload.Totals.CarbsG, 0.001)
	require.InDelta(t, 1.0, listPayload.Totals.FatG, 0.001)
}

func TestFoodsUPCRequiresProEntitlementIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	authHeader := registerAndAuthHeader(t, server, "non-pro-user@atlas.local")
	resp, body := doRequest(t, server, http.MethodGet, "/api/v1/foods/upc/012345678905", nil, authHeader)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	var payload generated.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, "subscription entitlement required: barcode_scan", payload.Message)
}

func TestBillingVerifyActivatesEntitlementsAndRestoreIntegration(t *testing.T) {
	server := setupIntegrationServer(t)
	authHeader := registerAndAuthHeader(t, server, "billing-verify-user@atlas.local")

	beforeResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/weekly-checkin", map[string]any{}, authHeader)
	require.Equal(t, http.StatusForbidden, beforeResp.StatusCode)

	verifyPayload := activateSubscription(t, server, authHeader, map[string]any{
		"platform":              "ios",
		"productId":             "atlas.pro.monthly",
		"receiptToken":          "rcpt-pro-monthly-1",
		"transactionId":         "tx-pro-monthly-1",
		"originalTransactionId": "orig-pro-monthly-1",
	})
	require.True(t, verifyPayload.IsPro)
	require.Equal(t, generated.CoachTier("pro"), verifyPayload.CoachTier)
	require.Contains(t, verifyPayload.Entitlements, generated.EntitlementKey("barcode_scan"))
	require.Contains(t, verifyPayload.Entitlements, generated.EntitlementKey("deep_nutrition"))
	require.False(t, verifyPayload.Restored)

	afterResp, _ := doRequest(t, server, http.MethodPost, "/api/v1/nutrition/weekly-checkin", map[string]any{}, authHeader)
	require.NotEqual(t, http.StatusForbidden, afterResp.StatusCode)

	restoreResp, restoreBody := doRequest(t, server, http.MethodPost, "/api/v1/billing/verify", map[string]any{
		"platform":              "ios",
		"productId":             "atlas.pro.monthly",
		"receiptToken":          "rcpt-pro-monthly-1",
		"transactionId":         "tx-pro-monthly-1",
		"originalTransactionId": "orig-pro-monthly-1",
		"restore":               true,
	}, authHeader)
	require.Equal(t, http.StatusOK, restoreResp.StatusCode)

	var restorePayload generated.BillingVerifyResponse
	require.NoError(t, json.Unmarshal(restoreBody, &restorePayload))
	require.True(t, restorePayload.Restored)
}

func TestDashboardSummaryIncludesFoodLogDailyTotalsIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	email := "dashboard-food-totals@atlas.local"
	authHeader := registerAndAuthHeader(t, server, email)
	userID := lookupUserIDByEmail(t, email)
	now := time.Now().UTC()

	database := openTestDatabase(t)
	defer database.Close()

	queries := db.New(database)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	foodRow, err := queries.UpsertFood(ctx, db.UpsertFoodParams{
		ExternalID:    "fixture-food-dashboard",
		Provider:      "fixture",
		Label:         "Dashboard Food",
		Brand:         "Atlas Test",
		NutrientsJson: []byte(`{"calories_kcal":250,"protein_g":20,"carbs_g":15,"fat_g":8}`),
	})
	require.NoError(t, err)

	_, err = queries.CreateFoodLog(ctx, db.CreateFoodLogParams{
		UserID:                userID,
		Datetime:              now,
		FoodID:                foodRow.ID,
		Quantity:              2,
		Unit:                  "serving",
		NutrientsSnapshotJson: []byte(`{"calories_kcal":500,"protein_g":40,"carbs_g":30,"fat_g":16}`),
	})
	require.NoError(t, err)

	resp, body := doRequest(t, server, http.MethodGet, "/api/v1/dashboard/summary", nil, authHeader)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload generated.DashboardSummaryResponse
	require.NoError(t, json.Unmarshal(body, &payload))
	require.InDelta(t, 500.0, payload.Summary.NutritionTotalsToday.CaloriesKcal, 0.001)
	require.InDelta(t, 40.0, payload.Summary.NutritionTotalsToday.ProteinG, 0.001)
	require.InDelta(t, 30.0, payload.Summary.NutritionTotalsToday.CarbsG, 0.001)
	require.InDelta(t, 16.0, payload.Summary.NutritionTotalsToday.FatG, 0.001)
}

func setupIntegrationServer(t *testing.T) *httptest.Server {
	t.Helper()

	database := openTestDatabase(t)
	applyMigrations(t, database)
	truncateTables(t, database)

	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Env = "test"
	cfg.ServiceName = "atlas-api"
	cfg.JWTSecret = "integration-test-secret"
	cfg.AccessTokenTTLMinutes = 15
	cfg.RefreshTokenTTLHours = 24

	queries := db.New(database)
	tokenSvc := auth.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL(), cfg.RefreshTokenTTL(), time.Now)
	handler := httpapi.NewRouter(zap.NewNop(), cfg, queries, tokenSvc)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Cleanup(func() {
		require.NoError(t, database.Close())
	})

	return server
}

func openTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := os.Getenv("ATLAS_TEST_POSTGRES_URL")
	if databaseURL == "" {
		databaseURL = "postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable"
	}

	database, err := sql.Open("pgx", databaseURL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		t.Skipf("skipping integration test: postgres unavailable at %s: %v", databaseURL, err)
	}

	return database
}

func applyMigrations(t *testing.T, database *sql.DB) {
	t.Helper()

	for _, migrationPath := range migrationFiles(t) {
		content, err := os.ReadFile(migrationPath)
		require.NoError(t, err)

		upSQL, err := extractGooseUpSQL(string(content))
		require.NoError(t, err)

		_, err = database.Exec(upSQL)
		require.NoErrorf(t, err, "failed applying migration %s", filepath.Base(migrationPath))
	}
}

func truncateTables(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec(`TRUNCATE TABLE form_check_uploads, app_events, cue_timeline, coach_session_assets, coach_sessions, crew_invites, crew_members, crews, grocery_items, meal_plan_items, meal_plans, recipes, weekly_checkins, readiness_checkins, food_logs, foods, user_weight_entries, nutrition_daily_checkins, nutrition_targets, momentum_sprint_reward_milestones, momentum_sprint_daily_checklist_entries, momentum_sprint_enrollments, habit_daily_logs, habits, workout_sets, workout_exercises, workouts, user_program_enrollments, program_session_exercises, program_sessions, program_weeks, programs, user_goals, user_profiles, exercise_biomech_asset_muscle_groups, muscle_groups, exercise_biomech_assets, exercise_media, exercises, subscriptions, consents, sessions, users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func migrationFiles(t *testing.T) []string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	files, err := filepath.Glob(filepath.Join(projectRoot, "migrations", "*.sql"))
	require.NoError(t, err)
	sort.Strings(files)
	require.NotEmpty(t, files)
	return files
}

func extractGooseUpSQL(migration string) (string, error) {
	upMarker := "-- +goose Up"
	downMarker := "-- +goose Down"

	upStart := strings.Index(migration, upMarker)
	if upStart == -1 {
		return "", fmt.Errorf("goose up marker not found")
	}
	downStart := strings.Index(migration, downMarker)
	if downStart == -1 {
		return "", fmt.Errorf("goose down marker not found")
	}
	if downStart <= upStart {
		return "", fmt.Errorf("goose down marker appears before up marker")
	}

	upSection := migration[upStart+len(upMarker) : downStart]
	lines := strings.Split(upSection, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose") {
			continue
		}
		filtered = append(filtered, line)
	}

	return strings.TrimSpace(strings.Join(filtered, "\n")), nil
}

func doRequest(
	t *testing.T,
	server *httptest.Server,
	method string,
	path string,
	body any,
	headers map[string]string,
) (*http.Response, []byte) {
	t.Helper()

	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(payload))
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = resp.Body.Close()
	})

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, respBody
}

func registerAndAuthHeader(t *testing.T, server *httptest.Server, email string) map[string]string {
	t.Helper()

	registerResp, registerBody := doRequest(t, server, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": "strongpass123",
	}, nil)
	require.Equal(t, http.StatusCreated, registerResp.StatusCode)

	var payload generated.AuthResponse
	require.NoError(t, json.Unmarshal(registerBody, &payload))
	return map[string]string{"Authorization": "Bearer " + payload.Tokens.AccessToken}
}

func activateSubscription(
	t *testing.T,
	server *httptest.Server,
	authHeader map[string]string,
	requestBody map[string]any,
) generated.BillingVerifyResponse {
	t.Helper()

	verifyResp, verifyBody := doRequest(t, server, http.MethodPost, "/api/v1/billing/verify", requestBody, authHeader)
	require.Equal(t, http.StatusOK, verifyResp.StatusCode)

	var payload generated.BillingVerifyResponse
	require.NoError(t, json.Unmarshal(verifyBody, &payload))
	return payload
}

func seedProgramFixtures(t *testing.T) uuid.UUID {
	t.Helper()

	database := openTestDatabase(t)
	defer database.Close()

	queries := db.New(database)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exercises := []struct {
		Slug               string
		Name               string
		PrimaryMuscleGroup string
		SecondaryMuscles   []string
		Pattern            string
		Equipment          []string
		Difficulty         string
		Description        string
	}{
		{
			Slug:               "back-squat",
			Name:               "Back Squat",
			PrimaryMuscleGroup: "quads",
			SecondaryMuscles:   []string{"glutes", "core"},
			Pattern:            "squat",
			Equipment:          []string{"barbell", "rack"},
			Difficulty:         "intermediate",
			Description:        "Barbell squat pattern.",
		},
		{
			Slug:               "bench-press",
			Name:               "Bench Press",
			PrimaryMuscleGroup: "chest",
			SecondaryMuscles:   []string{"triceps"},
			Pattern:            "push",
			Equipment:          []string{"barbell", "bench"},
			Difficulty:         "intermediate",
			Description:        "Barbell bench press pattern.",
		},
		{
			Slug:               "barbell-row",
			Name:               "Barbell Row",
			PrimaryMuscleGroup: "upper_back",
			SecondaryMuscles:   []string{"lats", "biceps"},
			Pattern:            "pull",
			Equipment:          []string{"barbell"},
			Difficulty:         "intermediate",
			Description:        "Barbell rowing pattern.",
		},
		{
			Slug:               "front-squat",
			Name:               "Front Squat",
			PrimaryMuscleGroup: "quads",
			SecondaryMuscles:   []string{"core"},
			Pattern:            "squat",
			Equipment:          []string{"barbell", "rack"},
			Difficulty:         "intermediate",
			Description:        "Front-loaded squat pattern.",
		},
	}

	for _, exercise := range exercises {
		primaryMusclesJSON, err := json.Marshal([]string{exercise.PrimaryMuscleGroup})
		require.NoError(t, err)
		secondaryMusclesJSON, err := json.Marshal(exercise.SecondaryMuscles)
		require.NoError(t, err)
		contraindicationsJSON, err := json.Marshal([]string{})
		require.NoError(t, err)
		equipmentJSON, err := json.Marshal(exercise.Equipment)
		require.NoError(t, err)

		_, err = queries.UpsertExercise(ctx, db.UpsertExerciseParams{
			ID:                   uuid.New(),
			Slug:                 exercise.Slug,
			Name:                 exercise.Name,
			PrimaryMuscleGroup:   exercise.PrimaryMuscleGroup,
			SecondaryMusclesJson: secondaryMusclesJSON,
			MovementPattern:      exercise.Pattern,
			PrimaryMuscles:       primaryMusclesJSON,
			SecondaryMuscles:     secondaryMusclesJSON,
			Contraindications:    contraindicationsJSON,
			EquipmentJson:        equipmentJSON,
			Difficulty:           exercise.Difficulty,
			Description:          exercise.Description,
		})
		require.NoError(t, err)
	}

	programID := uuid.New()
	weekID := uuid.New()
	day1SessionID := uuid.New()
	day3SessionID := uuid.New()
	day5SessionID := uuid.New()

	_, err := database.ExecContext(ctx, `
		INSERT INTO programs (id, slug, name, description, goal_tags_json, level, weeks_length, created_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, NOW())
		ON CONFLICT (slug)
		DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			goal_tags_json = EXCLUDED.goal_tags_json,
			level = EXCLUDED.level,
			weeks_length = EXCLUDED.weeks_length
	`, programID, "hypertrophy-foundations-3-days", "Hypertrophy Foundations (3 days/week)", "Three-day hypertrophy-focused template.", `["hypertrophy","foundations"]`, "beginner", 8)
	require.NoError(t, err)

	var persistedProgramID uuid.UUID
	err = database.QueryRowContext(ctx, `SELECT id FROM programs WHERE slug = $1`, "hypertrophy-foundations-3-days").Scan(&persistedProgramID)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO program_weeks (id, program_id, week_index, created_at)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (program_id, week_index) DO NOTHING
	`, weekID, persistedProgramID)
	require.NoError(t, err)

	var persistedWeekID uuid.UUID
	err = database.QueryRowContext(ctx, `SELECT id FROM program_weeks WHERE program_id = $1 AND week_index = 1`, persistedProgramID).Scan(&persistedWeekID)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO program_sessions (id, program_week_id, day_of_week, name, created_at)
		VALUES
			($1, $4, 1, 'Day 1 - Push', NOW()),
			($2, $4, 3, 'Day 2 - Lower', NOW()),
			($3, $4, 5, 'Day 3 - Pull', NOW())
		ON CONFLICT (program_week_id, day_of_week)
		DO UPDATE SET name = EXCLUDED.name
	`, day1SessionID, day3SessionID, day5SessionID, persistedWeekID)
	require.NoError(t, err)

	rows, err := database.QueryContext(ctx, `
		SELECT day_of_week, id
		FROM program_sessions
		WHERE program_week_id = $1
	`, persistedWeekID)
	require.NoError(t, err)
	defer rows.Close()

	sessionByDay := map[int32]uuid.UUID{}
	for rows.Next() {
		var day int32
		var sessionID uuid.UUID
		require.NoError(t, rows.Scan(&day, &sessionID))
		sessionByDay[day] = sessionID
	}
	require.NoError(t, rows.Err())

	_, err = database.ExecContext(ctx, `
		INSERT INTO program_session_exercises (id, program_session_id, exercise_id, prescription_json, order_index)
		SELECT $1, $2, e.id, '{"sets":4,"reps_range":"8-12","rest_seconds":120}'::jsonb, 1
		FROM exercises e
		WHERE e.slug = 'bench-press'
		ON CONFLICT (program_session_id, order_index)
		DO UPDATE SET exercise_id = EXCLUDED.exercise_id, prescription_json = EXCLUDED.prescription_json
	`, uuid.New(), sessionByDay[1])
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO program_session_exercises (id, program_session_id, exercise_id, prescription_json, order_index)
		SELECT $1, $2, e.id, '{"sets":4,"reps_range":"8-10","rest_seconds":150,"rpe_target":8}'::jsonb, 1
		FROM exercises e
		WHERE e.slug = 'back-squat'
		ON CONFLICT (program_session_id, order_index)
		DO UPDATE SET exercise_id = EXCLUDED.exercise_id, prescription_json = EXCLUDED.prescription_json
	`, uuid.New(), sessionByDay[3])
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO program_session_exercises (id, program_session_id, exercise_id, prescription_json, order_index)
		SELECT $1, $2, e.id, '{"sets":4,"reps_range":"8-12","rest_seconds":120}'::jsonb, 1
		FROM exercises e
		WHERE e.slug = 'barbell-row'
		ON CONFLICT (program_session_id, order_index)
		DO UPDATE SET exercise_id = EXCLUDED.exercise_id, prescription_json = EXCLUDED.prescription_json
	`, uuid.New(), sessionByDay[5])
	require.NoError(t, err)

	return persistedProgramID
}

func getProgramSessionIDForProgramDay(t *testing.T, programID uuid.UUID, dayOfWeek int32) uuid.UUID {
	t.Helper()

	database := openTestDatabase(t)
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var sessionID uuid.UUID
	err := database.QueryRowContext(ctx, `
		SELECT ps.id
		FROM program_sessions ps
		JOIN program_weeks pw ON pw.id = ps.program_week_id
		WHERE pw.program_id = $1
		  AND ps.day_of_week = $2
		ORDER BY ps.created_at ASC
		LIMIT 1
	`, programID, dayOfWeek).Scan(&sessionID)
	require.NoError(t, err)

	return sessionID
}

func findProgramSessionExerciseByDayAndOrder(t *testing.T, payload generated.CurrentProgramScheduleResponse, dayOfWeek int32, orderIndex int32) generated.ProgramSessionExercise {
	t.Helper()

	for _, session := range payload.Week.Sessions {
		if session.DayOfWeek != dayOfWeek {
			continue
		}

		for _, exercise := range session.Exercises {
			if exercise.OrderIndex == orderIndex {
				return exercise
			}
		}
	}

	t.Fatalf("exercise with day=%d orderIndex=%d not found", dayOfWeek, orderIndex)
	return generated.ProgramSessionExercise{}
}

func seedExerciseSubstituteFixtures(t *testing.T) map[string]uuid.UUID {
	t.Helper()

	database := openTestDatabase(t)
	defer database.Close()

	queries := db.New(database)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type fixture struct {
		Slug              string
		Name              string
		MovementPattern   string
		PrimaryMuscles    []string
		SecondaryMuscles  []string
		Contraindications []string
		Equipment         []string
		Difficulty        string
		Description       string
	}

	fixtures := []fixture{
		{
			Slug:              "back-squat",
			Name:              "Back Squat",
			MovementPattern:   "squat",
			PrimaryMuscles:    []string{"quads", "glutes"},
			SecondaryMuscles:  []string{"core"},
			Contraindications: []string{},
			Equipment:         []string{"barbell", "rack"},
			Difficulty:        "intermediate",
			Description:       "Barbell squat pattern.",
		},
		{
			Slug:              "goblet-squat",
			Name:              "Goblet Squat",
			MovementPattern:   "squat",
			PrimaryMuscles:    []string{"quads", "glutes"},
			SecondaryMuscles:  []string{"core"},
			Contraindications: []string{},
			Equipment:         []string{"dumbbell", "kettlebell"},
			Difficulty:        "beginner",
			Description:       "Dumbbell goblet squat.",
		},
		{
			Slug:              "front-squat",
			Name:              "Front Squat",
			MovementPattern:   "squat",
			PrimaryMuscles:    []string{"quads"},
			SecondaryMuscles:  []string{"core"},
			Contraindications: []string{"acute_knee_injury"},
			Equipment:         []string{"barbell", "rack"},
			Difficulty:        "advanced",
			Description:       "Front-loaded squat.",
		},
		{
			Slug:              "split-squat",
			Name:              "Split Squat",
			MovementPattern:   "squat",
			PrimaryMuscles:    []string{"quads"},
			SecondaryMuscles:  []string{"glutes"},
			Contraindications: []string{},
			Equipment:         []string{"dumbbell", "bodyweight"},
			Difficulty:        "beginner",
			Description:       "Single-leg squat pattern.",
		},
		{
			Slug:              "hip-thrust",
			Name:              "Hip Thrust",
			MovementPattern:   "hinge",
			PrimaryMuscles:    []string{"glutes"},
			SecondaryMuscles:  []string{"hamstrings"},
			Contraindications: []string{},
			Equipment:         []string{"barbell", "bench"},
			Difficulty:        "intermediate",
			Description:       "Hinge accessory.",
		},
	}

	ids := make(map[string]uuid.UUID, len(fixtures))
	for _, item := range fixtures {
		primaryMusclesJSON, err := json.Marshal(item.PrimaryMuscles)
		require.NoError(t, err)
		secondaryMusclesJSON, err := json.Marshal(item.SecondaryMuscles)
		require.NoError(t, err)
		contraindicationsJSON, err := json.Marshal(item.Contraindications)
		require.NoError(t, err)
		equipmentJSON, err := json.Marshal(item.Equipment)
		require.NoError(t, err)

		primaryMuscleGroup := ""
		if len(item.PrimaryMuscles) > 0 {
			primaryMuscleGroup = item.PrimaryMuscles[0]
		}

		row, err := queries.UpsertExercise(ctx, db.UpsertExerciseParams{
			ID:                   uuid.New(),
			Slug:                 item.Slug,
			Name:                 item.Name,
			PrimaryMuscleGroup:   primaryMuscleGroup,
			SecondaryMusclesJson: secondaryMusclesJSON,
			MovementPattern:      item.MovementPattern,
			PrimaryMuscles:       primaryMusclesJSON,
			SecondaryMuscles:     secondaryMusclesJSON,
			Contraindications:    contraindicationsJSON,
			EquipmentJson:        equipmentJSON,
			Difficulty:           item.Difficulty,
			Description:          item.Description,
		})
		require.NoError(t, err)
		ids[item.Slug] = row.ID
	}

	return ids
}

func seedExerciseBiomechanicsFixtures(t *testing.T) map[string]uuid.UUID {
	t.Helper()

	database := openTestDatabase(t)
	defer database.Close()

	queries := db.New(database)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type fixture struct {
		Slug               string
		Name               string
		PrimaryMuscleGroup string
		SecondaryMuscles   []string
		Pattern            string
		Equipment          []string
		Difficulty         string
		Description        string
		AnimationAssetKey  string
		MetadataJSON       string
	}

	fixtures := []fixture{
		{
			Slug:               "back-squat",
			Name:               "Back Squat",
			PrimaryMuscleGroup: "quads",
			SecondaryMuscles:   []string{"glutes", "core"},
			Pattern:            "squat",
			Equipment:          []string{"barbell", "rack"},
			Difficulty:         "intermediate",
			Description:        "Barbell squat pattern.",
			AnimationAssetKey:  "biomechanics/back-squat/clip_v1.fbx",
			MetadataJSON:       `{"muscleHighlights":[{"muscleGroup":"quads","activationLevel":1.0,"role":"primary"},{"muscleGroup":"glutes","activationLevel":0.82,"role":"secondary"}],"jointAngles":[{"joint":"knee","minDegrees":70,"maxDegrees":175,"targetDegrees":95,"unit":"deg"}]}`,
		},
		{
			Slug:               "bench-press",
			Name:               "Bench Press",
			PrimaryMuscleGroup: "chest",
			SecondaryMuscles:   []string{"triceps", "shoulders"},
			Pattern:            "push",
			Equipment:          []string{"barbell", "bench"},
			Difficulty:         "intermediate",
			Description:        "Barbell horizontal press.",
			AnimationAssetKey:  "biomechanics/bench-press/clip_v1.fbx",
			MetadataJSON:       `{"muscleHighlights":[{"muscleGroup":"chest","activationLevel":1.0,"role":"primary"},{"muscleGroup":"triceps","activationLevel":0.86,"role":"secondary"}],"jointAngles":[{"joint":"elbow","minDegrees":65,"maxDegrees":178,"targetDegrees":92,"unit":"deg"}]}`,
		},
		{
			Slug:               "barbell-row",
			Name:               "Barbell Row",
			PrimaryMuscleGroup: "upper_back",
			SecondaryMuscles:   []string{"lats", "biceps"},
			Pattern:            "pull",
			Equipment:          []string{"barbell"},
			Difficulty:         "intermediate",
			Description:        "Bent-over horizontal pull.",
			AnimationAssetKey:  "biomechanics/barbell-row/clip_v1.fbx",
			MetadataJSON:       `{"muscleHighlights":[{"muscleGroup":"upper_back","activationLevel":1.0,"role":"primary"},{"muscleGroup":"lats","activationLevel":0.83,"role":"secondary"}],"jointAngles":[{"joint":"elbow","minDegrees":50,"maxDegrees":165,"targetDegrees":85,"unit":"deg"}]}`,
		},
	}

	ids := make(map[string]uuid.UUID, len(fixtures))
	for _, item := range fixtures {
		primaryMusclesJSON, err := json.Marshal([]string{item.PrimaryMuscleGroup})
		require.NoError(t, err)
		secondaryMusclesJSON, err := json.Marshal(item.SecondaryMuscles)
		require.NoError(t, err)
		contraindicationsJSON, err := json.Marshal([]string{})
		require.NoError(t, err)
		equipmentJSON, err := json.Marshal(item.Equipment)
		require.NoError(t, err)

		exerciseRow, err := queries.UpsertExercise(ctx, db.UpsertExerciseParams{
			ID:                   uuid.New(),
			Slug:                 item.Slug,
			Name:                 item.Name,
			PrimaryMuscleGroup:   item.PrimaryMuscleGroup,
			SecondaryMusclesJson: secondaryMusclesJSON,
			MovementPattern:      item.Pattern,
			PrimaryMuscles:       primaryMusclesJSON,
			SecondaryMuscles:     secondaryMusclesJSON,
			Contraindications:    contraindicationsJSON,
			EquipmentJson:        equipmentJSON,
			Difficulty:           item.Difficulty,
			Description:          item.Description,
		})
		require.NoError(t, err)
		ids[item.Slug] = exerciseRow.ID

		_, err = database.ExecContext(
			ctx,
			`
			INSERT INTO exercise_biomech_assets (id, exercise_id, animation_asset_key, rig_version, metadata_json, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5::jsonb, NOW(), NOW())
			ON CONFLICT (exercise_id)
			DO UPDATE SET
				animation_asset_key = EXCLUDED.animation_asset_key,
				rig_version = EXCLUDED.rig_version,
				metadata_json = EXCLUDED.metadata_json,
				updated_at = NOW()
			`,
			uuid.New(),
			exerciseRow.ID,
			item.AnimationAssetKey,
			"atlas-humanoid-v1",
			item.MetadataJSON,
		)
		require.NoError(t, err)
	}

	return ids
}

func seedExerciseFixtures(t *testing.T) map[string]uuid.UUID {
	t.Helper()

	database := openTestDatabase(t)
	defer database.Close()

	queries := db.New(database)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type fixture struct {
		Slug               string
		Name               string
		PrimaryMuscleGroup string
		SecondaryMuscles   []string
		Contraindications  []string
		Pattern            string
		Equipment          []string
		Difficulty         string
		Description        string
	}

	fixtures := []fixture{
		{
			Slug:               "back-squat",
			Name:               "Back Squat",
			PrimaryMuscleGroup: "quads",
			SecondaryMuscles:   []string{"glutes", "core"},
			Contraindications:  []string{"acute_knee_injury"},
			Pattern:            "squat",
			Equipment:          []string{"barbell", "rack"},
			Difficulty:         "intermediate",
			Description:        "Barbell squat pattern.",
		},
		{
			Slug:               "kettlebell-swing",
			Name:               "Kettlebell Swing",
			PrimaryMuscleGroup: "glutes",
			SecondaryMuscles:   []string{"hamstrings", "core"},
			Contraindications:  []string{},
			Pattern:            "hinge",
			Equipment:          []string{"kettlebell"},
			Difficulty:         "intermediate",
			Description:        "Explosive hinge with kettlebell.",
		},
		{
			Slug:               "plank",
			Name:               "Plank",
			PrimaryMuscleGroup: "core",
			SecondaryMuscles:   []string{"glutes"},
			Contraindications:  []string{},
			Pattern:            "core",
			Equipment:          []string{"bodyweight"},
			Difficulty:         "beginner",
			Description:        "Static anti-extension core hold.",
		},
	}

	ids := make(map[string]uuid.UUID, len(fixtures))
	for _, item := range fixtures {
		primaryMusclesJSON, err := json.Marshal([]string{item.PrimaryMuscleGroup})
		require.NoError(t, err)
		secondaryMusclesJSON, err := json.Marshal(item.SecondaryMuscles)
		require.NoError(t, err)
		contraindicationsJSON, err := json.Marshal(item.Contraindications)
		require.NoError(t, err)

		equipmentJSON, err := json.Marshal(item.Equipment)
		require.NoError(t, err)

		row, err := queries.UpsertExercise(ctx, db.UpsertExerciseParams{
			ID:                   uuid.New(),
			Slug:                 item.Slug,
			Name:                 item.Name,
			PrimaryMuscleGroup:   item.PrimaryMuscleGroup,
			SecondaryMusclesJson: secondaryMusclesJSON,
			MovementPattern:      item.Pattern,
			PrimaryMuscles:       primaryMusclesJSON,
			SecondaryMuscles:     secondaryMusclesJSON,
			Contraindications:    contraindicationsJSON,
			EquipmentJson:        equipmentJSON,
			Difficulty:           item.Difficulty,
			Description:          item.Description,
		})
		require.NoError(t, err)
		ids[item.Slug] = row.ID
	}

	return ids
}

func seedMealPlanRecipes(t *testing.T) {
	t.Helper()

	database := openTestDatabase(t)
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type recipeSeed struct {
		Slug         string
		Name         string
		MealType     string
		Description  string
		Servings     float64
		CaloriesKcal int32
		ProteinG     int32
		CarbsG       int32
		FatG         int32
		Ingredients  string
		Instructions string
	}

	recipes := []recipeSeed{
		{
			Slug:         "overnight-oats-protein",
			Name:         "Protein Overnight Oats",
			MealType:     "breakfast",
			Description:  "Oats with yogurt and berries.",
			Servings:     1,
			CaloriesKcal: 420,
			ProteinG:     30,
			CarbsG:       56,
			FatG:         12,
			Ingredients:  `[{"name":"rolled oats","quantity":60,"unit":"g","category":"grains"},{"name":"greek yogurt","quantity":170,"unit":"g","category":"dairy"},{"name":"berries","quantity":100,"unit":"g","category":"produce"}]`,
			Instructions: `["Combine ingredients in a jar.","Chill overnight."]`,
		},
		{
			Slug:         "egg-veggie-scramble",
			Name:         "Egg Veggie Scramble",
			MealType:     "breakfast",
			Description:  "Eggs scrambled with spinach.",
			Servings:     1,
			CaloriesKcal: 390,
			ProteinG:     28,
			CarbsG:       24,
			FatG:         18,
			Ingredients:  `[{"name":"eggs","quantity":3,"unit":"item","category":"protein"},{"name":"spinach","quantity":80,"unit":"g","category":"produce"},{"name":"whole grain toast","quantity":1,"unit":"slice","category":"grains"}]`,
			Instructions: `["Scramble eggs and spinach.","Serve with toast."]`,
		},
		{
			Slug:         "chicken-rice-bowl",
			Name:         "Chicken Rice Bowl",
			MealType:     "lunch",
			Description:  "Chicken breast with rice and vegetables.",
			Servings:     1,
			CaloriesKcal: 650,
			ProteinG:     48,
			CarbsG:       72,
			FatG:         16,
			Ingredients:  `[{"name":"chicken breast","quantity":170,"unit":"g","category":"protein"},{"name":"jasmine rice","quantity":150,"unit":"g","category":"grains"},{"name":"broccoli","quantity":120,"unit":"g","category":"produce"}]`,
			Instructions: `["Cook chicken and rice.","Assemble bowl with vegetables."]`,
		},
		{
			Slug:         "turkey-wrap",
			Name:         "Turkey Wrap",
			MealType:     "lunch",
			Description:  "Turkey wrap with hummus and greens.",
			Servings:     1,
			CaloriesKcal: 610,
			ProteinG:     42,
			CarbsG:       60,
			FatG:         18,
			Ingredients:  `[{"name":"whole wheat wrap","quantity":1,"unit":"item","category":"grains"},{"name":"turkey breast","quantity":140,"unit":"g","category":"protein"},{"name":"hummus","quantity":45,"unit":"g","category":"fats"},{"name":"mixed greens","quantity":60,"unit":"g","category":"produce"}]`,
			Instructions: `["Layer ingredients in wrap.","Roll and slice."]`,
		},
		{
			Slug:         "salmon-potato-plate",
			Name:         "Salmon Potato Plate",
			MealType:     "dinner",
			Description:  "Roasted salmon with potatoes and green beans.",
			Servings:     1,
			CaloriesKcal: 700,
			ProteinG:     46,
			CarbsG:       54,
			FatG:         28,
			Ingredients:  `[{"name":"salmon fillet","quantity":180,"unit":"g","category":"protein"},{"name":"potatoes","quantity":250,"unit":"g","category":"produce"},{"name":"green beans","quantity":140,"unit":"g","category":"produce"}]`,
			Instructions: `["Roast salmon and potatoes.","Steam green beans and plate."]`,
		},
		{
			Slug:         "lean-beef-pasta",
			Name:         "Lean Beef Pasta",
			MealType:     "dinner",
			Description:  "Lean beef with pasta and tomato sauce.",
			Servings:     1,
			CaloriesKcal: 720,
			ProteinG:     50,
			CarbsG:       78,
			FatG:         20,
			Ingredients:  `[{"name":"lean ground beef","quantity":170,"unit":"g","category":"protein"},{"name":"dry pasta","quantity":90,"unit":"g","category":"grains"},{"name":"tomato sauce","quantity":180,"unit":"g","category":"produce"}]`,
			Instructions: `["Cook pasta.","Brown beef and add sauce.","Combine and serve."]`,
		},
	}

	for _, recipe := range recipes {
		_, err := database.ExecContext(ctx, `
			INSERT INTO recipes (
				id,
				slug,
				name,
				meal_type,
				description,
				servings,
				calories_kcal,
				protein_g,
				carbs_g,
				fat_g,
				ingredients_json,
				instructions_json,
				created_at,
				updated_at
			) VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11::jsonb,
				$12::jsonb,
				NOW(),
				NOW()
			)
			ON CONFLICT (slug)
			DO UPDATE SET
				name = EXCLUDED.name,
				meal_type = EXCLUDED.meal_type,
				description = EXCLUDED.description,
				servings = EXCLUDED.servings,
				calories_kcal = EXCLUDED.calories_kcal,
				protein_g = EXCLUDED.protein_g,
				carbs_g = EXCLUDED.carbs_g,
				fat_g = EXCLUDED.fat_g,
				ingredients_json = EXCLUDED.ingredients_json,
				instructions_json = EXCLUDED.instructions_json,
				updated_at = NOW()
		`,
			uuid.New(),
			recipe.Slug,
			recipe.Name,
			recipe.MealType,
			recipe.Description,
			recipe.Servings,
			recipe.CaloriesKcal,
			recipe.ProteinG,
			recipe.CarbsG,
			recipe.FatG,
			recipe.Ingredients,
			recipe.Instructions,
		)
		require.NoError(t, err)
	}
}

func lookupUserIDByEmail(t *testing.T, email string) uuid.UUID {
	t.Helper()

	database := openTestDatabase(t)
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var userID uuid.UUID
	err := database.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1 LIMIT 1`, email).Scan(&userID)
	require.NoError(t, err)
	return userID
}

func seedAnalyticsWorkoutData(t *testing.T, userID uuid.UUID, now time.Time) {
	t.Helper()

	database := openTestDatabase(t)
	defer database.Close()

	queries := db.New(database)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type exerciseSeed struct {
		Slug               string
		Name               string
		PrimaryMuscleGroup string
		SecondaryMuscles   []string
		MovementPattern    string
		Equipment          []string
		Difficulty         string
		Description        string
	}

	exerciseSeeds := []exerciseSeed{
		{
			Slug:               "back-squat",
			Name:               "Back Squat",
			PrimaryMuscleGroup: "quads",
			SecondaryMuscles:   []string{"glutes", "core"},
			MovementPattern:    "squat",
			Equipment:          []string{"barbell", "rack"},
			Difficulty:         "intermediate",
			Description:        "Barbell squat pattern.",
		},
		{
			Slug:               "bench-press",
			Name:               "Bench Press",
			PrimaryMuscleGroup: "chest",
			SecondaryMuscles:   []string{"triceps"},
			MovementPattern:    "push",
			Equipment:          []string{"barbell", "bench"},
			Difficulty:         "intermediate",
			Description:        "Barbell bench press pattern.",
		},
		{
			Slug:               "conventional-deadlift",
			Name:               "Conventional Deadlift",
			PrimaryMuscleGroup: "glutes",
			SecondaryMuscles:   []string{"hamstrings", "erectors"},
			MovementPattern:    "hinge",
			Equipment:          []string{"barbell"},
			Difficulty:         "advanced",
			Description:        "Conventional barbell deadlift.",
		},
	}

	exerciseIDs := map[string]uuid.UUID{}
	for _, item := range exerciseSeeds {
		primaryMusclesJSON, err := json.Marshal([]string{item.PrimaryMuscleGroup})
		require.NoError(t, err)
		secondaryMusclesJSON, err := json.Marshal(item.SecondaryMuscles)
		require.NoError(t, err)
		contraindicationsJSON, err := json.Marshal([]string{})
		require.NoError(t, err)
		equipmentJSON, err := json.Marshal(item.Equipment)
		require.NoError(t, err)

		row, err := queries.UpsertExercise(ctx, db.UpsertExerciseParams{
			ID:                   uuid.New(),
			Slug:                 item.Slug,
			Name:                 item.Name,
			PrimaryMuscleGroup:   item.PrimaryMuscleGroup,
			SecondaryMusclesJson: secondaryMusclesJSON,
			MovementPattern:      item.MovementPattern,
			PrimaryMuscles:       primaryMusclesJSON,
			SecondaryMuscles:     secondaryMusclesJSON,
			Contraindications:    contraindicationsJSON,
			EquipmentJson:        equipmentJSON,
			Difficulty:           item.Difficulty,
			Description:          item.Description,
		})
		require.NoError(t, err)
		exerciseIDs[item.Slug] = row.ID
	}

	workoutRecentOne := uuid.New()
	workoutRecentTwo := uuid.New()
	workoutOld := uuid.New()
	recentOneTime := now.AddDate(0, 0, -2)
	recentTwoTime := now.AddDate(0, 0, -1)
	oldTime := now.AddDate(0, 0, -20)

	_, err := database.ExecContext(ctx, `
		INSERT INTO workouts (id, user_id, started_at, completed_at, notes, created_at)
		VALUES
			($1, $2, $3, $3, '', $3),
			($4, $2, $5, $5, '', $5),
			($6, $2, $7, $7, '', $7)
	`, workoutRecentOne, userID, recentOneTime, workoutRecentTwo, recentTwoTime, workoutOld, oldTime)
	require.NoError(t, err)

	workoutExerciseRecentOneSquat := uuid.New()
	workoutExerciseRecentOneBench := uuid.New()
	workoutExerciseRecentTwoSquat := uuid.New()
	workoutExerciseOldDeadlift := uuid.New()

	_, err = database.ExecContext(ctx, `
		INSERT INTO workout_exercises (id, workout_id, exercise_id, order_index, planned_json, actual_json, created_at)
		VALUES
			($1, $2, $3, 1, '{}'::jsonb, '{}'::jsonb, $4),
			($5, $2, $6, 2, '{}'::jsonb, '{}'::jsonb, $4),
			($7, $8, $3, 1, '{}'::jsonb, '{}'::jsonb, $9),
			($10, $11, $12, 1, '{}'::jsonb, '{}'::jsonb, $13)
	`,
		workoutExerciseRecentOneSquat, workoutRecentOne, exerciseIDs["back-squat"], recentOneTime,
		workoutExerciseRecentOneBench, exerciseIDs["bench-press"],
		workoutExerciseRecentTwoSquat, workoutRecentTwo, recentTwoTime,
		workoutExerciseOldDeadlift, workoutOld, exerciseIDs["conventional-deadlift"], oldTime,
	)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO workout_sets (id, workout_exercise_id, set_index, reps, weight_kg, rpe, completed_at, created_at)
		VALUES
			($1, $2, 1, 5, 100.0, NULL, $3, $3),
			($4, $5, 1, 5, 80.0, NULL, $3, $3),
			($6, $7, 1, 3, 120.0, NULL, $8, $8),
			($9, $10, 1, 2, 170.0, NULL, $11, $11)
	`,
		uuid.New(), workoutExerciseRecentOneSquat, recentOneTime,
		uuid.New(), workoutExerciseRecentOneBench,
		uuid.New(), workoutExerciseRecentTwoSquat, recentTwoTime,
		uuid.New(), workoutExerciseOldDeadlift, oldTime,
	)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO nutrition_daily_checkins (
			id,
			user_id,
			date,
			calories_estimate,
			protein_g_estimate,
			hit_calories,
			hit_protein,
			notes,
			created_at
		) VALUES
			($1, $2, $3, 2100, 170, TRUE, TRUE, '', $4),
			($5, $2, $6, 2300, 140, FALSE, FALSE, '', $7),
			($8, $2, $9, 2200, 165, TRUE, TRUE, '', $10),
			($11, $2, $12, 2000, 175, TRUE, TRUE, '', $13)
	`,
		uuid.New(), userID, now.AddDate(0, 0, -2).Format("2006-01-02"), now.AddDate(0, 0, -2),
		uuid.New(), now.AddDate(0, 0, -1).Format("2006-01-02"), now.AddDate(0, 0, -1),
		uuid.New(), now.AddDate(0, 0, -6).Format("2006-01-02"), now.AddDate(0, 0, -6),
		uuid.New(), now.AddDate(0, 0, -20).Format("2006-01-02"), now.AddDate(0, 0, -20),
	)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO user_weight_entries (
			user_id,
			date,
			weight_kg,
			unit,
			created_at,
			updated_at
		) VALUES
			($1, $2, 84.6, 'kg', $6, $6),
			($1, $3, 84.1, 'kg', $7, $7),
			($1, $4, 83.8, 'kg', $8, $8),
			($1, $5, 83.5, 'kg', $9, $9)
	`,
		userID,
		now.AddDate(0, 0, -21).Format("2006-01-02"),
		now.AddDate(0, 0, -14).Format("2006-01-02"),
		now.AddDate(0, 0, -7).Format("2006-01-02"),
		now.Format("2006-01-02"),
		now.AddDate(0, 0, -21),
		now.AddDate(0, 0, -14),
		now.AddDate(0, 0, -7),
		now,
	)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO readiness_checkins (
			user_id,
			date,
			energy_level,
			sleep_quality,
			stress_level,
			readiness_score,
			created_at,
			updated_at
		) VALUES
			($1, $2, 3, 2, 1, 2.6667, $4, $4),
			($1, $3, 2, 2, 2, 2.0, $5, $5)
	`,
		userID,
		now.AddDate(0, 0, -2).Format("2006-01-02"),
		now.AddDate(0, 0, -1).Format("2006-01-02"),
		now.AddDate(0, 0, -2),
		now.AddDate(0, 0, -1),
	)
	require.NoError(t, err)
}
