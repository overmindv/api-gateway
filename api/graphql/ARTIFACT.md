# Версионированный GraphQL artifact

Каждый зелёный CI gateway публикует `graphql-schema-<full commit SHA>` на 90 дней.
Внутри `schema.graphqls` и `manifest.json`: repository, commit, schema_sha256,
format_version. Нет времени сборки, поэтому повторный export одного commit побайтово
одинаков. SDL экспортируется только из чистого относительно HEAD файла схемы.

## Проверки и получение

`npm ci --ignore-scripts && make schema-test generate-check` проверяет несовместимый
fixture и актуальность gqlgen, включая resolver interfaces. `make schema-check
BASE_SCHEMA=/path/to/base.graphqls` отвергает breaking changes через закреплённый
GraphQL.js. CI сравнивает PR с base SHA, push с предыдущим SHA; новые API проходят
тот же gate. Удаление API выполняется через совместимую миграцию, без обхода gate.

После успешного workflow скачайте artifact по точным repository/run ID/SHA через
GitHub Actions API или `actions/download-artifact@v4`. Для приватного cross-repository
доступа нужен token с Actions read у gateway; обычный frontend GITHUB_TOKEN может
не иметь такого доступа. Пример шага frontend workflow:

```yaml
- uses: actions/download-artifact@v4
  with:
    repository: overmindv/api-gateway
    run-id: ${{ vars.GATEWAY_SCHEMA_RUN_ID }}
    name: graphql-schema-${{ vars.GATEWAY_SCHEMA_COMMIT }}
    github-token: ${{ secrets.GATEWAY_SCHEMA_READ_TOKEN }}
    path: schema
```

Ожидаемый commit хранится отдельно от скачанного manifest. Проверка в gateway:
`node scripts/schema-artifact.mjs verify schema FULL_COMMIT_SHA`.
Frontend хранит проверенную копию вместе с pin; обычному CI token и соседний checkout
не нужны. Это также сохраняет воспроизводимость после истечения 90 дней хранения
artifact. Обновлять pin и копию нужно одним reviewable коммитом.

## Локальная воспроизводимость

Из checkout нужного SHA: `npm ci --ignore-scripts && make schema-artifact`.
Результат находится в `dist/graphql-schema`. Frontend codegen читает именно копию
SDL из `schema/`, а не соседний gateway. Полная активация generated TypeScript вместо
рукописных API-типов остаётся отдельным этапом C6.
