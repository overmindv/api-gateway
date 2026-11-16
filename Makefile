LOCAL_BIN := $(CURDIR)/bin
GQLGEN := $(LOCAL_BIN)/gqlgen
GOLANGCI_LINT := $(LOCAL_BIN)/golangci-lint

.PHONY: run build test integration lint generate graphql tidy

# Запуск
run:
	go run ./cmd/api-gateway

# Сборка
build:
	go build ./...

# Запуск тестов
test:
	go test -race ./...
	go test -coverprofile=coverage.out ./internal/client/users ./internal/client/tasks ./internal/graphql ./internal/middleware ./internal/config ./internal/server
	go tool cover -func=coverage.out | tail -1

# Запуск интеграционных тестов gateway
integration:
	go test ./tests/integration/...

# Проверка линтером
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

# Генерация GraphQL
generate: graphql

graphql: $(GQLGEN)
	$(GQLGEN) generate

# Обновление go mod
tidy:
	go mod tidy

$(GQLGEN):
	GOBIN="$(LOCAL_BIN)" go install github.com/99designs/gqlgen@v0.17.81

$(GOLANGCI_LINT):
	GOBIN="$(LOCAL_BIN)" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

.PHONY: generate-check schema-test schema-artifact schema-check
# Включает ручные resolver interfaces: drift нельзя скрыть за узким pathspec.
generate-check: generate
	git diff --exit-code -- internal/graphql go.mod go.sum
	@test -z "$$(git status --porcelain -- internal/graphql go.mod go.sum)" || (echo 'generated files drifted'; exit 1)

schema-test:
	npm test

schema-artifact:
	node scripts/schema-artifact.mjs export dist/graphql-schema

# BASE_SCHEMA — SDL предыдущего согласованного commit.
schema-check:
	@test -n "$(BASE_SCHEMA)" || (echo 'BASE_SCHEMA is required'; exit 1)
	node scripts/schema-diff.mjs "$(BASE_SCHEMA)" api/graphql/schema.graphqls
