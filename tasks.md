# План задач: api-gateway

Дата анализа: **31 октября 2026**. Локальный HEAD: `ce5e2f7`; анализ также учитывает рабочее дерево на 2026-10-31.

## Текущее состояние

Есть GraphQL BFF с users/catalog/tasks/hunter/media/feed/search, httpOnly cookie и обезличенными ошибками. Handler включает introspection; server открывает playground. В handler нет complexity limit. Content и learning plan ещё не интегрированы. Подборки уже находятся в локальных изменениях.

Проверенные опорные файлы: `api/graphql/schema.graphqls`, `internal/graphql/handler.go`, `internal/server/server.go`, `internal/middleware/session.go`, `internal/graphql/problem_lists.resolvers.go`.

## Правила выполнения

- Оценка по договорённости: **1 SP ≈ 1 рабочий день; 2 SP ≈ 2 рабочих дня одного разработчика**, включая проверку и review. Это предварительная оценка, не гарантия календарного срока.
- Сначала реализовать весь описанный в карточке функциональный срез, затем написать/обновить тесты последним шагом. Без TDD; общий e2e — после интеграции. Уже существующие проверки не выключать.
- Статус всех карточек: **запланировано**, код по этому backlog ещё не изменён. Наличие исходников и тестов не означает подтверждённую работоспособность.
- Обязательные зависимости указаны явно; рабочая очередь и расчёт срока — в [общем плане](../tasks.md). F0–F4 — технические этапы, а PILOT/PROD/LATER — границы выпуска.
- При смене контракта: согласовать производителя и потребителей, сгенерировать код, обеспечить совместимый rollout. Миграции только append-only; секреты/hidden tests/исходники решений в обычные логи не писать.
- Задачу больше двух дней после уточнения делить на отдельно принимаемые части, сохраняя ID родителя как ссылку. Не скрывать дополнительный объём внутри пункта «и всё остальное».

Всего: **13 задач / 19 SP**; обязательный срез: **9 задач / 13 SP**.

<a id="gw-01"></a>

## [api-gateway] Enforce browser request security

- **ID:** GW-01
- **Оценка:** 1 SP (1 рабочий день)
- **Этап:** F1 — надёжная основа; **контур:** PILOT — обязательный срез.
- **Приоритет:** P0 — блокер корректности/безопасности.
- **Зависит от:** нет; можно брать после согласования границ релиза.
- **Где менять:** `internal/server/server.go`, `internal/middleware`.
- **Готово, когда:** Чужой origin не может выполнить авторизованную mutation.

**Шаги:**

1. Для cookie mutations проверить Origin/CSRF policy, включая multipart upload.
2. В production требовать Secure cookie, ограниченный CORS и HTTPS.
3. Отключить playground/debug introspection для публичного контура, сохранить локальный доступ.
4. **Проверка после готовности функционала:** cross-origin JSON/multipart и valid same-origin сценарии.

<a id="gw-02"></a>

## [api-gateway] Bound GraphQL query cost

- **ID:** GW-02
- **Оценка:** 1 SP (1 рабочий день)
- **Этап:** F1 — надёжная основа; **контур:** PILOT — обязательный срез.
- **Приоритет:** P0 — блокер корректности/безопасности.
- **Зависит от:** нет; можно брать после согласования границ релиза.
- **Где менять:** `internal/graphql/handler.go`, `internal/server/server.go`.
- **Готово, когда:** Дорогой запрос отклоняется до fan-out.

**Шаги:**

1. Использовать gqlgen complexity limit с весом pagination/list fields.
2. Ограничить JSON body, aliases/batch, request timeout и upload.
3. Вернуть нейтральные codes без деталей внутренних сервисов.
4. **Проверка после готовности функционала:** nested/aliased large query и boundary limits.

<a id="gw-03"></a>

## [api-gateway] Check revoked sessions and fresh roles

- **ID:** GW-03
- **Оценка:** 1 SP (1 рабочий день)
- **Этап:** F1 — надёжная основа; **контур:** PILOT — обязательный срез.
- **Приоритет:** P0 — блокер корректности/безопасности.
- **Зависит от:** [USR-01](../users/tasks.md#usr-01).
- **Где менять:** `internal/middleware/jwt.go`, `internal/client/users`.
- **Готово, когда:** Смена роли/блокировка действует на существующую cookie.

**Шаги:**

1. Проверять session/token version через users с коротким bounded cache.
2. Security-sensitive mutations проверять актуально, при недоступности fail closed.
3. Logout вызывать в users и очищать cookie.
4. **Проверка после готовности функционала:** revoked token, cache expiry и users outage.

<a id="gw-04"></a>

## [api-gateway] Expose graded code execution modes

- **ID:** GW-04
- **Оценка:** 1 SP (1 рабочий день)
- **Этап:** F2 — пользовательские функции; **контур:** PILOT — обязательный срез.
- **Приоритет:** P1 — обязательная функция.
- **Зависит от:** [TSK-04](../tasks/tasks.md#tsk-04), [TSK-07](../tasks/tasks.md#tsk-07).
- **Где менять:** `api/graphql/schema.graphqls`, `internal/client/tasks`, `internal/graphql`.
- **Готово, когда:** Frontend может выбрать samples/graded и получить безопасный verdict.

**Шаги:**

1. Добавить runMode, suite-safe result и attempt status в GraphQL.
2. Обновить clients/mappers и make generate с preserve_resolver.
3. Не проксировать private fields даже если они есть во внутреннем ответе.
4. **Проверка после готовности функционала:** resolver contract и hidden-output leakage.

<a id="gw-05"></a>

## [api-gateway] Expose material reading and drafting

- **ID:** GW-05
- **Оценка:** 2 SP (2 рабочих дня)
- **Этап:** F2 — пользовательские функции; **контур:** PILOT — обязательный срез.
- **Приоритет:** P1 — обязательная функция.
- **Зависит от:** [CNT-02](../content/tasks.md#cnt-02), [CNT-03](../content/tasks.md#cnt-03), [CNT-10](../content/tasks.md#cnt-10).
- **Где менять:** `internal/client/content`, `api/graphql/schema.graphqls`.
- **Готово, когда:** Материалы доступны через единственный внешний API.

**Шаги:**

1. Подключить опубликованный protobuf client content и deadlines/service auth.
2. Добавить list/detail/revisions/createDraft/saveDraft/publish с actor metadata.
3. Перегенерировать gqlgen и зафиксировать error codes/nullable partial results.
4. **Проверка после готовности функционала:** mocked gRPC contract, role checks и draft visibility.

<a id="gw-06"></a>

## [api-gateway] Expose change proposals and reviews

- **ID:** GW-06
- **Оценка:** 2 SP (2 рабочих дня)
- **Этап:** F2 — пользовательские функции; **контур:** PILOT — обязательный срез.
- **Приоритет:** P1 — обязательная функция.
- **Зависит от:** [CNT-07](../content/tasks.md#cnt-07), [CNT-11](../content/tasks.md#cnt-11), [GW-05](#gw-05).
- **Где менять:** `api/graphql/schema.graphqls`, `internal/graphql`.
- **Готово, когда:** Ревью workflow полностью доступен frontend без бизнес-логики в resolver.

**Шаги:**

1. Добавить proposal/diff/history/comments/approve/requestChanges/merge operations.
2. Передавать expected head/base versions и отображать MERGE_CONFLICT машинным кодом.
3. Добавить moderation/rollback после CNT-11; make generate.
4. **Проверка после готовности функционала:** outdated approval, self-review и concurrent merge mapping.

<a id="gw-07"></a>

## [api-gateway] Expose learner goals and diagnostics

- **ID:** GW-07
- **Оценка:** 2 SP (2 рабочих дня)
- **Этап:** F2 — пользовательские функции; **контур:** PILOT — обязательный срез.
- **Приоритет:** P1 — обязательная функция.
- **Зависит от:** [REC-04](../recommendations/tasks.md#rec-04), [REC-08](../recommendations/tasks.md#rec-08).
- **Где менять:** `internal/client/recommendations`, `api/graphql/schema.graphqls`.
- **Готово, когда:** Онбординг и диагностика доступны только владельцу.

**Шаги:**

1. Добавить learnerProfile/goal/diagnosticSession queries и mutations.
2. Actor брать из JWT, не принимать user_id другого студента.
3. Передавать request idempotency, version и deadlines; make generate.
4. **Проверка после готовности функционала:** cross-user access и repeated diagnostic submit.

<a id="gw-08"></a>

## [api-gateway] Expose learning plans and feedback

- **ID:** GW-08
- **Оценка:** 2 SP (2 рабочих дня)
- **Этап:** F2 — пользовательские функции; **контур:** PILOT — обязательный срез.
- **Приоритет:** P1 — обязательная функция.
- **Зависит от:** [REC-15](../recommendations/tasks.md#rec-15), [REC-18](../recommendations/tasks.md#rec-18), [GW-07](#gw-07).
- **Где менять:** `internal/client/recommendations`, `api/graphql/schema.graphqls`.
- **Готово, когда:** Клиент видит версию плана и может объяснимо её изменить.

**Шаги:**

1. Добавить currentPlan, progress, nextStep и feedback/skip/replan.
2. Вернуть reason codes, freshness, revision и безопасный fallback.
3. Принятие resource click не превращать в authoritative learning outcome.
4. **Проверка после готовности функционала:** stale revision, service outage и no-candidates.

<a id="gw-09"></a>

## [api-gateway] Expose ratings and contribution history

- **ID:** GW-09
- **Оценка:** 1 SP (1 рабочий день)
- **Этап:** F2 — пользовательские функции; **контур:** PILOT — обязательный срез.
- **Приоритет:** P1 — обязательная функция.
- **Зависит от:** [RAT-05](../rating/tasks.md#rat-05), [USR-08](../users/tasks.md#usr-08).
- **Где менять:** `internal/client/rating`, `api/graphql/schema.graphqls`.
- **Готово, когда:** Баллы и причины начисления доступны без email и private activity.

**Шаги:**

1. Добавить leaderboard/myRating/scoreHistory с pagination.
2. Получать display names batch API users и учитывать privacy.
3. Не принимать начисление баллов из frontend.
4. **Проверка после готовности функционала:** private profile, empty board и own-history ACL.

<a id="gw-10"></a>

## [api-gateway] Finish public problem list queries

- **ID:** GW-10
- **Оценка:** 1 SP (1 рабочий день)
- **Этап:** F2 — пользовательские функции; **контур:** PROD — дополнение к полному продакшену.
- **Приоритет:** P2 — последующее улучшение.
- **Зависит от:** [TSK-11](../tasks/tasks.md#tsk-11).
- **Где менять:** `internal/graphql/problem_lists.resolvers.go`, `internal/client/tasks`.
- **Готово, когда:** Подборка возвращает только опубликованные задачи в стабильном порядке.

**Шаги:**

1. Довести текущие локальные изменения схемы и resolver.
2. Отделить модерационные списки от публичных подборок.
3. Обновить generated code и frontend contract artifact.
4. **Проверка после готовности функционала:** role/list filtering и deleted task.

<a id="gw-11"></a>

## [api-gateway] Add dependency isolation and tracing

- **ID:** GW-11
- **Оценка:** 2 SP (2 рабочих дня)
- **Этап:** F3 — выпуск и эксплуатация; **контур:** PROD — дополнение к полному продакшену.
- **Приоритет:** P2 — последующее улучшение.
- **Зависит от:** [PKR-05](../parker/tasks.md#pkr-05).
- **Где менять:** `internal/client/httpx`, `internal/server/server.go`.
- **Готово, когда:** Отказ feed не блокирует решение задачи.

**Шаги:**

1. Разделить liveness и readiness; необязательный hunter/feed не валит весь gateway.
2. Ограничить fan-out concurrency, добавить deadlines и retries только idempotent reads.
3. Добавить OpenTelemetry client spans и request-scoped batching там, где подтверждён N+1.
4. **Проверка после готовности функционала:** downstream timeout, cancellation и fan-out count.

<a id="gw-12"></a>

## [api-gateway] Publish reproducible GraphQL schema artifacts

- **ID:** GW-12
- **Оценка:** 1 SP (1 рабочий день)
- **Этап:** F1 — надёжная основа; **контур:** PROD — дополнение к полному продакшену.
- **Приоритет:** P2 — последующее улучшение.
- **Зависит от:** нет; можно брать после согласования границ релиза.
- **Где менять:** `Makefile`, `.github/workflows/ci.yml`, `api/graphql`.
- **Готово, когда:** Frontend codegen воспроизводим на согласованной версии схемы.

**Шаги:**

1. Публиковать schema artifact с commit SHA, доступный frontend CI без соседнего checkout.
2. Проверять breaking changes и актуальность generated code.
3. Все новые APIs проходят эту же проверку.
4. **Проверка после готовности функционала:** clean generation и проверка несовместимого fixture diff.

<a id="gw-13"></a>

## [api-gateway] Expose account recovery and privacy APIs

- **ID:** GW-13
- **Оценка:** 2 SP (2 рабочих дня)
- **Этап:** F3 — выпуск и эксплуатация; **контур:** PROD — дополнение к полному продакшену.
- **Приоритет:** P2 — последующее улучшение.
- **Зависит от:** [USR-03](../users/tasks.md#usr-03), [USR-04](../users/tasks.md#usr-04), [USR-09](../users/tasks.md#usr-09).
- **Где менять:** `internal/client/users`, `api/graphql/schema.graphqls`.
- **Готово, когда:** Полный жизненный цикл аккаунта доступен через gateway.

**Шаги:**

1. Добавить refresh/recovery и export/delete operations.
2. Защитить их от enumeration/CSRF/rate abuse.
3. Генерировать schema и документы ответов для UI.
4. **Проверка после готовности функционала:** reset/revoke/export/delete и token redaction.
