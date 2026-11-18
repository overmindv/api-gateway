# Ревью материалов

Правку открывает `openMaterialChange(input)` от опубликованной версии
`baseRevisionId`. Ссылка на предложение строится по ID из ответа.
`materialChangeProposal(id)` возвращает автора, статус, основу и текущую
ревизию `head`.

- `appendMaterialChange` создает новую ревизию. Передай `expectedHeadId`.
  Без поля `attachments` вложения наследуются. Пустой список очищает их.
- `materialChangeHistory` возвращает историю без текста версий.
  Для следующей страницы передай `nextBeforeOrdinal` в `beforeOrdinal`.
- `materialChangeDiff` сравнивает основание с заданной ревизией или с
  текущей. Показывай diff как обычный текст. `truncated` означает, что
  сравнение вышло за лимит и результат может быть неполным.
- `closeMaterialChange` закрывает предложение без удаления истории.

Обсуждение открывается через `createMaterialReviewThread`. `lineNumber: 0`
относится ко всей ревизии. `replyToMaterialReviewThread` добавляет ответ,
`setMaterialReviewThreadResolved` закрывает или снова открывает обсуждение.
Для `materialReviewThreads` и `materialReviewReplies` используй `nextAfterId`
как `afterId`. ID обсуждений и курсоры передаются строками.

`materialReviewStatus` возвращает назначения, голоса и итог ревью. Итог
применяется только при `current: true`. `submitMaterialReview` принимает
`approve` или `request_changes` для конкретного `revisionId`. Для запроса
правок укажи причину. Назначения, кворум и права проверяет Content.

`mergeMaterialChange` принимает `expectedHeadId`. `REVISION_CONFLICT`
означает, что изменилась ревизия. `MERGE_CONFLICT` означает, что материал
теперь опубликован на другом основании. Во втором случае нужно перечитать
материал и вызвать `rebaseMaterialChange` с новым `baseRevisionId`. После
этого правка проходит ревью заново.

`hasUnresolvedDiscussions` в `materialReviewStatus` относится ко всему
предложению. `null` означает, что Content не передал признак. Ошибка
`UNRESOLVED_DISCUSSIONS` при слиянии означает, что обсуждение еще открыто.
Перед обновлением Gateway обнови Content и примени миграцию индекса.

## Откат и модерация

`restoreMaterialRevision` восстанавливает прежнее содержание новой
ревизией, не затирая историю. Нужны ID исходной и ожидаемой опубликованной
ревизий. Доступность операции проверяет Content.

`materialModeration` читает видимость публикации. `setMaterialModeration`
принимает действие `hide` или `restore`, причину, строку `expectedVersion`
и `expectedPublishedRevisionId`. Gateway проверяет роль в Users перед
вызовом. JWT-роли без проверки сессии и заголовки клиента не дают доступа.
Отдельный `CONTENT_GRPC_MODERATION_TOKEN` нужен Gateway и Content.
Без него модерация отключена, остальные операции работают.
