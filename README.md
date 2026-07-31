# Laserbeak

Laserbeak - GraphQL API Gateway / BFF для frontend `soundwave`. Frontend обращается только к Laserbeak, а Laserbeak делегирует операции внутренним сервисам.

## Функционал

- единый GraphQL endpoint для frontend;
- регистрация, вход и профиль через Arcee;
- admin-only управление пользователями через Arcee;
- чтение и управление каталогом через Ironhide;
- проверка JWT и роли `admin`;
- отдельный request log для пользовательских HTTP-запросов и upstream-вызовов.

## Бизнес-логика

Laserbeak не владеет пользовательской или каталоговой бизнес-логикой. Он проверяет внешний контракт, авторизацию и маршрутизирует запросы в сервис-владелец данных. Admin-only mutations разрешаются пользователям с ролью `admin` в JWT, который выпускает Arcee.

Ошибки для frontend обезличены, а технические детали пишутся в structured logs. Логи пользовательских запросов при запуске через `ratchet` доступны отдельно от Docker stdout.

## Запуск

Локально gateway запускается в составе общего окружения из `ratchet`:

```bash
cd ../ratchet
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
