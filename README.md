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

### IP-контекст входа

Для `login` gateway берёт IP непосредственного TCP peer и передаёт users
подписанные `X-Login-Client-IP`, `X-Login-Client-Time`, `X-Login-Client-Signature`.
Подпись HMAC-SHA256 использует общий `JWT_SECRET` и отдельный префикс протокола.
Публичные XFF/Forwarded и одноимённые заголовки клиента игнорируются. За reverse
proxy квота IP относится к самому proxy; доверие XFF нужно внедрять отдельно
с явной моделью доверенных адресов. Users принимает подпись не старше 60 секунд;
часы сервисов должны быть синхронизированы. Старые users игнорируют заголовки,
новые users со старым gateway продолжают применять email и peer-IP лимиты.

### Браузерная граница и production

`APP_ENV` принимает `development` (по умолчанию), `test`, `production`.
В development/test автоматически разрешены HTTP origins localhost, 127.0.0.1 и
[::1] на порту gateway для локального Playground. В production обязательны `SESSION_COOKIE_SECURE=true` и точные HTTPS origins
в `CORS_ORIGINS` (без wildcard, пути, query и credentials). `/playground` и schema
introspection отключены; `__typename` для обычных GraphQL-клиентов сохраняется.

Все `/graphql` запросы с Origin проверяются до разбора JSON/multipart. Cookie без
Origin запрещена, включая logout. Non-browser клиенты без cookie могут использовать
Bearer и не передавать Origin. Ответы отказа: `ORIGIN_NOT_ALLOWED` / `HTTPS_REQUIRED`.
CORS не заменяет проверку авторизации; same-site cookie также не заменяет Origin.

Production требует HTTPS: TLS непосредственно либо `X-Forwarded-Proto: https`
от peer из явно заданного `TRUSTED_PROXY_CIDRS` (CSV, например `10.0.0.0/24`).
Proxy должен перезаписывать этот заголовок и быть единственной внешней точкой входа;
не указывайте общую пользовательскую сеть. Без настроенного proxy HTTP отклоняется.
Health probes остаются доступны внутри сети. Эти настройки не разворачивают TLS/proxy.

### Бюджет GraphQL

Перед fan-out применяются gqlgen complexity <= 25000 с весом запрошенной
pagination и списков, глубина <= 16, не более 1000 раскрытых AST-элементов,
100 aliases и 20 корневых полей. Fragment spreads учитываются при каждом
раскрытии. Parser ограничен 10000 tokens. Неиспользуемые ветви с directives
также расходуют бюджет. Обычная страница истории из 100 решений поддерживается.

JSON body ограничен 1 MiB, multipart — 512 KiB, включая chunked; JSON batch
запрещён. Context deadline запроса — 30 секунд, HTTP read timeout ограничивает
чтение тела; внутренние HTTP-клиенты получают отмену. Публичный endpoint только
POST, подписок/WebSocket нет. Ошибки не содержат текста исходника или запроса:
`COMPLEXITY_LIMIT_EXCEEDED`, `OPERATION_LIMIT_EXCEEDED`, `REQUEST_TOO_LARGE`,
`BATCH_NOT_ALLOWED`, `DEADLINE_EXCEEDED`.

## Проверка сессий (OVM-101)

Gateway проверяет сессию через внутренние POST `/internal/session/check` и
`/internal/session/revoke` Users (OVM-265). Успешный check кешируется максимум
5 секунд/1024 записи; каждое корневое поле mutation и admin guard проверяются
без кеша. Недоступность Users запрещает защищённую операцию. Актуальные роли
Users заменяют claims и ADMIN_USER_IDS. Logout отзывает сессию и очищает cookie.
