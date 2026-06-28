# OTel Collector (Staging)

- Collector receives OTLP gRPC (`4317`) and OTLP HTTP (`4318`).
- API is configured to export to `http://otel-collector:4318`.
- Collector exposes Prometheus metrics on `:8889`.

For CI trace-ingestion validation, expose the collector metrics endpoint to the GitHub runner and set:

- `STAGING_OTEL_COLLECTOR_METRICS_URL`

Expected metric signal used in CI:

- `otelcol_receiver_accepted_spans{...} > 0`
