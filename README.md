# Laserbeak 

Laserbeak - это единая точка входа для фронтенда. Он предоставляет gqlgen GraphQL и делегирует пользовательские операции другим сервисам через GraphQL-over-HTTP. У него нет пользовательской бизнес-логики.

## Configuration

| Переменная | Стандратно | Значение |
|---|---|---|
| `PORT` | `8081` | HTTP порт |
| `ARCEE_GRAPHQL_URL` | `http://localhost:8080/query` | Arcee GraphQL endpoint |
| `ARCEE_TIMEOUT` | `5s` | тайм-аут для запросов в Arcee |
| `JWT_SECRET` | - | HS256 секрет |
| `JWT_ISSUER` | `arcee` | JWT издатель |
| `CORS_ORIGINS` | `http://localhost:5173` | CORS |

## Запуск

```bash
export JWT_SECRET=local-development-secret-change-me
make run
```

- GraphQL: `POST http://localhost:8081/graphql`
- Playground: `GET http://localhost:8081/playground`
- Health: `GET http://localhost:8081/health`

Регистрация и вход в систему являются публичными. Для других операций требуется `Authorization: Bearer <jwt>`. Шлюз проверяет тот же секрет JWT и пересылает исходный заголовок в другие сервисы.