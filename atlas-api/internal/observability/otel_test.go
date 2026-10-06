package observability

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestInitOTelWithoutOTLPExporter(t *testing.T) {
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
	} {
		t.Setenv(key, "")
	}

	shutdown, err := InitOTel(context.Background(), zap.NewNop(), "atlas-api-test", "test")
	if err != nil {
		t.Fatalf("InitOTel returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("InitOTel returned nil shutdown func")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}
