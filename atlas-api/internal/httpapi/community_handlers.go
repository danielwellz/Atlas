package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/entitlement"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

const (
	defaultCrewInviteMaxUses     int32         = 1
	maxCrewInviteMaxUses         int32         = 100
	crewInviteCodeByteLength     int           = 6
	maxCrewInviteCodeGenAttempts int           = 8
	coachSessionSignedURLTTL     time.Duration = 15 * time.Minute
)

func (s *Server) GetCrews(ctx context.Context, _ generated.GetCrewsRequestObject) (generated.GetCrewsResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetCrews401JSONResponse{Message: "unauthorized"}, nil
	}

	rows, err := s.queries.ListCrewsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed listing crews", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetCrews500JSONResponse{Message: "could not list crews"}, nil
	}

	crews := make([]generated.Crew, 0, len(rows))
	for _, row := range rows {
		crews = append(crews, toAPICrewFromListRow(row))
	}

	return generated.GetCrews200JSONResponse{
		Crews: crews,
	}, nil
}

func (s *Server) PostCrews(ctx context.Context, request generated.PostCrewsRequestObject) (generated.PostCrewsResponseObject, error) {
	if request.Body == nil {
		return generated.PostCrews400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostCrews401JSONResponse{Message: "unauthorized"}, nil
	}

	name := strings.TrimSpace(request.Body.Name)
	if len(name) < 2 || len(name) > 80 {
		return generated.PostCrews400JSONResponse{Message: "name must be 2-80 characters"}, nil
	}

	description := ""
	if request.Body.Description != nil {
		description = strings.TrimSpace(*request.Body.Description)
	}

	isPrivate := true
	if request.Body.IsPrivate != nil {
		isPrivate = *request.Body.IsPrivate
	}

	row, err := s.queries.CreateCrew(ctx, db.CreateCrewParams{
		Name:            name,
		Description:     description,
		CreatedByUserID: userID,
		IsPrivate:       isPrivate,
		SharedPlanUrl:   nullableStringParam(request.Body.SharedPlanUrl),
		SharedHabitsUrl: nullableStringParam(request.Body.SharedHabitsUrl),
	})
	if err != nil {
		s.logger.Error("failed creating crew", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PostCrews500JSONResponse{Message: "could not create crew"}, nil
	}

	if _, err := s.queries.AddCrewMember(ctx, db.AddCrewMemberParams{
		CrewID: row.ID,
		UserID: userID,
		Role:   string(generated.Owner),
	}); err != nil {
		s.logger.Error("failed creating owner crew membership", zap.Error(err), zap.String("crew_id", row.ID.String()), zap.String("user_id", userID.String()))
		return generated.PostCrews500JSONResponse{Message: "could not create crew"}, nil
	}

	responsePayload, err := s.buildCrewResponseForUser(ctx, row.ID, userID)
	if err != nil {
		s.logger.Error("failed building crew response", zap.Error(err), zap.String("crew_id", row.ID.String()), zap.String("user_id", userID.String()))
		return generated.PostCrews500JSONResponse{Message: "could not create crew"}, nil
	}

	return generated.PostCrews201JSONResponse(responsePayload), nil
}

func (s *Server) GetCrewsId(ctx context.Context, request generated.GetCrewsIdRequestObject) (generated.GetCrewsIdResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetCrewsId401JSONResponse{Message: "unauthorized"}, nil
	}

	crewID := uuid.UUID(request.Id)
	if _, err := s.queries.GetCrewByID(ctx, crewID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetCrewsId404JSONResponse{Message: "crew not found"}, nil
		}
		s.logger.Error("failed fetching crew by id", zap.Error(err), zap.String("crew_id", crewID.String()))
		return generated.GetCrewsId500JSONResponse{Message: "could not fetch crew"}, nil
	}

	responsePayload, err := s.buildCrewResponseForUser(ctx, crewID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetCrewsId403JSONResponse{Message: "forbidden"}, nil
		}
		s.logger.Error("failed building crew detail response", zap.Error(err), zap.String("crew_id", crewID.String()), zap.String("user_id", userID.String()))
		return generated.GetCrewsId500JSONResponse{Message: "could not fetch crew"}, nil
	}

	return generated.GetCrewsId200JSONResponse(responsePayload), nil
}

func (s *Server) PostCrewsIdInvites(
	ctx context.Context,
	request generated.PostCrewsIdInvitesRequestObject,
) (generated.PostCrewsIdInvitesResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostCrewsIdInvites401JSONResponse{Message: "unauthorized"}, nil
	}

	crewID := uuid.UUID(request.Id)
	if _, err := s.queries.GetCrewByID(ctx, crewID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostCrewsIdInvites404JSONResponse{Message: "crew not found"}, nil
		}
		s.logger.Error("failed fetching crew for invite creation", zap.Error(err), zap.String("crew_id", crewID.String()), zap.String("user_id", userID.String()))
		return generated.PostCrewsIdInvites500JSONResponse{Message: "could not create invite"}, nil
	}

	membership, err := s.queries.GetCrewMemberByCrewIDAndUserID(ctx, db.GetCrewMemberByCrewIDAndUserIDParams{
		CrewID: crewID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostCrewsIdInvites403JSONResponse{Message: "forbidden"}, nil
		}
		s.logger.Error("failed checking crew membership for invite creation", zap.Error(err), zap.String("crew_id", crewID.String()), zap.String("user_id", userID.String()))
		return generated.PostCrewsIdInvites500JSONResponse{Message: "could not create invite"}, nil
	}
	if membership.Role != string(generated.Owner) && membership.Role != string(generated.Member) {
		return generated.PostCrewsIdInvites403JSONResponse{Message: "forbidden"}, nil
	}

	maxUses := defaultCrewInviteMaxUses
	expiresAt := sql.NullTime{}
	if request.Body != nil {
		if request.Body.MaxUses != nil {
			maxUses = *request.Body.MaxUses
		}
		if request.Body.ExpiresAt != nil {
			expiry := request.Body.ExpiresAt.UTC()
			if !expiry.After(s.currentTime().UTC()) {
				return generated.PostCrewsIdInvites400JSONResponse{Message: "expiresAt must be in the future"}, nil
			}
			expiresAt = sql.NullTime{Time: expiry, Valid: true}
		}
	}
	if maxUses < 1 || maxUses > maxCrewInviteMaxUses {
		return generated.PostCrewsIdInvites400JSONResponse{Message: "maxUses must be between 1 and 100"}, nil
	}

	var invite db.CrewInvite
	created := false
	for attempt := 0; attempt < maxCrewInviteCodeGenAttempts; attempt++ {
		inviteCode, err := generateCrewInviteCode()
		if err != nil {
			s.logger.Error("failed generating crew invite code", zap.Error(err), zap.String("crew_id", crewID.String()))
			return generated.PostCrewsIdInvites500JSONResponse{Message: "could not create invite"}, nil
		}

		invite, err = s.queries.CreateCrewInvite(ctx, db.CreateCrewInviteParams{
			CrewID:          crewID,
			InviteCode:      inviteCode,
			InvitedByUserID: userID,
			MaxUses:         maxUses,
			ExpiresAt:       expiresAt,
		})
		if err == nil {
			created = true
			break
		}
		if isUniqueViolation(err) {
			continue
		}

		s.logger.Error("failed creating crew invite", zap.Error(err), zap.String("crew_id", crewID.String()), zap.String("user_id", userID.String()))
		return generated.PostCrewsIdInvites500JSONResponse{Message: "could not create invite"}, nil
	}

	if !created {
		s.logger.Error("failed creating unique crew invite code", zap.String("crew_id", crewID.String()), zap.String("user_id", userID.String()))
		return generated.PostCrewsIdInvites500JSONResponse{Message: "could not create invite"}, nil
	}

	return generated.PostCrewsIdInvites201JSONResponse{
		Invite: toAPICrewInvite(invite),
	}, nil
}

func (s *Server) PostCrewsJoin(ctx context.Context, request generated.PostCrewsJoinRequestObject) (generated.PostCrewsJoinResponseObject, error) {
	if request.Body == nil {
		return generated.PostCrewsJoin400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostCrewsJoin401JSONResponse{Message: "unauthorized"}, nil
	}

	inviteCode := normalizeCrewInviteCode(request.Body.InviteCode)
	if inviteCode == "" {
		return generated.PostCrewsJoin400JSONResponse{Message: "inviteCode is required"}, nil
	}

	invite, err := s.queries.GetActiveCrewInviteByCode(ctx, inviteCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostCrewsJoin404JSONResponse{Message: "invite not found or expired"}, nil
		}
		s.logger.Error("failed loading active crew invite by code", zap.Error(err), zap.String("invite_code", inviteCode), zap.String("user_id", userID.String()))
		return generated.PostCrewsJoin500JSONResponse{Message: "could not join crew"}, nil
	}

	joined := false
	_, membershipErr := s.queries.GetCrewMemberByCrewIDAndUserID(ctx, db.GetCrewMemberByCrewIDAndUserIDParams{
		CrewID: invite.CrewID,
		UserID: userID,
	})
	switch {
	case errors.Is(membershipErr, sql.ErrNoRows):
		if _, err := s.queries.AddCrewMember(ctx, db.AddCrewMemberParams{
			CrewID: invite.CrewID,
			UserID: userID,
			Role:   string(generated.Member),
		}); err != nil {
			s.logger.Error("failed adding crew member from invite", zap.Error(err), zap.String("crew_id", invite.CrewID.String()), zap.String("user_id", userID.String()), zap.String("invite_code", inviteCode))
			return generated.PostCrewsJoin500JSONResponse{Message: "could not join crew"}, nil
		}

		if _, err := s.queries.IncrementCrewInviteUse(ctx, invite.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return generated.PostCrewsJoin404JSONResponse{Message: "invite not found or expired"}, nil
			}
			s.logger.Error("failed incrementing crew invite use", zap.Error(err), zap.String("invite_id", invite.ID.String()), zap.String("crew_id", invite.CrewID.String()))
			return generated.PostCrewsJoin500JSONResponse{Message: "could not join crew"}, nil
		}

		joined = true
	case membershipErr != nil:
		s.logger.Error("failed checking existing crew membership", zap.Error(membershipErr), zap.String("crew_id", invite.CrewID.String()), zap.String("user_id", userID.String()))
		return generated.PostCrewsJoin500JSONResponse{Message: "could not join crew"}, nil
	}

	crewResponse, err := s.buildCrewResponseForUser(ctx, invite.CrewID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostCrewsJoin404JSONResponse{Message: "crew not found"}, nil
		}
		s.logger.Error("failed building joined crew response", zap.Error(err), zap.String("crew_id", invite.CrewID.String()), zap.String("user_id", userID.String()))
		return generated.PostCrewsJoin500JSONResponse{Message: "could not join crew"}, nil
	}

	return generated.PostCrewsJoin200JSONResponse{
		Crew:   crewResponse.Crew,
		Joined: joined,
	}, nil
}

func (s *Server) GetCoachSessions(ctx context.Context, _ generated.GetCoachSessionsRequestObject) (generated.GetCoachSessionsResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetCoachSessions401JSONResponse{Message: "unauthorized"}, nil
	}

	rows, err := s.queries.ListCoachSessionsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed listing coach sessions", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetCoachSessions500JSONResponse{Message: "could not list coach sessions"}, nil
	}

	sessions := make([]generated.CoachSessionSummary, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, toAPICoachSessionSummaryFromListRow(row))
	}

	return generated.GetCoachSessions200JSONResponse{
		Sessions: sessions,
	}, nil
}

func (s *Server) GetCoachSessionsId(
	ctx context.Context,
	request generated.GetCoachSessionsIdRequestObject,
) (generated.GetCoachSessionsIdResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetCoachSessionsId401JSONResponse{Message: "unauthorized"}, nil
	}

	sessionID := uuid.UUID(request.Id)
	sessionRow, err := s.queries.GetCoachSessionByIDForUser(ctx, db.GetCoachSessionByIDForUserParams{
		ID:     sessionID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if _, lookupErr := s.queries.GetCoachSessionByID(ctx, sessionID); lookupErr != nil {
				if errors.Is(lookupErr, sql.ErrNoRows) {
					return generated.GetCoachSessionsId404JSONResponse{Message: "coach session not found"}, nil
				}
				s.logger.Error("failed checking coach session visibility", zap.Error(lookupErr), zap.String("coach_session_id", sessionID.String()), zap.String("user_id", userID.String()))
				return generated.GetCoachSessionsId500JSONResponse{Message: "could not fetch coach session"}, nil
			}
			return generated.GetCoachSessionsId403JSONResponse{Message: "forbidden"}, nil
		}

		s.logger.Error("failed fetching coach session detail", zap.Error(err), zap.String("coach_session_id", sessionID.String()), zap.String("user_id", userID.String()))
		return generated.GetCoachSessionsId500JSONResponse{Message: "could not fetch coach session"}, nil
	}

	userEntitlements := entitlement.NewSnapshot(nil)
	if s.entitlement != nil {
		snapshot, snapshotErr := s.entitlement.SnapshotForUser(ctx, userID)
		if snapshotErr != nil {
			s.logger.Error(
				"failed resolving coach session entitlements",
				zap.Error(snapshotErr),
				zap.String("coach_session_id", sessionID.String()),
				zap.String("user_id", userID.String()),
			)
			return generated.GetCoachSessionsId500JSONResponse{Message: "could not fetch coach session"}, nil
		}
		userEntitlements = snapshot
	}

	requiredTier := entitlement.ParseCoachTier(sessionRow.RequiredTier)
	if !userEntitlements.HasCoachTier(requiredTier) {
		return generated.GetCoachSessionsId403JSONResponse{Message: "subscription tier required"}, nil
	}

	assetsRows, err := s.queries.ListCoachSessionAssetsBySessionID(ctx, sessionID)
	if err != nil {
		s.logger.Error("failed listing coach session assets", zap.Error(err), zap.String("coach_session_id", sessionID.String()), zap.String("user_id", userID.String()))
		return generated.GetCoachSessionsId500JSONResponse{Message: "could not fetch coach session"}, nil
	}

	assets := make([]generated.CoachSessionAsset, 0, len(assetsRows))
	for _, asset := range assetsRows {
		signedURL := asset.StorageKey
		if s.assetStorage != nil {
			resolvedURL, resolveErr := s.assetStorage.SignedURL(ctx, asset.StorageKey, coachSessionSignedURLTTL)
			if resolveErr != nil {
				s.logger.Error("failed resolving signed url for coach session asset", zap.Error(resolveErr), zap.String("coach_session_id", sessionID.String()), zap.String("asset_id", asset.ID.String()), zap.String("storage_key", asset.StorageKey))
				return generated.GetCoachSessionsId500JSONResponse{Message: "could not fetch coach session"}, nil
			}
			signedURL = resolvedURL
		}

		assets = append(assets, generated.CoachSessionAsset{
			Id:         openapi_types.UUID(asset.ID),
			AssetType:  generated.CoachSessionAssetAssetType(asset.AssetType),
			StorageKey: asset.StorageKey,
			SignedUrl:  signedURL,
			MimeType:   asset.MimeType,
			CreatedAt:  asset.CreatedAt,
		})
	}

	cueRows, err := s.queries.ListCueTimelineBySessionID(ctx, sessionID)
	if err != nil {
		s.logger.Error("failed listing coach session cue timeline", zap.Error(err), zap.String("coach_session_id", sessionID.String()), zap.String("user_id", userID.String()))
		return generated.GetCoachSessionsId500JSONResponse{Message: "could not fetch coach session"}, nil
	}

	cues := make([]generated.CueTimelineItem, 0, len(cueRows))
	for _, cue := range cueRows {
		var exerciseID *openapi_types.UUID
		if cue.BiomechanicsExerciseID.Valid {
			value := openapi_types.UUID(cue.BiomechanicsExerciseID.UUID)
			exerciseID = &value
		}

		cues = append(cues, generated.CueTimelineItem{
			Id:                         openapi_types.UUID(cue.ID),
			CueIndex:                   cue.CueIndex,
			StartMs:                    cue.StartMs,
			EndMs:                      cue.EndMs,
			CueText:                    cue.CueText,
			BiomechanicsExerciseId:     exerciseID,
			BiomechanicsDefinitionType: generated.CueTimelineItemBiomechanicsDefinitionType(cue.BiomechanicsDefinitionType),
			BiomechanicsDefinitionKey:  cue.BiomechanicsDefinitionKey,
		})
	}

	return generated.GetCoachSessionsId200JSONResponse{
		Session: generated.CoachSession{
			Id:              openapi_types.UUID(sessionRow.ID),
			CrewId:          openapi_types.UUID(sessionRow.CrewID),
			CrewName:        sessionRow.CrewName,
			Title:           sessionRow.Title,
			Description:     sessionRow.Description,
			CoachName:       sessionRow.CoachName,
			DurationSeconds: sessionRow.DurationSeconds,
			RequiredTier:    generated.CoachTier(requiredTier),
			CreatedAt:       sessionRow.CreatedAt,
			Assets:          assets,
			Cues:            cues,
		},
	}, nil
}

func (s *Server) buildCrewResponseForUser(ctx context.Context, crewID uuid.UUID, userID uuid.UUID) (generated.CrewResponse, error) {
	crewRow, err := s.queries.GetCrewByIDForUser(ctx, db.GetCrewByIDForUserParams{
		ID:     crewID,
		UserID: userID,
	})
	if err != nil {
		return generated.CrewResponse{}, err
	}

	memberRows, err := s.queries.ListCrewMembersByCrewID(ctx, crewID)
	if err != nil {
		return generated.CrewResponse{}, err
	}

	members := make([]generated.CrewMember, 0, len(memberRows))
	for _, memberRow := range memberRows {
		members = append(members, toAPICrewMember(memberRow))
	}

	return generated.CrewResponse{
		Crew:    toAPICrewFromDetailRow(crewRow),
		Members: members,
	}, nil
}

func toAPICrewFromListRow(row db.ListCrewsByUserIDRow) generated.Crew {
	return generated.Crew{
		Id:              openapi_types.UUID(row.ID),
		Name:            row.Name,
		Description:     row.Description,
		CreatedByUserId: openapi_types.UUID(row.CreatedByUserID),
		IsPrivate:       row.IsPrivate,
		SharedPlanUrl:   nullableStringPointer(row.SharedPlanUrl),
		SharedHabitsUrl: nullableStringPointer(row.SharedHabitsUrl),
		MyRole:          generated.CrewRole(row.MyRole),
		MemberCount:     row.MemberCount,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func toAPICrewFromDetailRow(row db.GetCrewByIDForUserRow) generated.Crew {
	return generated.Crew{
		Id:              openapi_types.UUID(row.ID),
		Name:            row.Name,
		Description:     row.Description,
		CreatedByUserId: openapi_types.UUID(row.CreatedByUserID),
		IsPrivate:       row.IsPrivate,
		SharedPlanUrl:   nullableStringPointer(row.SharedPlanUrl),
		SharedHabitsUrl: nullableStringPointer(row.SharedHabitsUrl),
		MyRole:          generated.CrewRole(row.MyRole),
		MemberCount:     row.MemberCount,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func toAPICrewMember(row db.ListCrewMembersByCrewIDRow) generated.CrewMember {
	var displayName *string
	if trimmed := strings.TrimSpace(row.DisplayName); trimmed != "" {
		displayName = &trimmed
	}

	return generated.CrewMember{
		UserId:      openapi_types.UUID(row.UserID),
		Email:       openapi_types.Email(row.Email),
		DisplayName: displayName,
		Role:        generated.CrewRole(row.Role),
		JoinedAt:    row.JoinedAt,
	}
}

func toAPICrewInvite(row db.CrewInvite) generated.CrewInvite {
	var expiresAt *time.Time
	if row.ExpiresAt.Valid {
		value := row.ExpiresAt.Time
		expiresAt = &value
	}

	return generated.CrewInvite{
		Id:              openapi_types.UUID(row.ID),
		CrewId:          openapi_types.UUID(row.CrewID),
		InviteCode:      row.InviteCode,
		InvitedByUserId: openapi_types.UUID(row.InvitedByUserID),
		MaxUses:         row.MaxUses,
		UsesCount:       row.UsesCount,
		ExpiresAt:       expiresAt,
		CreatedAt:       row.CreatedAt,
	}
}

func toAPICoachSessionSummaryFromListRow(row db.ListCoachSessionsByUserIDRow) generated.CoachSessionSummary {
	return generated.CoachSessionSummary{
		Id:              openapi_types.UUID(row.ID),
		CrewId:          openapi_types.UUID(row.CrewID),
		CrewName:        row.CrewName,
		Title:           row.Title,
		Description:     row.Description,
		CoachName:       row.CoachName,
		DurationSeconds: row.DurationSeconds,
		RequiredTier:    generated.CoachTier(entitlement.ParseCoachTier(row.RequiredTier)),
		CreatedAt:       row.CreatedAt,
	}
}

func nullableStringParam(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return sql.NullString{}
	}

	return sql.NullString{
		String: trimmed,
		Valid:  true,
	}
}

func nullableStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}

	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func normalizeCrewInviteCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func generateCrewInviteCode() (string, error) {
	buffer := make([]byte, crewInviteCodeByteLength)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return strings.ToUpper(hex.EncodeToString(buffer)), nil
}
