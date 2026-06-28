package observability

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

const otelMetricsExportInterval = 30 * time.Second

// InitOTel configures global trace/metric providers and propagators.
func InitOTel(ctx context.Context, logger *zap.Logger, serviceName, env string) (func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.DeploymentEnvironment(env),
		),
	)
	if err != nil {
		return nil, err
	}

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	tracerProviderOptions := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))),
	}
	meterProviderOptions := []metric.Option{
		metric.WithResource(res),
	}

	if isOTLPConfigured() {
		traceExporter, traceErr := otlptracehttp.New(ctx)
		if traceErr != nil {
			logger.Warn("failed initializing OTLP trace exporter; traces will stay local", zap.Error(traceErr))
		} else {
			tracerProviderOptions = append(tracerProviderOptions, sdktrace.WithBatcher(traceExporter))
		}

		metricExporter, metricErr := otlpmetrichttp.New(ctx)
		if metricErr != nil {
			logger.Warn("failed initializing OTLP metrics exporter; metrics will stay local", zap.Error(metricErr))
		} else {
			meterProviderOptions = append(
				meterProviderOptions,
				metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(otelMetricsExportInterval))),
			)
		}
	} else {
		logger.Info("OTLP exporter not configured; telemetry remains in-process only")
	}

	tracerProvider := sdktrace.NewTracerProvider(tracerProviderOptions...)
	meterProvider := metric.NewMeterProvider(meterProviderOptions...)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	shutdown := func(shutdownCtx context.Context) error {
		var shutdownErr error
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
		if err := meterProvider.Shutdown(shutdownCtx); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
		return shutdownErr
	}

	return shutdown, nil
}

func isOTLPConfigured() bool {
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
	} {
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
