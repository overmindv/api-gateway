LOCAL_BIN := $(CURDIR)/bin
GQLGEN := $(LOCAL_BIN)/gqlgen

.PHONY: run build test generate graphql proto up down logs tidy

# Запуск
run:
	go run ./cmd/laserbeak

# Сборка
build:
	go build ./...

# Запуск тестов
test:
	go test -race ./...
	go test -coverprofile=coverage.out ./internal/client/arcee ./internal/graphql ./internal/middleware ./internal/config ./internal/server
	go tool cover -func=coverage.out | tail -1

# Генерация GraphQL
generate: graphql

graphql: $(GQLGEN)
	$(GQLGEN) generate

# Запуск всей системы
up:
	docker compose up --build -d

# Остановка всей системы
down:
	docker compose down -v

# Логи
logs:
	docker compose logs -f laserbeak

# Обновление go mod
tidy:
	go mod tidy

$(GQLGEN):
	GOBIN="$(LOCAL_BIN)" go install github.com/99designs/gqlgen@v0.17.81
