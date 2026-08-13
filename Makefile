LOCAL_BIN := $(CURDIR)/bin
GQLGEN := $(LOCAL_BIN)/gqlgen
GOLANGCI_LINT := $(LOCAL_BIN)/golangci-lint

.PHONY: run build test integration lint generate graphql tidy

# Запуск
run:
	go run ./cmd/laserbeak

# Сборка
build:
	go build ./...

# Запуск тестов
test:
	go test -race ./...
	go test -coverprofile=coverage.out ./internal/client/arcee ./internal/client/tasksit ./internal/graphql ./internal/middleware ./internal/config ./internal/server
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
	GOBIN="$(LOCAL_BIN)" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6
