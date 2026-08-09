# Интеграция с tasks-it

`api-gateway` обращается к внутреннему HTTP API `tasks-it` через отдельный клиент. Бизнес-правила, версии тестов, проверка ответов и история решений остаются в `tasks-it`.

## Конфигурация

```dotenv
TASKS_IT_URL=http://tasks-it:8080
TASKS_IT_TIMEOUT=5s
```

Gateway передаёт `X-Request-ID`. Для защищённых операций он формирует `X-User-ID` и `X-User-Roles` только из проверенного JWT. Значения из пользовательского HTTP-запроса напрямую не проксируются.

## Queries

- `itTasks(filter, pagination): ITTaskList!` — опубликованные тесты;
- `itTask(id): ITTask!` — актуальная опубликованная версия без правильных ответов;
- `adminITTasks(filter, pagination): ITTaskList!` — все тесты для администратора;
- `adminITTask(id): ITTask!` — текущая версия с `isCorrect`;
- `itSubmission(id): ITSubmission!` — результат владельца или администратора;
- `myITSubmissions(taskId, pagination): ITSubmissionList!` — история текущего пользователя.

Публичные query не требуют JWT. Admin query требуют роль `admin` или `superuser`. Query результатов требуют аутентификацию; доступ владельца дополнительно проверяет `tasks-it`.

## Mutations

- `createITTask(input): ITTask!` — создать draft и версию 1;
- `updateITTask(id, input): ITTask!` — полностью заменить содержимое новой версией;
- `changeITTaskStatus(id, status): ITTask!` — publish, archive или restore;
- `deleteITTask(id): Boolean!` — выполнить soft delete;
- `submitITTaskAnswer(taskId, input): ITSubmission!` — отправить ответ по выбранной версии.

Admin mutations требуют роль `admin` или `superuser`. Отправка ответа требует аутентификацию.

## Пример создания

```graphql
mutation CreateITTask($input: ITTaskInput!) {
  createITTask(input: $input) {
    id
    status
    taskVersionId
    versionNumber
    options {
      id
      text
      isCorrect
    }
  }
}
```

```json
{
  "input": {
    "topicId": null,
    "title": "Интерфейсы Go",
    "statement": "Выберите верное утверждение",
    "taskType": "single_choice",
    "difficulty": "easy",
    "options": [
      {"text": "Интерфейс реализуется неявно", "isCorrect": true},
      {"text": "Нужен implements", "isCorrect": false}
    ]
  }
}
```

## Пример отправки ответа

```graphql
mutation SubmitITTaskAnswer($taskId: ID!, $input: ITSubmissionInput!) {
  submitITTaskAnswer(taskId: $taskId, input: $input) {
    id
    correct
    verdict
    selectedOptionIds
    correctOptionIds
    taskVersionNumber
    taskUpdated
    latestTaskVersionId
    latestVersionNumber
  }
}
```

`idempotencyKey` — UUID, создаваемый frontend для одной попытки. Повтор с тем же содержимым возвращает сохранённый результат, а повтор с другим содержимым возвращает GraphQL error code `IDEMPOTENCY_KEY_CONFLICT`.

## Ошибки

Коды `{code, message}` от `tasks-it` сохраняются в `extensions.code` GraphQL-ошибки. Пользователю возвращается безопасное сообщение, технические детали остаются в структурированных логах gateway.
