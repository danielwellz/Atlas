package httpapi

import (
	"net/http"
	"time"

	"github.com/atlas/atlas-api/internal/auth"
	"github.com/atlas/atlas-api/internal/config"
	db "github.com/atlas/atlas-api/internal/db/sqlc"
	"go.uber.org/zap"
)

// NewRouterWithClock is NewRouter with the server clock replaced, so tests
// with date-bucketed assertions can run deterministically on any day.
func NewRouterWithClock(logger *zap.Logger, cfg config.Config, queries db.Querier, tokenSvc *auth.TokenService, now func() time.Time) http.Handler {
	apiServer := NewServer(logger, cfg, queries, tokenSvc)
	apiServer.currentTime = now
	return newRouter(logger, cfg, tokenSvc, apiServer)
}
