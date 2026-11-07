# api-gateway

`api-gateway` — GraphQL BFF для `frontend`. Внешние клиенты обращаются только к gateway, а он делегирует операции сервисам-владельцам данных.

## Функционал

- единый GraphQL endpoint для frontend;
- регистрация, вход и профиль через `users`;
- admin-only управление пользователями через `users`;
- чтение и управление каталогом через `entities`;
- тестовые IT-задачи и история решений через `tasks`;
- предварительная подборка практики через необязательный сервис `recommendations`;
- запуск и журнал сбора через отдельный client `task-hunter`;
- модерация собранных кандидатов и публикация `programming`-задач;
- проверка JWT и роли `admin`;
- отдельный request log для пользовательских HTTP-запросов и upstream-вызовов.

## Бизнес-логика

`api-gateway` не владеет пользовательской, каталоговой или task-бизнес-логикой. Он проверяет внешний контракт, авторизацию и маршрутизирует запросы в сервис-владелец данных. Admin-only mutations разрешаются пользователям с ролью `admin` или `superuser` в JWT, который выпускает `users`.

Интеграция с `tasks`, список GraphQL-операций и примеры описаны в [docs/tasks.md](docs/tasks.md).

Ошибки для frontend обезличены, а технические детали пишутся в structured logs. Логи пользовательских запросов при запуске через `infra` доступны отдельно от Docker stdout.

## Запуск

Для связи с `tasks` задаются переменные:

```dotenv
TASKS_URL=http://tasks:8080
TASKS_TIMEOUT=5s
TASK_HUNTER_URL=http://task-hunter:8080
TASK_HUNTER_TOKEN=replace-with-a-long-random-gateway-token
TASK_HUNTER_TIMEOUT=10s
```

Локально gateway запускается в составе общего окружения из `infra`:

```bash
cd ../infra
cp .env.example .env
make up
make request-logs
```

Для разработки самого сервиса:

```bash
make generate
make test
make integration
make build
```

GraphQL endpoint при локальном запуске стека: `http://localhost:8081/graphql`.

## Подборка практики

`practicePlan(topicIds: [ID!]!, minutes: Int!): PracticePlan!` доступен только
авторизованному пользователю. Одна GraphQL-операция может вычислять только одну подборку;
несколько aliases, включая aliases во fragments, отклоняются до обращения к сервисам
с кодом `OPERATION_LIMIT_EXCEEDED`. Повторные selections одного поля объединяются. Вход: 1–5 разных UUID активных тем и бюджет 10–240 минут.
Gateway собирает опубликованные задачи, prerequisites и собственную историю решений
пользователя; алгоритм и оценки времени принадлежат сервису `recommendations`.

```dotenv
RECOMMENDATIONS_URL=http://recommendations:8080
RECOMMENDATIONS_SERVICE_TOKEN=replace-with-a-long-random-service-token
RECOMMENDATIONS_TIMEOUT=5s
```

Без `RECOMMENDATIONS_URL` интеграция выключена: запрос возвращает обезличенную ошибку
с кодом `SERVICE_UNAVAILABLE`. При включении сервисный токен обязателен. Gateway
передаёт его в Bearer-заголовке, а `X-User-ID` формирует из проверенного JWT; пользовательский
JWT, исходный код, варианты ответов и содержимое тестов в рекомендации не передаются.

Граф prerequisites должен быть полным, активным, ацикличным и содержать не более 32 тем;
небезопасный или слишком большой граф отклоняется. Кандидаты обходятся по кругу между
темами и ограничены 2000 записями. Каждая из двух историй ограничена 1000 записями до
дедупликации по задаче/версии. Дополнительная запись проверяет факт усечения;
предупреждение `PARTIAL_DATA` передаётся в результат. Сбой зависимости возвращает ошибку,
а не пустую историю. Общий дедлайн подготовки preview — 10 секунд. Новая попытка пользователя
учитывается следующим запросом; preview не сохраняется и не обещает освоение темы.
