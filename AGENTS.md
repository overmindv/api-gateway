# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`api-gateway` — GraphQL BFF (backend-for-frontend) для `frontend`. Единственная точка входа: внешние клиенты обращаются только к gateway, а он делегирует данные сервисам-владельцам. Не владеет бизнес-логикой — проверяет контракт, авторизацию и маршрутизирует. Локально запускается в составе общего окружения из `overmindv/infra`.

## Commands

```bash
make run          # запуск сервиса (go run ./cmd/api-gateway)
make build        # сборка (go build ./...)
make test         # unit-тесты (go test -race ./... + coverage)
make integration  # e2e gateway: go test ./tests/integration/...
make lint         # golangci-lint v2
make generate     # генерация GraphQL из schema (gqlgen)
make tidy         # go mod tidy
```

Единичный тест: `go test -v -run TestName ./internal/graphql/...`
Вся конфигурация из environment (`internal/config/config.go`) — см. `README.md` и `docs/tasks.md` для списка переменных. Для локального стека: `cd ../infra && make up`, endpoint на `http://localhost:8081/graphql`.

CI (`.github/workflows/ci.yml`) выполняет lint, test, build и e2e-прогон из `overmindv/tests`.

## Architecture

Запрос идёт: `frontend → gateway (/graphql) → сервис-владелец данных`. GraphQL-schema в `api/graphql/schema.graphqls`, резолверы находятся в `internal/graphql/`.

### Зависимые сервисы (все в `internal/client/*/`)
| Сервис | Протокол | Клиент |
|---|---|---|
| `users` | GraphQL (HTTP) | `internal/client/users` — регистрация, вход, профиль |
| `entities` | REST | `internal/client/entities` — каталог (университеты/программы/курсы/темы) |
| `tasks` | REST | `internal/client/tasks` — IT-задачи, submission'ы, код, кандидаты модерации |
| `task-hunter` | REST + service token | `internal/client/taskhunter` — очередь сбора задач из веба |

Каждый клиент экспонирует интерфейс (напр. `tasks.Service`), который внедряется в `internal/graphql/resolver.go` как поле `Resolver`. `tasks.CandidateService` опционален — проверяется через type-assertion в `handler.go` и может быть `nil`.

### GraphQL-генерация (gqlgen)
`schema.graphqls` → `internal/graphql/generated/generated.go` + `model/models_gen.go`. Конфиг `gqlgen.yml` использует `preserve_resolver: true` — **генератор обновляет интерфейсы, но не трогает написанные вручную резолверы**. Ручные резолверы распределены между `schema.resolvers.go` (users, каталог, tasks) и `collection.resolvers.go` (task-hunter: sources/jobs/кандидаты). После правки schema обязателен `make generate`.

### Авторизация (`internal/middleware`)
- `jwt.go` — парсит `Authorization: Bearer` или session cookie `ovm_session`; `RequireAuth(ctx)` и `RequireAdmin(ctx)` в middleware-слое. Admin = роль `admin` **или** `superuser` в JWT, **или** `UserID` в `ADMIN_USER_IDS.
- `context.go` — `Auth(ctx)` возвращает `AuthInfo{UserID, Token, Roles}`; резолверы передают его нисходящим сервисам.
- `session.go` — выдаёт httpOnly cookie после login/register через `SetSessionToken(ctx, token, maxAge)`; убирает через `ClearSessionToken`. httpOnly защищает JWT от XSS из localStorage.
- `cors.go`, `http.go` (request_id + логирование), цепочка строится в `internal/server/server.go:New` (порядок сверху-вниз).

### Проксирование actor context
Gateway передаёт вниз только проверенные данные: `X-Request-ID` (сквозной), а для защищённых операций — `X-User-ID` и `X-User-Roles`, сформированные из проверенного JWT. **Значения из пользовательского HTTP-запроса напрямую не проксируются.** `task-hunter` дополнительно получает отдельный `TASK_HUNTER_TOKEN`.

### Ошибки
`internal/apperror/errors.go` — sentinel'ы `ErrUnauthenticated`, `ErrPermissionDenied`. `internal/graphql/errors.go` (`ErrorPresenter`) маппит коды: ошибки нисходящих сервисов (`*users.Error`, `*entities.Error`, `*tasks.Error`, `*taskhunter.Error`) сохраняются в `extensions.code` (напр. `IDEMPOTENCY_KEY_CONFLICT`), а **пользователю возвращается обезличенное сообщение** «Не удалось выполнить действие.». Технические детали — в structured logs, не в ответ.

## Key Patterns

- **Mappers** в `internal/graphql/mapper.go` — конвертация DTO клиентов (users/entities/tasks) в GraphQL model. Вспомогательные `tasksActor(ctx, admin bool)` и `*Filter`-функции в `tasks.go`/`catalog.go` собирают actor + фильтры для HTTP-вызовов.
- **Health** `/health` и `/healthz` опрашивают users, tasks, task-hunter.
- Тесты: unit в `internal/**/*_test.go` (без Docker), e2e в `tests/integration/` — шлют HTTP в `httptest` с фейковыми клиентами/JWT.
