# Backend

## Swagger и OpenAPI

После запуска `go run ./cmd/server` документация доступна на том же сервере:

- `/swagger/index.html` — Swagger UI с возможностью выполнить запросы;
- `/swagger/openapi.json` — контракт OpenAPI 3.0.3 в JSON;
- `/swagger/openapi.yaml` — тот же контракт в YAML;
- `/swagger/doc.json` — совместимый адрес JSON-контракта.

UI и контракт встроены в бинарный файл, внешние CDN и файлы рядом с бинарником не нужны.
Адрес сервера определяется текущим origin, поэтому контракт работает с любым `SERVER_ADDRESS`.

В аннотациях обработчиков `internal/server/` описаны все операции API и получение
аватара: входные параметры, тела запросов, ответы, ошибки и авторизация.
Общие сведения и схемы авторизации заданы в `cmd/server/main.go`.

После изменения обработчиков или DTO обновите контракт из каталога `backend/`:

```sh
go generate ./docs
go fmt ./...
go test ./...
```

Генератор `cmd/openapi` использует закреплённые в `go.mod` версии Swag и kin-openapi:
читает Swagger-аннотации, преобразует результат в OpenAPI 3, уточняет bearer/cookie
авторизацию и проверяет корректность контракта перед сохранением. Генерируемые
`docs/openapi.json` и `docs/openapi.yaml` хранятся в Git; вручную их не редактируйте.
Проверка актуальности без изменения исходников:

```sh
go generate ./docs
git diff --exit-code -- docs/openapi.json docs/openapi.yaml
```

Для защищённых операций зарегистрируйтесь или войдите через UI, затем вставьте
`accessToken` в **Authorize → BearerAuth** (без префикса `Bearer`). Cookie
`refresh_token` браузер сохраняет и отправляет автоматически при работе на том же
origin. Вручную задать HttpOnly cookie через Swagger UI нельзя; для `refresh` и
`logout` сначала выполните `register` или `login` на этом сервере.
Для локального HTTP используйте `COOKIE_SECURE=false`; остальные настройки и БД
описаны в `.env.example` и `compose.yaml`.
