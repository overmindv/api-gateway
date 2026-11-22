# OVM-269 — проверка JWT по JWKS

JWT_JWKS_URL включает проверку Ed25519 через публичный JWKS Users.
JWT_ISSUER по умолчанию users, JWT_AUDIENCE — overmindv-gateway. Подпись,
audience, issuer, kid, exp/iat и TTL <=15 минут проверяются до обращения
к владельцу сессии. Durable session authority не заменяется локальной подписью.

JWKS URL требует HTTPS, кроме loopback HTTP для локальных тестов; credentials,
query и fragment в URL запрещены. Redirect и URL из JWT не используются.
Ответ <=32 KiB и <=8 уникальных OKP/Ed25519/sig ключей; cache 1 минута.
Unknown kid вызывает refresh не чаще 5 секунд. Истёкший cache при недоступном
JWKS не разрешает доступ. Удалённый Users ключ также проверяется session authority:
cache чтений 5 секунд, перед защищёнными mutations — свежий запрос.

USERS_LOGIN_CONTEXT_SECRET должен совпадать с Users и быть отдельным секретом
32–4096 байт. После перехода gateway не получает JWT_SECRET или signing seed.
Во время миграции старый JWT_SECRET принимается только с явным абсолютным
JWT_LEGACY_HS256_UNTIL (RFC3339, максимум 24h от запуска). После deadline
HS256 отклоняется независимо от exp. Удалить оба legacy env после перехода.

[Порядок ротации и аварийного отзыва в Users](https://github.com/overmindv/users/blob/main/docs/signing-keys.md).
Новый режим не включается автоматически; deployment/ротация реальных ключей
не выполнялись. Следующий пункт очереди — OVM-271.

Проверено 22.11.2026: make test -race (coverage 65.5%), integration, build,
lint 0, go vet — exit 0. Живой локальный контракт с Users/PostgreSQL 17
проверил me и обновлённый refresh access через JWKS без JWT_SECRET.
Review без P1/P2. HTTPS ingress и production/browser rollout не проверялись.
