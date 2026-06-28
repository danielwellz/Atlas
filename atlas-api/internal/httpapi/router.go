package httpapi

import (
	"net/http"

	"github.com/atlas/atlas-api/internal/auth"
	"github.com/atlas/atlas-api/internal/config"
	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"github.com/atlas/atlas-api/internal/httpapi/generated"
	"github.com/atlas/atlas-api/internal/httpapi/middleware"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

var publicPathAllowlist = map[string]struct{}{
	"/api/v1/health":        {},
	"/api/v1/auth/register": {},
	"/api/v1/auth/login":    {},
	"/api/v1/auth/logout":   {},
	"/api/v1/auth/refresh":  {},
	"/api/v1/events":        {},
	"/api/v1/exercises":     {},
	"/api/v1/exercises/*":   {},
	"/api/v1/programs":      {},
}

func NewRouter(logger *zap.Logger, cfg config.Config, queries db.Querier, tokenSvc *auth.TokenService) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.OTelHTTP(cfg.ServiceName))
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Authenticator(logger, tokenSvc, publicPathAllowlist))

	apiServer := NewServer(logger, cfg, queries, tokenSvc)

	strictMiddlewares := []generated.StrictMiddlewareFunc{
		apiServer.strictEntitlementMiddleware(),
	}
	generated.HandlerFromMux(generated.NewStrictHandler(apiServer, strictMiddlewares), r)

	return r
}
