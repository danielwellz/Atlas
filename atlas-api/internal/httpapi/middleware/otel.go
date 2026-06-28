package middleware

import (
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func OTelHTTP(serviceName string) func(http.Handler) http.Handler {
	base := otelhttp.NewMiddleware(serviceName)

	return func(next http.Handler) http.Handler {
		return base(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if requestID := chimiddleware.GetReqID(r.Context()); requestID != "" {
				trace.SpanFromContext(r.Context()).SetAttributes(attribute.String("http.request_id", requestID))
			}
			next.ServeHTTP(w, r)
		}))
	}
}
