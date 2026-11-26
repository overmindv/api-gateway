# Экспорт и удаление аккаунта (account privacy)

Gateway проксирует внутренний REST Users `/internal/privacy/*` и публикует
жизненный цикл запроса в GraphQL. Механизм на стороне Users включается
`USERS_PRIVACY_ENABLED=true`; выключенный сервис отвечает
`SERVICE_UNAVAILABLE`.

## Контракт

```graphql
mutation {
  requestAccountExport(password: "<текущий пароль>"): PrivacyRequestStart!
  requestAccountDeletion(password: "<текущий пароль>"): PrivacyRequestStart!
  confirmPrivacyRequest(id: ID!, receipt: String!): PrivacyRequest!
}

query {
  privacyRequest(id: ID!, receipt: String!): PrivacyRequest!
  accountExport(id: ID!): AccountExport!
}
```

- `requestAccountExport`/`requestAccountDeletion` требуют активной сессии и
  текущего пароля. Gateway передаёт Users Bearer пользователя, полученный из
  проверенного JWT или httpOnly cookie.
- Ответ `PrivacyRequestStart` содержит `request` и одноразовый `receipt`.
  Receipt подтверждает владение запросом: gateway не пишет его в логи и не
  помещает в URL; он передаётся только телом GraphQL и заголовком
  `X-Privacy-Receipt` во внутреннем вызове.
- `confirmPrivacyRequest` действует по receipt и не требует access JWT: запрос
  можно подтвердить после истечения сессии. Для `delete` Users атомарно
  обезличивает профиль и отзывает access/refresh-сессии, поэтому gateway
  очищает cookie `ovm_session` и `ovm_refresh`.
- `privacyRequest` показывает состояние и статусы владельцев по receipt.
- `accountExport` отдаёт готовый артефакт только владельцу с активным access
  JWT; одним receipt скачать экспорт нельзя.

## Значения

- `kind`: `export` или `delete`.
- `state`: `awaiting_confirmation`, `processing`, `completed`, `failed`,
  `expired`.
- `publication` (только для `delete`): статус outbox-публикации
  `user.deleted.v1` или `not_queued`.
- `owners[].state`: состояние обработки конкретным сервисом; `errorCode`
  содержит безопасный код причины.

Значения отдаются строками, чтобы новые состояния Users не ломали схему.

## Артефакт экспорта

`AccountExport` повторяет allowlist Users:

- `profile` — явно выбранные поля профиля; password/token hashes не входят.
- `services[].pages[].records[]` — `user_id`, `id`, `kind`, `occurred_at`,
  `resource_id`, `title`, `text`, `result`. Других полей нет.
- Пределы Users: 10 MiB на запрос, до 1000 страниц на владельца, артефакт
  живёт 24 часа. Превышение возвращает ошибку, а не обрезанный «успешный»
  экспорт. Gateway даёт на скачивание 30 секунд.

## Защита

- Перечисление email неприменимо: операции работают по активной сессии, а
  `requestAccountRecovery` выравнивает ответы.
- CSRF: cookie-запросы к `/graphql` проходят существующую Origin-проверку
  `BrowserSecurity`; запрос с cookie без разрешённого `Origin` отклоняется.
- Rate abuse: Users ограничивает 5 попыток пароля на subject, 50 на IP и
  5 запросов в сутки. Gateway подписывает проверенный IP браузера заголовками
  `X-Login-Client-*`, чтобы лимиты считались по клиенту, а не по pod'у gateway.
- Ответы внутреннего API помечены `no-store`; gateway не логирует тела,
  пароли, receipt и содержимое экспорта.

## Ошибки

`extensions.code` сохраняет безопасный код: `UNAUTHENTICATED`,
`PERMISSION_DENIED`, `RATE_LIMITED`, `PRIVACY_REQUEST_INVALID`,
`SERVICE_UNAVAILABLE`, `DEADLINE_EXCEEDED`. Сообщение обезличено, тело ответа
Users в ошибку не попадает.

## Проверки

- `internal/client/users/privacy_test.go` — маршруты, Bearer/receipt,
  подпись IP, отсутствие receipt в URL и безопасный маппинг статусов.
- `internal/graphql/account_privacy_test.go` — обязательная аутентификация
  prepare/export, маппинг состояний и владельцев, очистка cookie после
  подтверждения удаления, сортировка сервисов в артефакте.
