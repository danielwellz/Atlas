package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCrewsInviteMembershipIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	ownerAuth := registerAndAuthHeader(t, server, "crew-owner@atlas.local")

	createResp, createBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews",
		map[string]any{
			"name":            "Strength Crew",
			"description":     "Private crew for strength training.",
			"sharedPlanUrl":   "https://atlas.local/plans/strength",
			"sharedHabitsUrl": "https://atlas.local/habits/strength",
		},
		ownerAuth,
	)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createPayload generated.CrewResponse
	require.NoError(t, json.Unmarshal(createBody, &createPayload))
	require.True(t, createPayload.Crew.IsPrivate)
	require.Equal(t, int32(1), createPayload.Crew.MemberCount)
	require.Len(t, createPayload.Members, 1)
	require.Equal(t, generated.Owner, createPayload.Members[0].Role)

	crewID := uuid.UUID(createPayload.Crew.Id)

	inviteResp, inviteBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews/"+crewID.String()+"/invites",
		map[string]any{
			"maxUses": 2,
		},
		ownerAuth,
	)
	require.Equal(t, http.StatusCreated, inviteResp.StatusCode)

	var invitePayload generated.CrewInviteResponse
	require.NoError(t, json.Unmarshal(inviteBody, &invitePayload))
	require.Equal(t, int32(2), invitePayload.Invite.MaxUses)
	require.Equal(t, int32(0), invitePayload.Invite.UsesCount)
	require.NotEmpty(t, invitePayload.Invite.InviteCode)

	inviteID := uuid.UUID(invitePayload.Invite.Id)
	inviteCode := invitePayload.Invite.InviteCode

	joinerAuth := registerAndAuthHeader(t, server, "crew-joiner@atlas.local")

	forbiddenResp, _ := doRequest(t, server, http.MethodGet, "/api/v1/crews/"+crewID.String(), nil, joinerAuth)
	require.Equal(t, http.StatusForbidden, forbiddenResp.StatusCode)

	joinResp, joinBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews/join",
		map[string]any{
			"inviteCode": inviteCode,
		},
		joinerAuth,
	)
	require.Equal(t, http.StatusOK, joinResp.StatusCode)

	var joinPayload generated.JoinCrewResponse
	require.NoError(t, json.Unmarshal(joinBody, &joinPayload))
	require.True(t, joinPayload.Joined)
	require.Equal(t, crewID, uuid.UUID(joinPayload.Crew.Id))
	require.Equal(t, int32(2), joinPayload.Crew.MemberCount)

	joinAgainResp, joinAgainBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews/join",
		map[string]any{
			"inviteCode": inviteCode,
		},
		joinerAuth,
	)
	require.Equal(t, http.StatusOK, joinAgainResp.StatusCode)

	var joinAgainPayload generated.JoinCrewResponse
	require.NoError(t, json.Unmarshal(joinAgainBody, &joinAgainPayload))
	require.False(t, joinAgainPayload.Joined)
	require.Equal(t, int32(2), joinAgainPayload.Crew.MemberCount)

	detailResp, detailBody := doRequest(t, server, http.MethodGet, "/api/v1/crews/"+crewID.String(), nil, joinerAuth)
	require.Equal(t, http.StatusOK, detailResp.StatusCode)

	var detailPayload generated.CrewResponse
	require.NoError(t, json.Unmarshal(detailBody, &detailPayload))
	require.Equal(t, int32(2), detailPayload.Crew.MemberCount)
	require.Len(t, detailPayload.Members, 2)

	database := openTestDatabase(t)
	defer database.Close()

	var usesCount int
	require.NoError(t, database.QueryRow(`SELECT uses_count FROM crew_invites WHERE id = $1`, inviteID).Scan(&usesCount))
	require.Equal(t, 1, usesCount)
}

func TestCoachSessionsContentAccessIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	ownerAuth := registerAndAuthHeader(t, server, "coach-owner@atlas.local")
	memberAuth := registerAndAuthHeader(t, server, "coach-member@atlas.local")
	outsiderAuth := registerAndAuthHeader(t, server, "coach-outsider@atlas.local")

	createResp, createBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews",
		map[string]any{
			"name":        "Coach Cohort Alpha",
			"description": "Coach-led cohort for squat mechanics.",
		},
		ownerAuth,
	)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createPayload generated.CrewResponse
	require.NoError(t, json.Unmarshal(createBody, &createPayload))
	crewID := uuid.UUID(createPayload.Crew.Id)

	inviteResp, inviteBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews/"+crewID.String()+"/invites",
		map[string]any{
			"maxUses": 3,
		},
		ownerAuth,
	)
	require.Equal(t, http.StatusCreated, inviteResp.StatusCode)

	var invitePayload generated.CrewInviteResponse
	require.NoError(t, json.Unmarshal(inviteBody, &invitePayload))

	joinResp, _ := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews/join",
		map[string]any{
			"inviteCode": invitePayload.Invite.InviteCode,
		},
		memberAuth,
	)
	require.Equal(t, http.StatusOK, joinResp.StatusCode)

	sessionID := seedCoachSessionFixture(t, crewID)

	listResp, listBody := doRequest(t, server, http.MethodGet, "/api/v1/coach-sessions", nil, memberAuth)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listPayload generated.CoachSessionsResponse
	require.NoError(t, json.Unmarshal(listBody, &listPayload))
	require.Len(t, listPayload.Sessions, 1)
	require.Equal(t, sessionID, uuid.UUID(listPayload.Sessions[0].Id))

	detailResp, detailBody := doRequest(t, server, http.MethodGet, "/api/v1/coach-sessions/"+sessionID.String(), nil, memberAuth)
	require.Equal(t, http.StatusOK, detailResp.StatusCode)

	var detailPayload generated.CoachSessionResponse
	require.NoError(t, json.Unmarshal(detailBody, &detailPayload))
	require.Equal(t, sessionID, uuid.UUID(detailPayload.Session.Id))
	require.Equal(t, "Coach Cohort Alpha", detailPayload.Session.CrewName)
	require.NotEmpty(t, detailPayload.Session.Assets)
	require.NotEmpty(t, detailPayload.Session.Assets[0].SignedUrl)
	require.Len(t, detailPayload.Session.Cues, 2)
	require.Equal(t, int32(0), detailPayload.Session.Cues[0].StartMs)
	require.Equal(t, "muscle_group", string(detailPayload.Session.Cues[0].BiomechanicsDefinitionType))
	require.Equal(t, "quads", detailPayload.Session.Cues[0].BiomechanicsDefinitionKey)

	outsiderDetailResp, _ := doRequest(t, server, http.MethodGet, "/api/v1/coach-sessions/"+sessionID.String(), nil, outsiderAuth)
	require.Equal(t, http.StatusForbidden, outsiderDetailResp.StatusCode)

	outsiderListResp, outsiderListBody := doRequest(t, server, http.MethodGet, "/api/v1/coach-sessions", nil, outsiderAuth)
	require.Equal(t, http.StatusOK, outsiderListResp.StatusCode)

	var outsiderListPayload generated.CoachSessionsResponse
	require.NoError(t, json.Unmarshal(outsiderListBody, &outsiderListPayload))
	require.Empty(t, outsiderListPayload.Sessions)
}

func TestCoachSessionsTierEnforcementIntegration(t *testing.T) {
	server := setupIntegrationServer(t)

	ownerAuth := registerAndAuthHeader(t, server, "coach-tier-owner@atlas.local")
	memberAuth := registerAndAuthHeader(t, server, "coach-tier-member@atlas.local")

	createResp, createBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews",
		map[string]any{
			"name":        "Coach Tier Crew",
			"description": "Tier-gated coach content",
		},
		ownerAuth,
	)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createPayload generated.CrewResponse
	require.NoError(t, json.Unmarshal(createBody, &createPayload))
	crewID := uuid.UUID(createPayload.Crew.Id)

	inviteResp, inviteBody := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews/"+crewID.String()+"/invites",
		map[string]any{
			"maxUses": 1,
		},
		ownerAuth,
	)
	require.Equal(t, http.StatusCreated, inviteResp.StatusCode)

	var invitePayload generated.CrewInviteResponse
	require.NoError(t, json.Unmarshal(inviteBody, &invitePayload))

	joinResp, _ := doRequest(
		t,
		server,
		http.MethodPost,
		"/api/v1/crews/join",
		map[string]any{
			"inviteCode": invitePayload.Invite.InviteCode,
		},
		memberAuth,
	)
	require.Equal(t, http.StatusOK, joinResp.StatusCode)

	sessionID := seedCoachSessionFixtureWithTier(t, crewID, "elite")

	activateSubscription(t, server, memberAuth, map[string]any{
		"platform":      "ios",
		"productId":     "atlas.pro.monthly",
		"receiptToken":  "rcpt-coach-tier-pro",
		"transactionId": "tx-coach-tier-pro",
	})

	forbiddenResp, _ := doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/coach-sessions/"+sessionID.String(),
		nil,
		memberAuth,
	)
	require.Equal(t, http.StatusForbidden, forbiddenResp.StatusCode)

	activateSubscription(t, server, memberAuth, map[string]any{
		"platform":      "ios",
		"productId":     "atlas.elite.monthly",
		"receiptToken":  "rcpt-coach-tier-elite",
		"transactionId": "tx-coach-tier-elite",
	})

	allowedResp, allowedBody := doRequest(
		t,
		server,
		http.MethodGet,
		"/api/v1/coach-sessions/"+sessionID.String(),
		nil,
		memberAuth,
	)
	require.Equal(t, http.StatusOK, allowedResp.StatusCode)

	var allowedPayload generated.CoachSessionResponse
	require.NoError(t, json.Unmarshal(allowedBody, &allowedPayload))
	require.Equal(t, generated.CoachTier("elite"), allowedPayload.Session.RequiredTier)
}

func seedCoachSessionFixture(t *testing.T, crewID uuid.UUID) uuid.UUID {
	return seedCoachSessionFixtureWithTier(t, crewID, "free")
}

func seedCoachSessionFixtureWithTier(t *testing.T, crewID uuid.UUID, requiredTier string) uuid.UUID {
	t.Helper()

	database := openTestDatabase(t)
	defer database.Close()

	sessionID := uuid.New()
	videoAssetID := uuid.New()
	cueOneID := uuid.New()
	cueTwoID := uuid.New()

	now := time.Now().UTC()

	_, err := database.Exec(
		`INSERT INTO coach_sessions (
			id,
			crew_id,
			title,
			description,
			coach_name,
			duration_seconds,
			required_tier,
			published,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, $8, $9)`,
		sessionID,
		crewID,
		"Squat Pattern Masterclass",
		"Coach-led session for better squat mechanics.",
		"Coach Elena",
		840,
		requiredTier,
		now,
		now,
	)
	require.NoError(t, err)

	_, err = database.Exec(
		`INSERT INTO coach_session_assets (
			id,
			coach_session_id,
			asset_type,
			storage_key,
			mime_type,
			created_at
		) VALUES ($1, $2, 'video', 'coach/sessions/squat-masterclass.mp4', 'video/mp4', $3)`,
		videoAssetID,
		sessionID,
		now,
	)
	require.NoError(t, err)

	_, err = database.Exec(
		`INSERT INTO cue_timeline (
			id,
			coach_session_id,
			cue_index,
			start_ms,
			end_ms,
			cue_text,
			biomechanics_definition_type,
			biomechanics_definition_key,
			created_at
		) VALUES
			($1, $2, 1, 0, 7000, 'Brace and keep chest up.', 'muscle_group', 'quads', $3),
			($4, $2, 2, 7000, 14000, 'Drive knees over toes with control.', 'joint_angle', 'knee_flexion', $3)`,
		cueOneID,
		sessionID,
		now,
		cueTwoID,
	)
	require.NoError(t, err)

	return sessionID
}
