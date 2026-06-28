package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/atlas/atlas-api/internal/analytics"
	"github.com/atlas/atlas-api/internal/auth"
	"github.com/atlas/atlas-api/internal/biomechanics"
	"github.com/atlas/atlas-api/internal/config"
	"github.com/atlas/atlas-api/internal/consent"
	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/entitlement"
	"github.com/atlas/atlas-api/internal/exercise"
	"github.com/atlas/atlas-api/internal/food"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/atlas/atlas-api/internal/onboarding"
	"github.com/atlas/atlas-api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/sqlc-dev/pqtype"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	logger       *zap.Logger
	cfg          config.Config
	queries      db.Querier
	queryDB      db.DBTX
	assetStorage storage.Storage
	tokenSvc     *auth.TokenService
	exerciseSvc  *exercise.Service
	biomechSvc   *biomechanics.Service
	analyticsSvc *analytics.Service
	foodSvc      *food.Service
	entitlement  *entitlement.Service
	currentTime  func() time.Time
}

func NewServer(logger *zap.Logger, cfg config.Config, queries db.Querier, tokenSvc *auth.TokenService) *Server {
	usdaProvider := food.NewUSDAProvider(cfg.USDAAPIBaseURL, cfg.USDAAPIKey, cfg.FoodDetailsCacheTTL())
	edamamUPCProvider := food.NewEdamamUPCProvider(cfg.EdamamAPIBaseURL, cfg.EdamamAppID, cfg.EdamamAppKey)
	entitlementSvc := entitlement.NewService(queries)
	assetStorage := resolveAssetStorage(logger, cfg)

	var queryDB db.DBTX
	if provider, ok := queries.(interface{ DBTX() db.DBTX }); ok {
		queryDB = provider.DBTX()
	}

	return &Server{
		logger:       logger.Named("httpapi"),
		cfg:          cfg,
		queries:      queries,
		queryDB:      queryDB,
		assetStorage: assetStorage,
		tokenSvc:     tokenSvc,
		exerciseSvc:  exercise.NewService(queries, assetStorage),
		biomechSvc:   biomechanics.NewService(queries, queryDB, assetStorage),
		analyticsSvc: analytics.NewService(queries),
		foodSvc:      food.NewService(logger, queries, food.USDAProviderName, edamamUPCProvider, usdaProvider),
		entitlement:  entitlementSvc,
		currentTime:  time.Now,
	}
}

func resolveAssetStorage(logger *zap.Logger, cfg config.Config) storage.Storage {
	backend := strings.ToLower(strings.TrimSpace(cfg.AssetStorageBackend))
	if backend == "s3" || backend == "minio" {
		s3Storage, err := storage.NewS3Storage(storage.S3StorageConfig{
			Endpoint:      cfg.MinioEndpoint,
			AccessKeyID:   cfg.MinioAccess,
			SecretAccess:  cfg.MinioSecret,
			DefaultBucket: cfg.AssetStorageBucket,
			UseSSL:        cfg.MinioUseSSL,
		})
		if err == nil {
			return s3Storage
		}

		logger.Warn(
			"failed to initialize s3 storage, falling back to local file storage",
			zap.Error(err),
			zap.String("backend", backend),
		)
	}

	return storage.NewLocalFileStorage(".")
}

func (s *Server) GetHealth(_ context.Context, _ generated.GetHealthRequestObject) (generated.GetHealthResponseObject, error) {
	return generated.GetHealth200JSONResponse{
		Status:    "ok",
		Service:   s.cfg.ServiceName,
		Env:       s.cfg.Env,
		Timestamp: s.currentTime().UTC(),
	}, nil
}

func (s *Server) PostAuthRegister(ctx context.Context, request generated.PostAuthRegisterRequestObject) (generated.PostAuthRegisterResponseObject, error) {
	if request.Body == nil {
		return generated.PostAuthRegister400JSONResponse{Message: "request body is required"}, nil
	}

	email := normalizeEmail(string(request.Body.Email))
	password := strings.TrimSpace(request.Body.Password)
	if email == "" || password == "" {
		return generated.PostAuthRegister400JSONResponse{Message: "email and password are required"}, nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("failed to hash password", zap.Error(err))
		return generated.PostAuthRegister400JSONResponse{Message: "invalid credentials"}, nil
	}

	createdUser, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: string(passwordHash),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return generated.PostAuthRegister409JSONResponse{Message: "user already exists"}, nil
		}
		s.logger.Error("failed creating user", zap.Error(err), zap.String("email", email))
		return generated.PostAuthRegister400JSONResponse{Message: "could not create user"}, nil
	}

	tokens, err := s.issueSessionTokens(ctx, createdUser.ID)
	if err != nil {
		s.logger.Error("failed issuing register tokens", zap.Error(err), zap.String("email", email))
		return generated.PostAuthRegister400JSONResponse{Message: "could not create user session"}, nil
	}

	return generated.PostAuthRegister201JSONResponse{
		User:   s.toAPIUser(ctx, createdUser),
		Tokens: tokens,
	}, nil
}

func (s *Server) PostAuthLogin(ctx context.Context, request generated.PostAuthLoginRequestObject) (generated.PostAuthLoginResponseObject, error) {
	if request.Body == nil {
		return generated.PostAuthLogin400JSONResponse{Message: "request body is required"}, nil
	}

	email := normalizeEmail(string(request.Body.Email))
	password := strings.TrimSpace(request.Body.Password)
	if email == "" || password == "" {
		return generated.PostAuthLogin400JSONResponse{Message: "email and password are required"}, nil
	}

	userRecord, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostAuthLogin401JSONResponse{Message: "invalid credentials"}, nil
		}
		s.logger.Error("failed fetching user by email", zap.Error(err), zap.String("email", email))
		return generated.PostAuthLogin400JSONResponse{Message: "invalid credentials"}, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(userRecord.PasswordHash), []byte(password)); err != nil {
		return generated.PostAuthLogin401JSONResponse{Message: "invalid credentials"}, nil
	}

	tokens, err := s.issueSessionTokens(ctx, userRecord.ID)
	if err != nil {
		s.logger.Error("failed issuing login tokens", zap.Error(err), zap.String("email", email))
		return generated.PostAuthLogin400JSONResponse{Message: "could not create user session"}, nil
	}

	return generated.PostAuthLogin200JSONResponse{
		User:   s.toAPIUser(ctx, userRecord),
		Tokens: tokens,
	}, nil
}

func (s *Server) PostAuthRefresh(ctx context.Context, request generated.PostAuthRefreshRequestObject) (generated.PostAuthRefreshResponseObject, error) {
	if request.Body == nil {
		return generated.PostAuthRefresh400JSONResponse{Message: "request body is required"}, nil
	}

	refreshToken := strings.TrimSpace(request.Body.RefreshToken)
	if refreshToken == "" {
		return generated.PostAuthRefresh400JSONResponse{Message: "refreshToken is required"}, nil
	}

	hash := auth.HashToken(refreshToken)
	sessionRecord, err := s.queries.GetActiveSessionByRefreshTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostAuthRefresh401JSONResponse{Message: "invalid refresh token"}, nil
		}
		s.logger.Error("failed fetching active session", zap.Error(err))
		return generated.PostAuthRefresh400JSONResponse{Message: "invalid refresh token"}, nil
	}

	if _, err := s.queries.RevokeSessionByID(ctx, sessionRecord.ID); err != nil {
		s.logger.Error("failed revoking session", zap.Error(err), zap.String("session_id", sessionRecord.ID.String()))
		return generated.PostAuthRefresh400JSONResponse{Message: "invalid refresh token"}, nil
	}

	tokens, err := s.issueSessionTokens(ctx, sessionRecord.UserID)
	if err != nil {
		s.logger.Error("failed issuing refreshed tokens", zap.Error(err), zap.String("user_id", sessionRecord.UserID.String()))
		return generated.PostAuthRefresh400JSONResponse{Message: "invalid refresh token"}, nil
	}

	return generated.PostAuthRefresh200JSONResponse(tokens), nil
}

func (s *Server) PostAuthLogout(ctx context.Context, request generated.PostAuthLogoutRequestObject) (generated.PostAuthLogoutResponseObject, error) {
	if request.Body == nil {
		return generated.PostAuthLogout400JSONResponse{Message: "request body is required"}, nil
	}

	refreshToken := strings.TrimSpace(request.Body.RefreshToken)
	if refreshToken == "" {
		return generated.PostAuthLogout400JSONResponse{Message: "refreshToken is required"}, nil
	}

	hash := auth.HashToken(refreshToken)
	if _, err := s.queries.RevokeSessionByRefreshTokenHash(ctx, hash); err != nil {
		s.logger.Error("failed revoking session by refresh hash", zap.Error(err))
		return generated.PostAuthLogout400JSONResponse{Message: "could not revoke session"}, nil
	}

	return generated.PostAuthLogout204Response{}, nil
}

func (s *Server) GetMe(ctx context.Context, _ generated.GetMeRequestObject) (generated.GetMeResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetMe401JSONResponse{Message: "unauthorized"}, nil
	}

	userRecord, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetMe401JSONResponse{Message: "unauthorized"}, nil
		}
		s.logger.Error("failed fetching current user", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetMe401JSONResponse{Message: "unauthorized"}, nil
	}

	return generated.GetMe200JSONResponse{User: s.toAPIUser(ctx, userRecord)}, nil
}

func (s *Server) GetConsents(ctx context.Context, _ generated.GetConsentsRequestObject) (generated.GetConsentsResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetConsents401JSONResponse{Message: "unauthorized"}, nil
	}

	consentRows, err := s.queries.ListConsentsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("failed listing consents", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetConsents401JSONResponse{Message: "unauthorized"}, nil
	}

	consents := make([]generated.Consent, 0, len(consentRows))
	for _, row := range consentRows {
		consents = append(consents, toAPIConsent(row))
	}

	return generated.GetConsents200JSONResponse{Consents: consents}, nil
}

func (s *Server) GetOnboardingStatus(ctx context.Context, _ generated.GetOnboardingStatusRequestObject) (generated.GetOnboardingStatusResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetOnboardingStatus401JSONResponse{Message: "unauthorized"}, nil
	}

	status, err := s.queries.GetOnboardingStatus(ctx, userID)
	if err != nil {
		s.logger.Error("failed fetching onboarding status", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetOnboardingStatus401JSONResponse{Message: "unauthorized"}, nil
	}

	profileCompleted := status.HasProfile
	goalsCompleted := false
	var firstWeekPlan *generated.FirstWeekPlanSummary
	var trainingProfile *generated.TrainingProfile
	var planExplanation *string

	if status.HasGoals {
		goalsRow, err := s.queries.GetUserGoalsByUserID(ctx, userID)
		if err != nil {
			s.logger.Error("failed fetching onboarding goals details", zap.Error(err), zap.String("user_id", userID.String()))
			return generated.GetOnboardingStatus401JSONResponse{Message: "unauthorized"}, nil
		}

		trainingProfileValue, err := buildTrainingProfile(goalsRow)
		if err != nil {
			s.logger.Error("failed building training profile", zap.Error(err), zap.String("user_id", userID.String()))
			return generated.GetOnboardingStatus401JSONResponse{Message: "unauthorized"}, nil
		}
		trainingProfile = &trainingProfileValue

		goalsCompleted = isGoalsSelectionComplete(goalsRow, trainingProfileValue)
		if goalsCompleted && trainingProfile != nil {
			plan := buildFirstWeekPlanSummary(goalsRow)
			firstWeekPlan = &plan

			explanation := buildOnboardingPlanExplanation(*trainingProfile, plan)
			planExplanation = &explanation
		}
	}

	return generated.GetOnboardingStatus200JSONResponse{
		ProfileCompleted:    profileCompleted,
		GoalsCompleted:      goalsCompleted,
		OnboardingCompleted: goalsCompleted,
		FirstWeekPlan:       firstWeekPlan,
		TrainingProfile:     trainingProfile,
		PlanExplanation:     planExplanation,
	}, nil
}

func (s *Server) GetPrograms(ctx context.Context, _ generated.GetProgramsRequestObject) (generated.GetProgramsResponseObject, error) {
	rows, err := s.queries.ListPrograms(ctx)
	if err != nil {
		s.logger.Error("failed listing programs", zap.Error(err))
		return generated.GetPrograms500JSONResponse{Message: "could not list programs"}, nil
	}

	programs := make([]generated.Program, 0, len(rows))
	for _, row := range rows {
		blockRows, err := s.queries.ListProgramBlocksByProgramID(ctx, row.ID)
		if err != nil {
			s.logger.Error("failed listing program blocks", zap.Error(err), zap.String("program_id", row.ID.String()))
			return generated.GetPrograms500JSONResponse{Message: "could not list programs"}, nil
		}

		blocks, _, _, weeklyFrequency := buildProgramBlocks(row.ID, row.WeeksLength, blockRows)
		program, err := toAPIProgram(row)
		if err != nil {
			s.logger.Error("failed mapping program row", zap.Error(err), zap.String("program_id", row.ID.String()))
			return generated.GetPrograms500JSONResponse{Message: "could not list programs"}, nil
		}
		program.Blocks = blocks
		program.WeeklyFrequency = weeklyFrequency
		programs = append(programs, program)
	}

	return generated.GetPrograms200JSONResponse{
		Programs: programs,
	}, nil
}

func (s *Server) PostProgramsEnroll(ctx context.Context, request generated.PostProgramsEnrollRequestObject) (generated.PostProgramsEnrollResponseObject, error) {
	if request.Body == nil {
		return generated.PostProgramsEnroll400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostProgramsEnroll401JSONResponse{Message: "unauthorized"}, nil
	}

	programID := uuid.UUID(request.Body.ProgramId)
	if _, err := s.queries.GetProgramByID(ctx, programID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostProgramsEnroll404JSONResponse{Message: "program not found"}, nil
		}
		s.logger.Error("failed fetching program by id", zap.Error(err), zap.String("program_id", programID.String()))
		return generated.PostProgramsEnroll400JSONResponse{Message: "could not enroll in program"}, nil
	}

	now := s.currentTime().UTC()
	startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	row, err := s.queries.UpsertUserProgramEnrollment(ctx, db.UpsertUserProgramEnrollmentParams{
		UserID:    userID,
		ProgramID: programID,
		StartDate: startDate,
	})
	if err != nil {
		s.logger.Error("failed upserting program enrollment", zap.Error(err), zap.String("user_id", userID.String()), zap.String("program_id", programID.String()))
		return generated.PostProgramsEnroll400JSONResponse{Message: "could not enroll in program"}, nil
	}

	return generated.PostProgramsEnroll200JSONResponse{
		Enrollment: toAPIProgramEnrollment(row),
	}, nil
}

func (s *Server) GetProgramsCurrent(ctx context.Context, _ generated.GetProgramsCurrentRequestObject) (generated.GetProgramsCurrentResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetProgramsCurrent401JSONResponse{Message: "unauthorized"}, nil
	}

	programData, err := s.loadCurrentProgramData(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, errNoActiveProgramEnrollment):
			return generated.GetProgramsCurrent404JSONResponse{Message: "no active program enrollment"}, nil
		case errors.Is(err, errActiveProgramNotFound):
			return generated.GetProgramsCurrent404JSONResponse{Message: "program not found"}, nil
		case errors.Is(err, errProgramScheduleNotFound):
			return generated.GetProgramsCurrent404JSONResponse{Message: "program week schedule not found"}, nil
		default:
			s.logger.Error("failed fetching current program context", zap.Error(err), zap.String("user_id", userID.String()))
			return generated.GetProgramsCurrent500JSONResponse{Message: "could not fetch current program schedule"}, nil
		}
	}

	scheduleContext := programData.buildScheduleContext(s.currentTime().UTC())
	templateBlock := programData.templateByWeek[scheduleContext.TemplateWeekIndex]

	preferences, err := programData.schedulePreferences()
	if err != nil {
		s.logger.Error("failed parsing schedule preferences", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetProgramsCurrent500JSONResponse{Message: "could not fetch current program schedule"}, nil
	}

	sessions, err := s.buildProgramSessionsForTemplateWeek(
		ctx,
		userID,
		templateBlock.ID,
		preferences,
		programData.weeklyFrequency,
		programData.programState,
	)
	if err != nil {
		s.logger.Error("failed building week sessions", zap.Error(err), zap.String("user_id", userID.String()), zap.String("template_week_id", templateBlock.ID.String()))
		return generated.GetProgramsCurrent500JSONResponse{Message: "could not fetch current program schedule"}, nil
	}

	program, err := programData.toAPIProgram()
	if err != nil {
		s.logger.Error("failed mapping program row", zap.Error(err), zap.String("program_id", programData.program.ID.String()))
		return generated.GetProgramsCurrent500JSONResponse{Message: "could not fetch current program schedule"}, nil
	}

	block := programData.blockByWeek[scheduleContext.BlockWeekIndex]
	weekID := block.Id
	if weekID == openapi_types.UUID(uuid.Nil) {
		weekID = openapi_types.UUID(templateBlock.ID)
	}

	return generated.GetProgramsCurrent200JSONResponse{
		Enrollment: toAPIProgramEnrollment(programData.enrollment),
		Program:    program,
		Context:    scheduleContext,
		Week: generated.ProgramWeekSchedule{
			Id:        weekID,
			WeekIndex: scheduleContext.BlockWeekIndex,
			Sessions:  sessions,
		},
	}, nil
}

func (s *Server) GetProgramsCurrentSessions(
	ctx context.Context,
	request generated.GetProgramsCurrentSessionsRequestObject,
) (generated.GetProgramsCurrentSessionsResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetProgramsCurrentSessions401JSONResponse{Message: "unauthorized"}, nil
	}

	programData, err := s.loadCurrentProgramData(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, errNoActiveProgramEnrollment):
			return generated.GetProgramsCurrentSessions404JSONResponse{Message: "no active program enrollment"}, nil
		case errors.Is(err, errActiveProgramNotFound):
			return generated.GetProgramsCurrentSessions404JSONResponse{Message: "program not found"}, nil
		case errors.Is(err, errProgramScheduleNotFound):
			return generated.GetProgramsCurrentSessions404JSONResponse{Message: "program week schedule not found"}, nil
		default:
			s.logger.Error("failed fetching current program context", zap.Error(err), zap.String("user_id", userID.String()))
			return generated.GetProgramsCurrentSessions500JSONResponse{Message: "could not fetch scheduled sessions"}, nil
		}
	}

	fromDate := normalizeDateUTC(s.currentTime().UTC())
	if request.Params.From != nil {
		fromDate = normalizeDateUTC(request.Params.From.Time)
	}

	toDate := fromDate.AddDate(0, 0, 13)
	if request.Params.To != nil {
		toDate = normalizeDateUTC(request.Params.To.Time)
	}

	if toDate.Before(fromDate) {
		return generated.GetProgramsCurrentSessions400JSONResponse{
			Message: "invalid date range: to must be on or after from",
		}, nil
	}
	if toDate.Sub(fromDate) > 90*24*time.Hour {
		return generated.GetProgramsCurrentSessions400JSONResponse{
			Message: "invalid date range: maximum span is 90 days",
		}, nil
	}

	sessions, err := s.generateScheduledSessionsInRange(ctx, userID, programData, fromDate, toDate)
	if err != nil {
		s.logger.Error(
			"failed generating scheduled sessions",
			zap.Error(err),
			zap.String("user_id", userID.String()),
			zap.String("from", fromDate.Format("2006-01-02")),
			zap.String("to", toDate.Format("2006-01-02")),
		)
		return generated.GetProgramsCurrentSessions500JSONResponse{Message: "could not fetch scheduled sessions"}, nil
	}

	program, err := programData.toAPIProgram()
	if err != nil {
		s.logger.Error("failed mapping program row", zap.Error(err), zap.String("program_id", programData.program.ID.String()))
		return generated.GetProgramsCurrentSessions500JSONResponse{Message: "could not fetch scheduled sessions"}, nil
	}

	return generated.GetProgramsCurrentSessions200JSONResponse{
		Enrollment: toAPIProgramEnrollment(programData.enrollment),
		Program:    program,
		Context:    programData.buildScheduleContext(s.currentTime().UTC()),
		Sessions:   sessions,
	}, nil
}

func (s *Server) GetExercises(ctx context.Context, request generated.GetExercisesRequestObject) (generated.GetExercisesResponseObject, error) {
	filter := exercise.ListFilter{}
	if request.Params.Query != nil {
		filter.Query = *request.Params.Query
	}
	if request.Params.Equipment != nil {
		filter.Equipment = *request.Params.Equipment
	}
	if request.Params.Pattern != nil {
		filter.Pattern = *request.Params.Pattern
	}

	exercises, err := s.exerciseSvc.List(ctx, filter)
	if err != nil {
		s.logger.Error("failed listing exercises", zap.Error(err), zap.Any("filter", filter))
		return generated.GetExercises500JSONResponse{Message: "could not list exercises"}, nil
	}

	apiExercises := make([]generated.Exercise, 0, len(exercises))
	for _, catalogExercise := range exercises {
		apiExercises = append(apiExercises, toAPIExercise(catalogExercise))
	}

	return generated.GetExercises200JSONResponse{
		Exercises: apiExercises,
	}, nil
}

func (s *Server) GetExerciseById(ctx context.Context, request generated.GetExerciseByIdRequestObject) (generated.GetExerciseByIdResponseObject, error) {
	exerciseID := uuid.UUID(request.Id)
	catalogExercise, err := s.exerciseSvc.GetByID(ctx, exerciseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetExerciseById404JSONResponse{Message: "exercise not found"}, nil
		}
		s.logger.Error("failed fetching exercise by id", zap.Error(err), zap.String("exercise_id", exerciseID.String()))
		return generated.GetExerciseById500JSONResponse{Message: "could not fetch exercise"}, nil
	}

	return generated.GetExerciseById200JSONResponse{
		Exercise: toAPIExercise(catalogExercise),
	}, nil
}

func (s *Server) GetExerciseBiomechanicsById(
	ctx context.Context,
	request generated.GetExerciseBiomechanicsByIdRequestObject,
) (generated.GetExerciseBiomechanicsByIdResponseObject, error) {
	exerciseID := uuid.UUID(request.Id)
	biomechanicsPayload, err := s.biomechSvc.GetByExerciseID(ctx, exerciseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetExerciseBiomechanicsById404JSONResponse{Message: "exercise biomechanics not found"}, nil
		}

		s.logger.Error("failed fetching exercise biomechanics", zap.Error(err), zap.String("exercise_id", exerciseID.String()))
		return generated.GetExerciseBiomechanicsById500JSONResponse{Message: "could not fetch exercise biomechanics"}, nil
	}

	return generated.GetExerciseBiomechanicsById200JSONResponse{
		Biomechanics: toAPIExerciseBiomechanics(biomechanicsPayload),
	}, nil
}

func (s *Server) GetExerciseSubstitutesById(
	ctx context.Context,
	request generated.GetExerciseSubstitutesByIdRequestObject,
) (generated.GetExerciseSubstitutesByIdResponseObject, error) {
	exerciseID := uuid.UUID(request.Id)

	limit := int32(5)
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 20 {
		limit = 20
	}

	filter := exercise.SubstituteFilter{
		Limit: limit,
	}

	if request.Params.Constraints != nil {
		constraints, err := parseSubstituteConstraints(*request.Params.Constraints)
		if err != nil {
			return generated.GetExerciseSubstitutesById400JSONResponse{
				Message: "constraints must be a valid JSON object",
			}, nil
		}
		filter.Equipment = append(filter.Equipment, constraints.Equipment...)
		filter.InjuryFlags = append(filter.InjuryFlags, constraints.InjuryFlags...)
	}
	if request.Params.Equipment != nil {
		filter.Equipment = append(filter.Equipment, parseCSVTokens(*request.Params.Equipment)...)
	}
	if request.Params.InjuryFlags != nil {
		filter.InjuryFlags = append(filter.InjuryFlags, parseCSVTokens(*request.Params.InjuryFlags)...)
	}
	filter.Equipment = dedupeTokens(filter.Equipment)
	filter.InjuryFlags = dedupeTokens(filter.InjuryFlags)

	substitutes, err := s.exerciseSvc.ListSubstitutes(ctx, exerciseID, filter)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetExerciseSubstitutesById404JSONResponse{Message: "exercise not found"}, nil
		}

		s.logger.Error("failed listing exercise substitutes", zap.Error(err), zap.String("exercise_id", exerciseID.String()))
		return generated.GetExerciseSubstitutesById500JSONResponse{Message: "could not list substitutes"}, nil
	}

	apiSubstitutes := make([]generated.ExerciseSubstitute, 0, len(substitutes))
	for _, rankedSubstitute := range substitutes {
		apiSubstitutes = append(apiSubstitutes, toAPIExerciseSubstitute(rankedSubstitute))
	}

	return generated.GetExerciseSubstitutesById200JSONResponse{
		Substitutes: apiSubstitutes,
	}, nil
}

func (s *Server) PutProfile(ctx context.Context, request generated.PutProfileRequestObject) (generated.PutProfileResponseObject, error) {
	if request.Body == nil {
		return generated.PutProfile400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PutProfile401JSONResponse{Message: "unauthorized"}, nil
	}

	if err := onboarding.ValidateProfileInput(onboarding.ProfileInput{
		DisplayName:     request.Body.DisplayName,
		Sex:             request.Body.Sex,
		ExperienceLevel: request.Body.ExperienceLevel,
	}); err != nil {
		return generated.PutProfile400JSONResponse{Message: err.Error()}, nil
	}

	if request.Body.HeightCm <= 0 {
		return generated.PutProfile400JSONResponse{Message: "heightCm must be greater than 0"}, nil
	}
	if request.Body.WeightKg <= 0 {
		return generated.PutProfile400JSONResponse{Message: "weightKg must be greater than 0"}, nil
	}

	row, err := s.queries.UpsertUserProfile(ctx, db.UpsertUserProfileParams{
		UserID:          userID,
		DisplayName:     strings.TrimSpace(request.Body.DisplayName),
		Sex:             strings.TrimSpace(request.Body.Sex),
		HeightCm:        request.Body.HeightCm,
		WeightKg:        float64(request.Body.WeightKg),
		ExperienceLevel: strings.TrimSpace(request.Body.ExperienceLevel),
	})
	if err != nil {
		s.logger.Error("failed upserting user profile", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutProfile400JSONResponse{Message: "could not upsert profile"}, nil
	}

	return generated.PutProfile200JSONResponse{
		Profile: toAPIUserProfile(row),
	}, nil
}

func (s *Server) PutGoals(ctx context.Context, request generated.PutGoalsRequestObject) (generated.PutGoalsResponseObject, error) {
	if request.Body == nil {
		return generated.PutGoals400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PutGoals401JSONResponse{Message: "unauthorized"}, nil
	}

	if strings.TrimSpace(request.Body.PrimaryGoal) == "" {
		return generated.PutGoals400JSONResponse{Message: "primaryGoal is required"}, nil
	}

	equipmentAccess := request.Body.EquipmentAccessJson
	if equipmentAccess == nil {
		equipmentAccess = []string{}
	}

	constraints := request.Body.ConstraintsJson
	if constraints == nil {
		constraints = map[string]interface{}{}
	}

	injuriesLimitations := request.Body.InjuriesLimitationsJson
	if len(injuriesLimitations) == 0 {
		injuriesLimitations = []string{"none"}
	}

	modalityPreferences := request.Body.ModalityPreferencesJson
	if len(modalityPreferences) == 0 {
		modalityPreferences = []string{"general_fitness"}
	}

	var priorTrainingHistory map[string]interface{}
	if request.Body.PriorTrainingHistoryJson != nil {
		priorTrainingHistory = *request.Body.PriorTrainingHistoryJson
	}

	var readinessSignals map[string]interface{}
	if request.Body.ReadinessSignalsJson != nil {
		readinessSignals = *request.Body.ReadinessSignalsJson
	}

	if err := onboarding.ValidateGoalsInput(onboarding.GoalsInput{
		DaysPerWeek:            request.Body.DaysPerWeek,
		SessionDurationMinutes: request.Body.SessionDurationMinutes,
	}); err != nil {
		return generated.PutGoals400JSONResponse{Message: err.Error()}, nil
	}

	equipmentAccessJSON, err := json.Marshal(equipmentAccess)
	if err != nil {
		return generated.PutGoals400JSONResponse{Message: "invalid equipmentAccessJson"}, nil
	}

	constraintsJSON, err := json.Marshal(constraints)
	if err != nil {
		return generated.PutGoals400JSONResponse{Message: "invalid constraintsJson"}, nil
	}

	injuriesLimitationsJSON, err := json.Marshal(injuriesLimitations)
	if err != nil {
		return generated.PutGoals400JSONResponse{Message: "invalid injuriesLimitationsJson"}, nil
	}

	modalityPreferencesJSON, err := json.Marshal(modalityPreferences)
	if err != nil {
		return generated.PutGoals400JSONResponse{Message: "invalid modalityPreferencesJson"}, nil
	}

	priorTrainingHistoryJSON, err := marshalOptionalObjectValue(priorTrainingHistory)
	if err != nil {
		return generated.PutGoals400JSONResponse{Message: "invalid priorTrainingHistoryJson"}, nil
	}

	readinessSignalsJSON, err := marshalOptionalObjectValue(readinessSignals)
	if err != nil {
		return generated.PutGoals400JSONResponse{Message: "invalid readinessSignalsJson"}, nil
	}

	secondaryGoal := sql.NullString{}
	if request.Body.SecondaryGoal != nil {
		trimmed := strings.TrimSpace(*request.Body.SecondaryGoal)
		if trimmed != "" {
			secondaryGoal = sql.NullString{String: trimmed, Valid: true}
		}
	}

	row, err := s.queries.UpsertUserGoals(ctx, db.UpsertUserGoalsParams{
		UserID:                   userID,
		PrimaryGoal:              strings.TrimSpace(request.Body.PrimaryGoal),
		SecondaryGoal:            secondaryGoal,
		DaysPerWeek:              request.Body.DaysPerWeek,
		SessionDurationMinutes:   request.Body.SessionDurationMinutes,
		EquipmentAccessJson:      equipmentAccessJSON,
		ConstraintsJson:          constraintsJSON,
		InjuriesLimitationsJson:  injuriesLimitationsJSON,
		ModalityPreferencesJson:  modalityPreferencesJSON,
		PriorTrainingHistoryJson: priorTrainingHistoryJSON,
		ReadinessSignalsJson:     readinessSignalsJSON,
	})
	if err != nil {
		s.logger.Error("failed upserting user goals", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutGoals400JSONResponse{Message: "could not upsert goals"}, nil
	}

	goals, err := toAPIUserGoals(row)
	if err != nil {
		s.logger.Error("failed mapping user goals response", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutGoals400JSONResponse{Message: "could not upsert goals"}, nil
	}

	return generated.PutGoals200JSONResponse{
		Goals: goals,
	}, nil
}

func (s *Server) PutOnboardingProfile(
	ctx context.Context,
	request generated.PutOnboardingProfileRequestObject,
) (generated.PutOnboardingProfileResponseObject, error) {
	if request.Body == nil {
		return generated.PutOnboardingProfile400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PutOnboardingProfile401JSONResponse{Message: "unauthorized"}, nil
	}

	secondaryGoal := sql.NullString{}
	if request.Body.SecondaryGoal != nil {
		trimmed := strings.TrimSpace(*request.Body.SecondaryGoal)
		if trimmed != "" {
			secondaryGoal = sql.NullString{String: trimmed, Valid: true}
		}
	}

	equipmentAccess := request.Body.EquipmentAccessJson
	constraints := request.Body.ConstraintsJson

	var priorTrainingHistory map[string]interface{}
	if request.Body.PriorTrainingHistory != nil {
		priorTrainingHistory = *request.Body.PriorTrainingHistory
	}

	var readinessSignals map[string]interface{}
	if request.Body.ReadinessSignals != nil {
		readinessSignals = *request.Body.ReadinessSignals
	}

	if err := onboarding.ValidateTrainingProfileInput(onboarding.TrainingProfileInput{
		PrimaryGoal:            request.Body.PrimaryGoal,
		DaysPerWeek:            request.Body.DaysPerWeek,
		SessionDurationMinutes: request.Body.SessionDurationMinutes,
		EquipmentAccess:        equipmentAccess,
		Constraints:            constraints,
		InjuriesLimitations:    request.Body.InjuriesLimitationsFlags,
		ModalityPreferences:    request.Body.ModalityPreferences,
		PriorTrainingHistory:   priorTrainingHistory,
		ReadinessSignals:       readinessSignals,
	}); err != nil {
		return generated.PutOnboardingProfile400JSONResponse{Message: err.Error()}, nil
	}

	equipmentAccessJSON, err := json.Marshal(equipmentAccess)
	if err != nil {
		return generated.PutOnboardingProfile400JSONResponse{Message: "invalid equipmentAccessJson"}, nil
	}

	constraintsJSON, err := json.Marshal(constraints)
	if err != nil {
		return generated.PutOnboardingProfile400JSONResponse{Message: "invalid constraintsJson"}, nil
	}

	injuriesLimitationsJSON, err := json.Marshal(request.Body.InjuriesLimitationsFlags)
	if err != nil {
		return generated.PutOnboardingProfile400JSONResponse{Message: "invalid injuriesLimitationsFlags"}, nil
	}

	modalityPreferencesJSON, err := json.Marshal(request.Body.ModalityPreferences)
	if err != nil {
		return generated.PutOnboardingProfile400JSONResponse{Message: "invalid modalityPreferences"}, nil
	}

	priorTrainingHistoryJSON, err := marshalOptionalObjectValue(priorTrainingHistory)
	if err != nil {
		return generated.PutOnboardingProfile400JSONResponse{Message: "invalid priorTrainingHistory"}, nil
	}

	readinessSignalsJSON, err := marshalOptionalObjectValue(readinessSignals)
	if err != nil {
		return generated.PutOnboardingProfile400JSONResponse{Message: "invalid readinessSignals"}, nil
	}

	row, err := s.queries.UpsertUserGoals(ctx, db.UpsertUserGoalsParams{
		UserID:                   userID,
		PrimaryGoal:              strings.TrimSpace(request.Body.PrimaryGoal),
		SecondaryGoal:            secondaryGoal,
		DaysPerWeek:              request.Body.DaysPerWeek,
		SessionDurationMinutes:   request.Body.SessionDurationMinutes,
		EquipmentAccessJson:      equipmentAccessJSON,
		ConstraintsJson:          constraintsJSON,
		InjuriesLimitationsJson:  injuriesLimitationsJSON,
		ModalityPreferencesJson:  modalityPreferencesJSON,
		PriorTrainingHistoryJson: priorTrainingHistoryJSON,
		ReadinessSignalsJson:     readinessSignalsJSON,
	})
	if err != nil {
		s.logger.Error("failed upserting onboarding profile", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutOnboardingProfile400JSONResponse{Message: "could not upsert onboarding profile"}, nil
	}

	trainingProfile, err := buildTrainingProfile(row)
	if err != nil {
		s.logger.Error("failed building onboarding profile response", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.PutOnboardingProfile400JSONResponse{Message: "could not upsert onboarding profile"}, nil
	}

	return generated.PutOnboardingProfile200JSONResponse{
		TrainingProfile: trainingProfile,
	}, nil
}

func (s *Server) GetOnboardingPlan(
	ctx context.Context,
	_ generated.GetOnboardingPlanRequestObject,
) (generated.GetOnboardingPlanResponseObject, error) {
	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.GetOnboardingPlan401JSONResponse{Message: "unauthorized"}, nil
	}

	goalsRow, err := s.queries.GetUserGoalsByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.GetOnboardingPlan404JSONResponse{Message: "onboarding plan is not ready"}, nil
		}
		s.logger.Error("failed fetching onboarding goals for plan", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetOnboardingPlan401JSONResponse{Message: "unauthorized"}, nil
	}

	trainingProfile, err := buildTrainingProfile(goalsRow)
	if err != nil {
		s.logger.Error("failed building training profile", zap.Error(err), zap.String("user_id", userID.String()))
		return generated.GetOnboardingPlan404JSONResponse{Message: "onboarding plan is not ready"}, nil
	}

	if !isGoalsSelectionComplete(goalsRow, trainingProfile) {
		return generated.GetOnboardingPlan404JSONResponse{Message: "onboarding plan is not ready"}, nil
	}

	plan := buildFirstWeekPlanSummary(goalsRow)
	explanation := buildOnboardingPlanExplanation(trainingProfile, plan)

	return generated.GetOnboardingPlan200JSONResponse{
		TrainingProfile: trainingProfile,
		FirstWeekPlan:   plan,
		Explanation:     explanation,
	}, nil
}

func (s *Server) PostConsentsGrant(ctx context.Context, request generated.PostConsentsGrantRequestObject) (generated.PostConsentsGrantResponseObject, error) {
	if request.Body == nil {
		return generated.PostConsentsGrant400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostConsentsGrant401JSONResponse{Message: "unauthorized"}, nil
	}

	consentType := string(request.Body.ConsentType)
	if !consent.IsValidType(consentType) {
		return generated.PostConsentsGrant400JSONResponse{Message: "invalid consent type"}, nil
	}

	metadata := map[string]interface{}{}
	if request.Body.MetadataJson != nil {
		metadata = *request.Body.MetadataJson
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return generated.PostConsentsGrant400JSONResponse{Message: "invalid metadataJson"}, nil
	}

	row, err := s.queries.UpsertConsent(ctx, db.UpsertConsentParams{
		UserID:       userID,
		ConsentType:  consentType,
		MetadataJson: metadataJSON,
	})
	if err != nil {
		s.logger.Error("failed granting consent", zap.Error(err), zap.String("user_id", userID.String()), zap.String("consent_type", consentType))
		return generated.PostConsentsGrant400JSONResponse{Message: "could not grant consent"}, nil
	}

	return generated.PostConsentsGrant200JSONResponse{
		Consent: toAPIConsent(row),
	}, nil
}

func (s *Server) PostConsentsRevoke(ctx context.Context, request generated.PostConsentsRevokeRequestObject) (generated.PostConsentsRevokeResponseObject, error) {
	if request.Body == nil {
		return generated.PostConsentsRevoke400JSONResponse{Message: "request body is required"}, nil
	}

	userID, ok := middleware.AuthenticatedUserID(ctx)
	if !ok {
		return generated.PostConsentsRevoke401JSONResponse{Message: "unauthorized"}, nil
	}

	consentType := string(request.Body.ConsentType)
	if !consent.IsValidType(consentType) {
		return generated.PostConsentsRevoke400JSONResponse{Message: "invalid consent type"}, nil
	}

	row, err := s.queries.RevokeConsent(ctx, db.RevokeConsentParams{
		UserID:      userID,
		ConsentType: consentType,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generated.PostConsentsRevoke404JSONResponse{Message: "active consent not found"}, nil
		}
		s.logger.Error("failed revoking consent", zap.Error(err), zap.String("user_id", userID.String()), zap.String("consent_type", consentType))
		return generated.PostConsentsRevoke400JSONResponse{Message: "could not revoke consent"}, nil
	}

	return generated.PostConsentsRevoke200JSONResponse{
		Consent: toAPIConsent(row),
	}, nil
}

func (s *Server) issueSessionTokens(ctx context.Context, userID uuid.UUID) (generated.TokenResponse, error) {
	sessionID := uuid.New()
	refreshToken, refreshHash, expiresAt, err := s.tokenSvc.NewRefreshToken()
	if err != nil {
		return generated.TokenResponse{}, err
	}

	if _, err := s.queries.CreateSession(ctx, db.CreateSessionParams{
		ID:               sessionID,
		UserID:           userID,
		RefreshTokenHash: refreshHash,
		ExpiresAt:        expiresAt,
	}); err != nil {
		return generated.TokenResponse{}, err
	}

	accessToken, err := s.tokenSvc.IssueAccessToken(userID, sessionID)
	if err != nil {
		return generated.TokenResponse{}, err
	}

	return generated.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    s.tokenSvc.AccessTTLSeconds(),
	}, nil
}

func (s *Server) toAPIUser(ctx context.Context, row db.User) generated.User {
	entitlements := entitlement.NewSnapshot(nil)
	if s.entitlement != nil {
		snapshot, err := s.entitlement.SnapshotForUser(ctx, row.ID)
		if err != nil {
			s.logger.Error(
				"failed loading user entitlements",
				zap.Error(err),
				zap.String("user_id", row.ID.String()),
			)
		} else {
			entitlements = snapshot
		}
	}

	return generated.User{
		Id:           openapi_types.UUID(row.ID),
		Email:        openapi_types.Email(row.Email),
		IsPro:        entitlements.IsPro(),
		Entitlements: toAPIEntitlementKeys(entitlements.List()),
		CoachTier:    generated.CoachTier(entitlements.CoachTier()),
		CreatedAt:    row.CreatedAt,
	}
}

func toAPIEntitlementKeys(values []string) []generated.EntitlementKey {
	if len(values) == 0 {
		return []generated.EntitlementKey{}
	}

	result := make([]generated.EntitlementKey, 0, len(values))
	for _, value := range values {
		result = append(result, generated.EntitlementKey(value))
	}
	return result
}

func toAPIConsent(row db.Consent) generated.Consent {
	metadata := map[string]interface{}{}
	if len(row.MetadataJson) > 0 {
		_ = json.Unmarshal(row.MetadataJson, &metadata)
	}

	var revokedAt *time.Time
	if row.RevokedAt.Valid {
		timestamp := row.RevokedAt.Time
		revokedAt = &timestamp
	}

	return generated.Consent{
		Id:           openapi_types.UUID(row.ID),
		ConsentType:  generated.ConsentType(row.ConsentType),
		GrantedAt:    row.GrantedAt,
		RevokedAt:    revokedAt,
		MetadataJson: metadata,
	}
}

func toAPIExercise(catalogExercise exercise.CatalogExercise) generated.Exercise {
	apiMedia := make([]generated.ExerciseMedia, 0, len(catalogExercise.Media))
	for _, media := range catalogExercise.Media {
		apiMedia = append(apiMedia, generated.ExerciseMedia{
			Id:              openapi_types.UUID(media.ID),
			ExerciseId:      openapi_types.UUID(media.ExerciseID),
			MediaType:       generated.ExerciseMediaMediaType(media.MediaType),
			Uri:             media.URI,
			ThumbnailUri:    media.ThumbnailURI,
			DurationSeconds: media.DurationSeconds,
			CreatedAt:       media.CreatedAt,
		})
	}

	return generated.Exercise{
		Id:                 openapi_types.UUID(catalogExercise.ID),
		Slug:               catalogExercise.Slug,
		Name:               catalogExercise.Name,
		PrimaryMuscleGroup: catalogExercise.PrimaryMuscleGroup,
		PrimaryMuscles:     catalogExercise.PrimaryMuscles,
		SecondaryMuscles:   catalogExercise.SecondaryMuscles,
		MovementPattern:    catalogExercise.MovementPattern,
		Contraindications:  catalogExercise.Contraindications,
		Equipment:          catalogExercise.Equipment,
		Difficulty:         generated.ExerciseDifficulty(catalogExercise.Difficulty),
		Description:        catalogExercise.Description,
		CreatedAt:          catalogExercise.CreatedAt,
		Media:              apiMedia,
	}
}

func toAPIExerciseSubstitute(rankedSubstitute exercise.RankedSubstitute) generated.ExerciseSubstitute {
	return generated.ExerciseSubstitute{
		Exercise: toAPIExercise(rankedSubstitute.Exercise),
		Why: generated.ExerciseSubstituteWhy{
			MatchedPattern: rankedSubstitute.Why.MatchedPattern,
			MatchedMuscles: rankedSubstitute.Why.MatchedMuscles,
			EquipmentFit:   generated.ExerciseSubstituteWhyEquipmentFit(rankedSubstitute.Why.EquipmentFit),
		},
	}
}

func toAPIExerciseBiomechanics(payload biomechanics.ExerciseBiomechanics) generated.ExerciseBiomechanics {
	muscleHighlights := make([]generated.MuscleHighlight, 0, len(payload.MuscleHighlights))
	for _, highlight := range payload.MuscleHighlights {
		var colorHex *string
		if trimmed := strings.TrimSpace(highlight.ColorHex); trimmed != "" {
			colorHex = &trimmed
		}

		muscleHighlights = append(muscleHighlights, generated.MuscleHighlight{
			MuscleGroup:     highlight.MuscleGroup,
			ActivationLevel: float32(highlight.ActivationLevel),
			Role:            highlight.Role,
			ColorHex:        colorHex,
		})
	}

	jointAngles := make([]generated.JointAngle, 0, len(payload.JointAngles))
	for _, angle := range payload.JointAngles {
		jointAngles = append(jointAngles, generated.JointAngle{
			Joint:         angle.Joint,
			MinDegrees:    float32(angle.MinDegrees),
			MaxDegrees:    float32(angle.MaxDegrees),
			TargetDegrees: float32(angle.TargetDegrees),
			Unit:          angle.Unit,
		})
	}

	return generated.ExerciseBiomechanics{
		ExerciseId:        openapi_types.UUID(payload.ExerciseID),
		ExerciseSlug:      payload.ExerciseSlug,
		ExerciseName:      payload.ExerciseName,
		AnimationAssetKey: payload.AnimationAssetKey,
		AnimationAssetUri: payload.AnimationAssetURI,
		RigVersion:        payload.RigVersion,
		MuscleHighlights:  muscleHighlights,
		JointAngles:       jointAngles,
		Metadata:          payload.Metadata,
	}
}

func toAPIUserProfile(row db.UserProfile) generated.UserProfile {
	return generated.UserProfile{
		UserId:          openapi_types.UUID(row.UserID),
		DisplayName:     row.DisplayName,
		Sex:             row.Sex,
		HeightCm:        row.HeightCm,
		WeightKg:        float32(row.WeightKg),
		ExperienceLevel: row.ExperienceLevel,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func toAPIUserGoals(row db.UserGoal) (generated.UserGoals, error) {
	equipmentAccess := make([]string, 0)
	if len(row.EquipmentAccessJson) > 0 {
		if err := json.Unmarshal(row.EquipmentAccessJson, &equipmentAccess); err != nil {
			return generated.UserGoals{}, err
		}
	}

	constraints := map[string]interface{}{}
	if len(row.ConstraintsJson) > 0 {
		if err := json.Unmarshal(row.ConstraintsJson, &constraints); err != nil {
			return generated.UserGoals{}, err
		}
	}

	injuriesLimitations := make([]string, 0)
	if len(row.InjuriesLimitationsJson) > 0 {
		if err := json.Unmarshal(row.InjuriesLimitationsJson, &injuriesLimitations); err != nil {
			return generated.UserGoals{}, err
		}
	}

	modalityPreferences := make([]string, 0)
	if len(row.ModalityPreferencesJson) > 0 {
		if err := json.Unmarshal(row.ModalityPreferencesJson, &modalityPreferences); err != nil {
			return generated.UserGoals{}, err
		}
	}

	var priorTrainingHistory *map[string]interface{}
	if row.PriorTrainingHistoryJson.Valid {
		payload := map[string]interface{}{}
		if err := json.Unmarshal(row.PriorTrainingHistoryJson.RawMessage, &payload); err != nil {
			return generated.UserGoals{}, err
		}
		priorTrainingHistory = &payload
	}

	var readinessSignals *map[string]interface{}
	if row.ReadinessSignalsJson.Valid {
		payload := map[string]interface{}{}
		if err := json.Unmarshal(row.ReadinessSignalsJson.RawMessage, &payload); err != nil {
			return generated.UserGoals{}, err
		}
		readinessSignals = &payload
	}

	var secondaryGoal *string
	if row.SecondaryGoal.Valid {
		value := row.SecondaryGoal.String
		secondaryGoal = &value
	}

	return generated.UserGoals{
		UserId:                   openapi_types.UUID(row.UserID),
		PrimaryGoal:              row.PrimaryGoal,
		SecondaryGoal:            secondaryGoal,
		DaysPerWeek:              row.DaysPerWeek,
		SessionDurationMinutes:   row.SessionDurationMinutes,
		EquipmentAccessJson:      equipmentAccess,
		ConstraintsJson:          constraints,
		InjuriesLimitationsJson:  injuriesLimitations,
		ModalityPreferencesJson:  modalityPreferences,
		PriorTrainingHistoryJson: priorTrainingHistory,
		ReadinessSignalsJson:     readinessSignals,
		CreatedAt:                row.CreatedAt,
		UpdatedAt:                row.UpdatedAt,
	}, nil
}

func buildTrainingProfile(row db.UserGoal) (generated.TrainingProfile, error) {
	equipmentAccess := make([]string, 0)
	if len(row.EquipmentAccessJson) > 0 {
		if err := json.Unmarshal(row.EquipmentAccessJson, &equipmentAccess); err != nil {
			return generated.TrainingProfile{}, err
		}
	}

	constraints := map[string]interface{}{}
	if len(row.ConstraintsJson) > 0 {
		if err := json.Unmarshal(row.ConstraintsJson, &constraints); err != nil {
			return generated.TrainingProfile{}, err
		}
	}

	injuriesLimitations := make([]string, 0)
	if len(row.InjuriesLimitationsJson) > 0 {
		if err := json.Unmarshal(row.InjuriesLimitationsJson, &injuriesLimitations); err != nil {
			return generated.TrainingProfile{}, err
		}
	}

	modalityPreferences := make([]string, 0)
	if len(row.ModalityPreferencesJson) > 0 {
		if err := json.Unmarshal(row.ModalityPreferencesJson, &modalityPreferences); err != nil {
			return generated.TrainingProfile{}, err
		}
	}

	var priorTrainingHistory *map[string]interface{}
	if row.PriorTrainingHistoryJson.Valid {
		payload := map[string]interface{}{}
		if err := json.Unmarshal(row.PriorTrainingHistoryJson.RawMessage, &payload); err != nil {
			return generated.TrainingProfile{}, err
		}
		priorTrainingHistory = &payload
	}

	var readinessSignals *map[string]interface{}
	if row.ReadinessSignalsJson.Valid {
		payload := map[string]interface{}{}
		if err := json.Unmarshal(row.ReadinessSignalsJson.RawMessage, &payload); err != nil {
			return generated.TrainingProfile{}, err
		}
		readinessSignals = &payload
	}

	var secondaryGoal *string
	if row.SecondaryGoal.Valid {
		value := strings.TrimSpace(row.SecondaryGoal.String)
		if value != "" {
			secondaryGoal = &value
		}
	}

	return generated.TrainingProfile{
		PrimaryGoal:              row.PrimaryGoal,
		SecondaryGoal:            secondaryGoal,
		DaysPerWeek:              row.DaysPerWeek,
		SessionDurationMinutes:   row.SessionDurationMinutes,
		EquipmentAccess:          equipmentAccess,
		ScheduleDays:             extractScheduleDays(constraints, row.DaysPerWeek),
		InjuriesLimitationsFlags: injuriesLimitations,
		ModalityPreferences:      modalityPreferences,
		PriorTrainingHistory:     priorTrainingHistory,
		ReadinessSignals:         readinessSignals,
	}, nil
}

func toAPIProgram(row db.Program) (generated.Program, error) {
	goalTags := make([]string, 0)
	if len(row.GoalTagsJson) > 0 {
		if err := json.Unmarshal(row.GoalTagsJson, &goalTags); err != nil {
			return generated.Program{}, err
		}
	}

	return generated.Program{
		Id:              openapi_types.UUID(row.ID),
		Slug:            row.Slug,
		Name:            row.Name,
		Description:     row.Description,
		GoalTags:        goalTags,
		Level:           row.Level,
		WeeksLength:     row.WeeksLength,
		WeeklyFrequency: 0,
		Blocks:          []generated.ProgramBlock{},
		CreatedAt:       row.CreatedAt,
	}, nil
}

func toAPIProgramEnrollment(row db.UserProgramEnrollment) generated.ProgramEnrollment {
	return generated.ProgramEnrollment{
		Id:          openapi_types.UUID(row.ID),
		UserId:      openapi_types.UUID(row.UserID),
		ProgramId:   openapi_types.UUID(row.ProgramID),
		StartDate:   openapi_types.Date{Time: row.StartDate},
		CurrentWeek: row.CurrentWeek,
		CreatedAt:   row.CreatedAt,
	}
}

func isGoalsSelectionComplete(_ db.UserGoal, trainingProfile generated.TrainingProfile) bool {
	if strings.TrimSpace(trainingProfile.PrimaryGoal) == "" {
		return false
	}

	if len(trainingProfile.EquipmentAccess) == 0 {
		return false
	}

	if len(trainingProfile.ScheduleDays) == 0 {
		return false
	}

	if len(trainingProfile.InjuriesLimitationsFlags) == 0 {
		return false
	}

	if len(trainingProfile.ModalityPreferences) == 0 {
		return false
	}

	return true
}

func buildFirstWeekPlanSummary(row db.UserGoal) generated.FirstWeekPlanSummary {
	constraints := map[string]interface{}{}
	if len(row.ConstraintsJson) > 0 {
		_ = json.Unmarshal(row.ConstraintsJson, &constraints)
	}

	scheduleDays := extractScheduleDays(constraints, row.DaysPerWeek)
	sessionTheme := inferSessionTheme(row.PrimaryGoal)

	days := make([]generated.FirstWeekPlanDay, 0, len(scheduleDays))
	for index, day := range scheduleDays {
		days = append(days, generated.FirstWeekPlanDay{
			Day:         day,
			SessionName: fmt.Sprintf("%s %d", sessionTheme, index+1),
		})
	}

	return generated.FirstWeekPlanSummary{
		Days: days,
	}
}

func extractScheduleDays(constraints map[string]interface{}, fallbackDays int32) []string {
	dayOrder := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	dayRank := map[string]int{
		"monday":    0,
		"tuesday":   1,
		"wednesday": 2,
		"thursday":  3,
		"friday":    4,
		"saturday":  5,
		"sunday":    6,
	}

	normalizeDay := func(value string) (string, bool) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "", false
		}
		rank, ok := dayRank[strings.ToLower(trimmed)]
		if !ok {
			return "", false
		}
		return dayOrder[rank], true
	}

	scheduleDays := make([]string, 0, len(dayOrder))
	seen := make(map[string]struct{}, len(dayOrder))

	if raw, ok := constraints["scheduleDays"]; ok {
		if values, ok := raw.([]interface{}); ok {
			for _, item := range values {
				dayText, ok := item.(string)
				if !ok {
					continue
				}
				normalized, valid := normalizeDay(dayText)
				if !valid {
					continue
				}
				if _, exists := seen[normalized]; exists {
					continue
				}
				seen[normalized] = struct{}{}
				scheduleDays = append(scheduleDays, normalized)
			}
		}
	}

	if len(scheduleDays) > 0 {
		return scheduleDays
	}

	count := int(fallbackDays)
	if count <= 0 {
		return []string{}
	}
	if count > len(dayOrder) {
		count = len(dayOrder)
	}

	return append([]string{}, dayOrder[:count]...)
}

func inferSessionTheme(primaryGoal string) string {
	normalized := strings.ToLower(strings.TrimSpace(primaryGoal))
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.ReplaceAll(normalized, "-", " ")

	switch {
	case strings.Contains(normalized, "strength"):
		return "Strength Builder"
	case strings.Contains(normalized, "fat"):
		return "Conditioning Builder"
	case strings.Contains(normalized, "endurance"):
		return "Endurance Builder"
	default:
		return "Fitness Builder"
	}
}

func buildOnboardingPlanExplanation(
	trainingProfile generated.TrainingProfile,
	plan generated.FirstWeekPlanSummary,
) string {
	scheduleSummary := "your selected schedule days"
	if len(trainingProfile.ScheduleDays) > 0 {
		scheduleSummary = strings.Join(trainingProfile.ScheduleDays, ", ")
	}

	modalitySummary := "balanced training"
	if len(trainingProfile.ModalityPreferences) > 0 {
		modalitySummary = strings.Join(trainingProfile.ModalityPreferences, ", ")
	}

	injurySummary := "no major movement limitations noted"
	if len(trainingProfile.InjuriesLimitationsFlags) > 0 &&
		!containsNormalizedValue(trainingProfile.InjuriesLimitationsFlags, "none") {
		injurySummary = fmt.Sprintf(
			"we reduced movement risk around: %s",
			strings.Join(trainingProfile.InjuriesLimitationsFlags, ", "),
		)
	}

	readinessSummary := ""
	if trainingProfile.ReadinessSignals != nil && len(*trainingProfile.ReadinessSignals) > 0 {
		readinessSummary = " Readiness signals were included to tune the opening week intensity."
	}

	return fmt.Sprintf(
		"This first week has %d sessions on %s. Session themes prioritize %s, and %s.%s",
		len(plan.Days),
		scheduleSummary,
		modalitySummary,
		injurySummary,
		readinessSummary,
	)
}

func containsNormalizedValue(values []string, needle string) bool {
	target := strings.ToLower(strings.TrimSpace(needle))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == target {
			return true
		}
	}
	return false
}

func marshalOptionalObjectValue(values map[string]interface{}) (pqtype.NullRawMessage, error) {
	if values == nil {
		return pqtype.NullRawMessage{}, nil
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return pqtype.NullRawMessage{}, err
	}

	return pqtype.NullRawMessage{
		RawMessage: payload,
		Valid:      true,
	}, nil
}

func parseCSVTokens(value string) []string {
	parts := strings.Split(value, ",")
	tokens := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		token := strings.ToLower(strings.TrimSpace(part))
		if token == "" {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}

		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}

	return tokens
}

type substituteConstraintsQuery struct {
	Equipment   []string `json:"equipment"`
	InjuryFlags []string `json:"injuryFlags"`
}

func parseSubstituteConstraints(raw string) (substituteConstraintsQuery, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return substituteConstraintsQuery{}, nil
	}

	payload := substituteConstraintsQuery{}
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return substituteConstraintsQuery{}, err
	}

	payload.Equipment = dedupeTokens(payload.Equipment)
	payload.InjuryFlags = dedupeTokens(payload.InjuryFlags)
	return payload, nil
}

func dedupeTokens(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		token := strings.ToLower(strings.TrimSpace(value))
		if token == "" {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		normalized = append(normalized, token)
	}
	return normalized
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
