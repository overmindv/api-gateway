# OVM-295 — браузерное продление сессии

## Решение и границы

Продолжение OVM-267. Users — единственный владелец rotation. Gateway хранит refresh
только в httpOnly/Secure/SameSite cookie ovm_refresh; наружу credential не возвращает.
Login/register имеют optional refreshSession=false; frontend включает только если
доступен Web Locks (secure context). Без Web Locks сохраняется прежний access-only вход.
Mutation refreshSession возвращает nullable expiry: свежий access — без rotation,
истёкший/отсутствующий access + refresh cookie — rotation Users, guest — null.
Origin обязателен также для запросов только с refresh cookie. Logout отзывает family
по refresh даже после истечения access, удаляет обе cookie только после подтверждения.

Frontend перед бизнес-запросом вызывает refreshSession под общей Web Lock. Login/logout/register используют ту же блокировку.
Срок не кэшируется на клиенте: часы браузера могут отличаться от сервера. Бизнес-операции не повторяются.
Между репликами gateway нет shared-state: сериализация выполняется браузером до fetch,
поэтому процессный singleflight или grace period для украденных tokens не нужны.
Несколько независимых браузеров не должны разделять cookie; потеря ответа требует входа.

## Коммиты и проверки

1. Gateway users client: opt-in и refresh/revoke, transport tests (3 файла).
2. Gateway middleware: cookie/context/Origin, regression tests (до 7 файлов).
3. GraphQL: schema/generated/resolver/mapping tests (до 7 файлов).
4. Frontend: fetch wrapper/Web Locks и tests, client wiring, schema generated при необходимости.
5. Документация/результаты и независимый review. Users → gateway → frontend порядок слияния.

Unit/race, lint/build, schema/codegen, mock browser concurrent tabs, cookies/Origin/logout.
Не изменять ветки Лёни, общий stack/БД. Deployment отдельно от merge.

## Проверено локально

Gateway make test (-race, coverage 64.8%), build, pinned lint 2.11.4 — успешно.
Frontend 173 tests, lint, typecheck/build — успешно; известное предупреждение bundle >500KB.
Chromium: две вкладки, два preflight, одна rotation, шесть отправок без повторов,
httpOnly cookie скрыта от JS; принудительно неверные часы не мешают.
Независимый review: две проблемы исправлены (clock cache и failed logout), повторный
review без actionable замечаний; reviewer повторил целевые backend/frontend tests.

Документирован один frontend origin на cookie jar; без Web Locks legacy access-only.
Full production stack/deployment не выполнялись. Публикация и merge разрешены владельцем.
Следующий шаг: CI и merge gateway, обновить pinned frontend schema, CI/merge frontend.
OpenSpec: docs/openspec/changes/browser-refresh-sessions (отдельный docs worktree).
