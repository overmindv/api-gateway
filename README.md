# api-gateway

`api-gateway` — GraphQL BFF для `frontend`. Внешние клиенты обращаются только к gateway, а он делегирует операции сервисам-владельцам данных.

## Функционал

- единый GraphQL endpoint для frontend;
- регистрация, вход и профиль через `users`;
- admin-only управление пользователями через `users`;
- чтение и управление каталогом через `entities`;
- тестовые IT-задачи и история решений через `tasks-it`;
- запуск и журнал сбора через отдельный client `task-hunter`;
- модерация собранных кандидатов и публикация `programming`-задач;
- проверка JWT и роли `admin`;
- отдельный request log для пользовательских HTTP-запросов и upstream-вызовов.

## Бизнес-логика

`api-gateway` не владеет пользовательской, каталоговой или task-бизнес-логикой. Он проверяет внешний контракт, авторизацию и маршрутизирует запросы в сервис-владелец данных. Admin-only mutations разрешаются пользователям с ролью `admin` или `superuser` в JWT, который выпускает `users`.

Интеграция с `tasks-it`, список GraphQL-операций и примеры описаны в [docs/tasks-it.md](docs/tasks-it.md).

Ошибки для frontend обезличены, а технические детали пишутся в structured logs. Логи пользовательских запросов при запуске через `infra` доступны отдельно от Docker stdout.

## Запуск

Для связи с `tasks-it` задаются переменные:

```dotenv
TASKS_IT_URL=http://tasks-it:8080
TASKS_IT_TIMEOUT=5s
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
