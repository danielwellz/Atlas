package middleware

import (
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			startedAt := time.Now()

			next.ServeHTTP(wrapped, r)

			statusCode := wrapped.Status()
			if statusCode == 0 {
				statusCode = http.StatusOK
			}

			requestID := chimiddleware.GetReqID(r.Context())
			spanContext := trace.SpanContextFromContext(r.Context())
			loggerFields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", statusCode),
				zap.Int("bytes", wrapped.BytesWritten()),
				zap.Duration("duration", time.Since(startedAt)),
				zap.String("request_id", requestID),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			}
			if spanContext.IsValid() {
				loggerFields = append(
					loggerFields,
					zap.String("trace_id", spanContext.TraceID().String()),
					zap.String("span_id", spanContext.SpanID().String()),
				)
			}

			if requestID != "" {
				trace.SpanFromContext(r.Context()).SetAttributes(attribute.String("http.request_id", requestID))
			}

			logger.Info("http request",
				loggerFields...,
			)
		})
	}
}
