# Подборки задач (problem lists)

GraphQL-запросы `problemLists` и `problemList` обслуживаются `tasks` и доступны
любому аутентифицированному пользователю. Модерационная очередь кандидатов
(`taskCandidates`, approve/reject) остаётся административной.

## Роли и маршруты

Клиент `internal/client/tasks` выбирает маршрут по роли actor:

| Роль | Журнал | Детали |
|---|---|---|
| `admin` / `superuser` | `GET /v1/admin/problem-lists` | `GET /v1/admin/problem-lists/{id}` |
| остальные | `GET /v1/problem-lists` | `GET /v1/problem-lists/{id}` |

Видимость окончательно решает `tasks`: публичный маршрут возвращает только
опубликованные и не удалённые задачи в порядке сбора, а административный — ещё
и черновики.

## Поля

- `truncated` — источник собрал список частично.
- `memberCount` — число видимых задач списка.
- `pendingCandidates` — для обычного пользователя равен 0: `tasks` не раскрывает
  очередь модерации в публичной выдаче.
- `members` — опубликованные задачи с позицией сбора.

## Контракт

Схема экспортируется артефактом `make schema-artifact` (CI-джоба `GraphQL contract`).
Frontend закрепляет коммит и checksum артефакта в `schema/pin.json`; обновление
pin выполняется после merge изменения схемы.

## Проверки

- `internal/client/tasks/problem_lists_test.go` — выбор маршрута по роли.
- `internal/graphql/problem_lists_test.go` — нулевая очередь модерации у студента,
  передача actor и обязательная аутентификация.
- `make schema-test generate-check`, `make lint`, `go test ./...`.
