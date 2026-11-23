# Цели обучения и адаптивный план в GraphQL

Gateway публикует типизированный owner-only контракт сервиса `recommendations` для
профиля доступного времени, целей, диагностики и адаптивного учебного плана.

## Авторизация и конкурентные записи

Все операции требуют JWT. Идентификатор ученика берётся только из проверенного
контекста авторизации и передаётся в `recommendations` как `X-User-ID`; аргумента
`userId` в публичной схеме нет.

Записи профиля и целей принимают `requestId` и `expectedVersion`. Gateway передаёт
их без преобразований как `Idempotency-Key` и `X-Expected-Version`. Создание цели
требует только `requestId`. Генерация плана и feedback передают `requestId` как
`Idempotency-Key`, а версии — в JSON-теле согласно контракту владельца.

Повтор с тем же ключом и тем же содержимым безопасен. Конфликт содержимого или
версии возвращается через `extensions.code`, в частности
`IDEMPOTENCY_CONFLICT` или `VERSION_CONFLICT`; gateway не увеличивает версии
самостоятельно.

## Операции

Queries:

- `learnerGoalTemplates`, `learnerProfile`, `learnerGoals`, `learnerGoal`;
- `learnerDiagnostic`;
- `currentLearningPlan`, `learningPlanExplanation`.

Mutations:

- `saveLearnerProfile`;
- `createLearnerGoal`, `updateLearnerGoal`, `deleteLearnerGoal`;
- `startLearnerDiagnostic`, `skipLearnerDiagnostic`,
  `finalizeLearnerDiagnostic`;
- `generateLearningPlan`, `submitLearningPlanFeedback`;
- `startLearningPlanStep`, `completeLearningPlanStepActivity`,
  `recordLearningPlanStepAssessment`.

Открытие ресурса через `startLearningPlanStep` фиксирует только начало активности.
Оно не считается завершением и не создаёт оценку. Независимая проверка учитывается
только через `recordLearningPlanStepAssessment`, когда у `recommendations` уже есть
подходящий результат проверки.

## Состояния и fallback

Gateway не вычисляет `stale`, reason codes или объяснения самостоятельно. Он
типизированно переносит ответ `recommendations`, включая `freshness`,
`blockedReason`, `nextStep`, ranking/evidence policy и evidence watermark.

При недоступном каталоге допустим сохранённый план со следующей комбинацией:

- `stale = false`;
- `freshness.status = "unavailable"`;
- `blockedReason = "catalog_unavailable"`;
- `nextStep = null`.

Это не означает, что план актуален для следующего действия: frontend должен учитывать
`freshness` и `blockedReason`. При полной недоступности `recommendations` gateway
возвращает `SERVICE_UNAVAILABLE` и не использует собственный незаверсионированный
кэш.

Статусы диагностики `no_tasks` и `skipped` являются успешными состояниями.
Финальный отчёт сохраняет `submissionId`, `eventId` и `correctionVersion` для
трассируемости, а объяснение плана — evidence reference и watermark.
