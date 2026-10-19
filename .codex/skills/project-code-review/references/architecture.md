# Архитектура Uptime App

Снимок существующих соглашений; источники истины — `AGENTS.md` проектов и ревьюируемый код. Пути ниже относительно корня репозитория. При расхождении сверяй текущие файлы. Обоснованная смена архитектуры возможна; оценивай согласованность всего изменения, а не сходство с этим снимком.

## Backend

Go-модуль `uptime-app/backend`, Go 1.25.4. Транспорт — стандартный `net/http`/`ServeMux`, PostgreSQL через GORM. Дополнительного обязательного framework или слоя controllers/usecases здесь нет.

| Место | Ответственность и направление зависимостей |
| --- | --- |
| `cmd/server/main.go` | Composition root: конфигурация → БД → repository → auth/monitor services → HTTP server; shutdown и закрытие ресурсов. Не место для бизнес-правил. |
| `internal/config/config.go` | Чтение и проверка environment; секреты не в коде. Новая настройка отражается в очищенном `.env.example`. |
| `internal/server/server.go`, `monitors.go` | Маршруты `/api/v1`, DTO/JSON, авторизация, HTTP-статусы, cookies/CORS, загрузка и выдача аватаров. Обработчик вызывает сервис с `r.Context()`, не GORM. Файловый I/O аватаров сейчас находится здесь; не требуй отдельный storage-layer без причины. |
| `internal/auth/auth.go` | Пользователь, refresh-сессия, интерфейс `auth.Repository`; валидация, bcrypt, JWT HS256, генерация refresh token и хеширование, ротация сессий. Не импортирует server/store/GORM. |
| `internal/monitor/monitor.go` | Модель монитора, `monitor.Repository`, проверка URL и интервала, создание и чтение точек. Не импортирует transport/store. Сейчас хранит настройки, **не выполняет HTTP-проверки сайтов**. |
| `internal/store/postgres.go`, `monitors.go` | Реализация доменных repository-интерфейсов, приватные GORM-модели и преобразования, parameterized SQL, транзакции и перевод ошибок БД. Зависит от auth/monitor; обратной зависимости нет. |
| `migrations/*.up.sql`, `*.down.sql` | Версионированная схема PostgreSQL. Не заменяй миграции неявным AutoMigrate при запуске сервиса. |
| `cmd/openapi/`, `docs/docs.go`, `internal/server/swagger.go` | Генерация/валидация OpenAPI из Swaggo, встраивание контракта/Swagger UI. `docs/openapi.{json,yaml}` и `docs/redoc.html` генерируются и коммитятся вместе с изменением API. |

`cmd/` — точки входа; `internal/` — приложение; `pkg/` допустим только для действительно внешне переиспользуемого пакета, сейчас не требуется. Растущий код группируется по функции.

Текущие инварианты: доступ определяется сервером по Bearer access token; ID владельца берётся из аутентификации. Мониторы фильтруются по `user_id`, порядок выдачи детерминирован. Диапазон интервала — 1…2147483647 секунд; URL — http/https без credentials. Сервис не совершает запрос при создании монитора: SSRF-проверки DNS/redirect нужны при появлении сетевого выполнения, не как фиктивный дефект текущего хранения URL.

Пароли хранятся как bcrypt-хеши; refresh token в БД — хеш, в браузере — HttpOnly cookie с Path `/api/v1/auth`; access token короткоживущий. Создание пользователя с сессией и ротация refresh выполняются атомарно в store. Смотри фактическую реализацию на предмет сохранения гарантий, а не считай их автоматически выполненными.

Тесты рядом с исходниками: `internal/auth/auth_test.go`, `internal/server/*_test.go` с fake repositories/httptest, проверки Swagger в `swagger_test.go`. `TestMonitorPostgres` в `monitors_postgres_test.go` требует `TEST_DATABASE_URL`, создаёт временную схему в транзакции и делает rollback. Отсутствие переменной означает skip, не подтверждение корректности PostgreSQL.

## Frontend

Next.js 16.3.5 App Router, React 19.2.8, TypeScript strict. Версии — из `frontend/package.json`, точное разрешение — из lockfile. Правила установленного Next.js проверяй по локальным docs.

| Место | Ответственность |
| --- | --- |
| `src/app/` | Маршруты, layouts, глобальные стили; директории маршрутов lowercase. Есть `/login`, `/register`, `/dashboard`, `/profile`; `layout.tsx` подключает AuthProvider. Server/Client boundary определяется потребностью в browser API, state и hooks, не единым требованием делать все страницы серверными. |
| `src/app/dashboard/monitor-config.ts`, `monitor-list.tsx` | Логика интервалов и UI конкретного dashboard; route-specific код хранится рядом со страницей. |
| `src/components/` | Действительно переиспользуемые компоненты (`auth-form.tsx`, `auth-provider.tsx`). Не обязательная свалка всех route-specific компонентов. |
| `src/lib/auth-api.ts`, `monitor-api.ts` | HTTP-клиенты, формы API DTO, `ApiError`, URL backend из `NEXT_PUBLIC_API_URL`. Не доступ к PostgreSQL/секретам сервера. Для multipart браузер задаёт boundary. |
| `src/components/auth-provider.tsx` | React context и `useAuth`, access token в памяти, восстановление через refresh cookie, объединение конкурентных refresh-запросов, один повтор `authorizedFetch` после 401, очистка состояния при logout. UI-декодирование JWT не является серверной проверкой подписи. |
| `public/` | Статика; не место для приватных файлов и секретов. |

Компоненты/типы — PascalCase; функции/переменные — camelCase, hooks — `use…`. Go: короткие lowercase packages, PascalCase exports, camelCase private identifiers, идиоматичные initialisms `ID`, `URL`, `HTTP`. Сохраняй локальные соглашения, не выдумывай новые правила именования файлов TSX.

Frontend взаимодействует с самостоятельным Go API; Next API routes, server actions, BFF, ORM во frontend и общий workspace package manager сейчас не обязательны. Добавление такого слоя требует конкретного обоснования и проверки границ, а не механического запрета.

Vitest/jsdom и Testing Library настроены в `vitest.config.mts`; тесты colocated (`monitor-config.test.ts`, `monitor-list.test.tsx`). ESLint — flat config с next/core-web-vitals и next/typescript. `npm test`, `npm run lint`, `npm run build` — команды проекта. Coverage provider для Vitest в зависимостях сейчас не установлен: не запускай автоустановку и не выдумывай численный порог.
