# Интеграция с tasks

`api-gateway` обращается к внутреннему HTTP API `tasks` через отдельный клиент. Бизнес-правила, версии тестов, проверка ответов и история решений остаются в `tasks`.

## Конфигурация

```dotenv
TASKS_URL=http://tasks:8080
TASKS_TIMEOUT=5s
TASK_HUNTER_URL=http://task-hunter:8080
TASK_HUNTER_TOKEN=replace-with-a-long-random-gateway-token
TASK_HUNTER_TIMEOUT=10s
```

Gateway передаёт `X-Request-ID`. Для защищённых операций он формирует `X-User-ID` и `X-User-Roles` только из проверенного JWT. Значения из пользовательского HTTP-запроса напрямую не проксируются.

## Queries

- `itTasks(filter, pagination): ITTaskList!` — опубликованные тесты;
- `itTask(id): ITTask!` — актуальная опубликованная версия без правильных ответов;
- `adminITTasks(filter, pagination): ITTaskList!` — все тесты для администратора;
- `adminITTask(id): ITTask!` — текущая версия с `isCorrect`;
- `itSubmission(id): ITSubmission!` — результат владельца или администратора;
- `myITSubmissions(taskId, pagination): ITSubmissionList!` — история ответов текущего пользователя;
- `itCodeSubmission(id): ITCodeSubmission!` — актуальный статус и результат проверки файла;
- `myITCodeSubmissions(taskId, pagination): ITCodeSubmissionList!` — история программных решений;
- `taskCollectionSources`, `taskCollectionJobs`, `taskCollectionJob` — allowlist и журнал сбора;
- `taskCandidates`, `taskCandidate` — очередь модерации с provenance и revision.

Публичные query не требуют JWT. Admin query требуют роль `admin` или `superuser`. Query результатов требуют аутентификацию; доступ владельца дополнительно проверяет `tasks`.

## Mutations

- `createITTask(input): ITTask!` — создать draft и версию 1;
- `updateITTask(id, input): ITTask!` — полностью заменить содержимое новой версией;
- `changeITTaskStatus(id, status): ITTask!` — publish, archive или restore;
- `deleteITTask(id): Boolean!` — выполнить soft delete;
- `submitITTaskAnswer(taskId, input): ITSubmission!` — отправить ответ по выбранной версии;
- `submitITTaskCode(taskId, input): ITCodeSubmission!` — загрузить Python- или Go-файл программного решения;
- `startTaskCollection`, `acknowledgeTaskCollectionJob` — запустить ручной job до 20 website URL одним списком и подтвердить terminal-уведомление;
- `updateTaskCandidate`, `approveTaskCandidate`, `rejectTaskCandidate` — редактировать и завершать кандидата с optimistic locking.

Admin mutations требуют роль `admin` или `superuser`. Отправка любого решения требует аутентификацию.

Gateway передаёт `task-hunter` только actor context из проверенного JWT и отдельный service token. Для опубликованной `programming`-задачи GraphQL возвращает `tags`, `examples`, `constraints` и `source`; отправка choice-ответа для неё запрещена кодом `TASK_TYPE_NOT_SUBMITTABLE`.

`websiteUrls` принимает CodeRun, LeetCode и Codeforces. Канонизацию похожих URL и проверку дублей выполняет `task-hunter`; результат каждого URL доступен в `taskCollectionJob.sources`, поэтому ошибка одного сайта не скрывает успешные задачи остальных.

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

## Пример отправки файла

```graphql
mutation SubmitITTaskCode($taskId: ID!, $input: ITCodeSubmissionInput!) {
  submitITTaskCode(taskId: $taskId, input: $input) {
    id
    status
    verdict
    sourceFileName
    compilation {
      exitCode
      stdout
      stderr
      durationMs
      memoryBytes
    }
    execution {
      exitCode
      stdout
      stderr
      durationMs
      memoryBytes
    }
    tests {
      testId
      verdict
      durationMs
      memoryBytes
    }
    failure {
      code
      message
    }
  }
}
```

Переменная `input.file` передаётся по GraphQL multipart request specification как `Upload`. Поддерживаются `language: python` с расширением `.py` и `language: go` с расширением `.go`; размер исходника ограничен 256 КБ. Первичный ответ обычно имеет статус `queued`. До статуса `completed` клиент повторяет `itCodeSubmission(id)` и затем показывает `verdict`, результаты фаз и открытых тестов. Gateway принимает multipart-запросы размером не более 512 КБ с учётом служебных частей формы.

## Ошибки

Коды `{code, message}` от `tasks` сохраняются в `extensions.code` GraphQL-ошибки. Пользователю возвращается безопасное сообщение, технические детали остаются в структурированных логах gateway.
