.PHONY: dev-up dev-down generate verify-generated config\:print config\:validate api-test api-run api-generate api-openapi-generate api-sqlc-generate api-migrate-up api-migrate-down api-seed api-seed-exercises mobile-install mobile-api-generate mobile-run-ios mobile-run-android

POSTGRES_URL ?= postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable
OAPI_CODEGEN_VERSION ?= v2.4.1
SQLC_VERSION ?= 1.27.0
GOOSE_VERSION ?= v3.24.1
OPENAPI_TYPESCRIPT_VERSION ?= 7.10.1

dev-up:
	docker compose -f infra/local/docker-compose.yml up -d

dev-down:
	docker compose -f infra/local/docker-compose.yml down

generate: api-generate mobile-api-generate

verify-generated:
	@if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then \
		echo "verify-generated requires a git checkout"; \
		exit 1; \
	fi
	@if ! git diff --quiet -- atlas-api/internal/httpapi/generated/openapi.gen.go atlas-api/internal/db/sqlc atlas-mobile/src/api/generated/openapi.ts; then \
		echo "Generated artifacts are stale. Run 'make generate' and commit updated files."; \
		git --no-pager diff -- atlas-api/internal/httpapi/generated/openapi.gen.go atlas-api/internal/db/sqlc atlas-mobile/src/api/generated/openapi.ts; \
		exit 1; \
	fi

config\:print:
	cd atlas-api && go run ./cmd/config print

config\:validate:
	cd atlas-api && go run ./cmd/config validate

api-test:
	cd atlas-api && go test ./...

api-run:
	cd atlas-api && go run ./cmd/atlas-api

api-generate: api-openapi-generate api-sqlc-generate

api-openapi-generate:
	cd atlas-api && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) \
		-generate types,chi-server,strict-server \
		-package generated \
		-o internal/httpapi/generated/openapi.gen.go \
		openapi/openapi.yaml

api-sqlc-generate:
	cd atlas-api && docker run --rm -v "$$PWD:/src" -w /src sqlc/sqlc:$(SQLC_VERSION) generate -f sqlc.yaml

api-migrate-up:
	cd atlas-api && go run -tags "no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir migrations postgres "$(POSTGRES_URL)" up

api-migrate-down:
	cd atlas-api && go run -tags "no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir migrations postgres "$(POSTGRES_URL)" down

api-seed:
	cd atlas-api && psql "$(POSTGRES_URL)" -f seeds/001_local_seed.sql

api-seed-exercises:
	cd atlas-api && go run ./cmd/seed exercises --file ./seed/exercises.csv

mobile-install:
	cd atlas-mobile && npm ci

mobile-api-generate:
	@version="$$(cd atlas-mobile && npx --no-install openapi-typescript --version)"; \
	if [ "$$version" != "v$(OPENAPI_TYPESCRIPT_VERSION)" ]; then \
		echo "Expected openapi-typescript v$(OPENAPI_TYPESCRIPT_VERSION), found $$version"; \
		exit 1; \
	fi
	cd atlas-mobile && npm run api:generate

mobile-run-ios:
	cd atlas-mobile && npx react-native run-ios

mobile-run-android:
	cd atlas-mobile && npx react-native run-android
