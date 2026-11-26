# Восстановление доступа (account recovery)

GraphQL-операции `requestAccountRecovery`, `resetAccountPassword` и
`verifyAccountEmail` проксируют одноимённые мутации Users и не требуют активной
сессии: пользователь ещё не аутентифицирован.

## Контракт

```graphql
enum AccountRecoveryPurpose { reset verify }

mutation {
  requestAccountRecovery(email: "user@example.com", purpose: reset): Boolean!
  resetAccountPassword(token: "<из письма>", password: "<8–1024 байта>"): Boolean!
  verifyAccountEmail(token: "<из письма>"): Boolean!
}
```

- `purpose` по умолчанию `reset`; `verify` отправляет письмо подтверждения email.
- Токен приходит только в письме. Gateway не возвращает его в ответе и не пишет
  в логи: request-лог содержит метод, путь, статус и длительность, но не тело.
- `requestAccountRecovery` отвечает одинаково для существующего, отсутствующего
  и ограниченного по частоте email — перечисление адресов через gateway
  невозможно.
- `resetAccountPassword` использует одноразовый токен, меняет пароль и отзывает
  прежние access/refresh-сессии. После успеха gateway очищает `ovm_session` и
  `ovm_refresh`, потому что текущие браузерные credentials больше недействительны.
- `verifyAccountEmail` подтверждает владение адресом и не меняет политику входа.

## Ограничения и защита

Users применяет лимиты: 5 запросов на email и 100 на IP за 15 минут для выдачи
письма; 5 попыток на токен и 20 на IP за 15 минут для consume. Чтобы лимиты
считались по браузеру, gateway подписывает проверенный адрес соединения теми же
заголовками `X-Login-Client-*`, что и для `login`; пользовательские
`X-Forwarded-For` не используются.

Для cookie-аутентифицированных GraphQL-запросов действует существующая
Origin-проверка `BrowserSecurity`: запрос с cookie без разрешённого `Origin`
отклоняется до выполнения мутации. Bearer-запросы не несут ambient-credentials
и CSRF не подвержены.

## Ошибки

Ошибки Users сохраняются в `extensions.code` (`RATE_LIMITED`,
`UNAUTHENTICATED`, `INVALID_ARGUMENT`, `SERVICE_UNAVAILABLE`), сообщение
обезличено. Технические детали остаются в structured logs.

## Проверки

- `internal/client/users/recovery_test.go` — подпись client IP, отсутствие
  access JWT и проброс кода `RATE_LIMITED`.
- `internal/graphql/account_recovery_test.go` — работа без аутентификации,
  значение `purpose`, очистка cookie после reset и отсутствие секретов в ответе.
