# Автоматические проверки

Скрипт `scripts/run_checks.py` использует Python 3 standard library, Git и уже установленные инструменты проекта. Запускай из корня репозитория либо передай `--repo /path/to/repo`. Без режима он проверяет `--worktree`; выбор «чистая ветка → origin/main» выполняет агент по SKILL.md.

```sh
# Статические проверки staged + unstaged + untracked, без изменения индекса
python3 .codex/skills/project-code-review/scripts/run_checks.py --worktree
# Полная проверка текущей ветки относительно merge-base с main
python3 .codex/skills/project-code-review/scripts/run_checks.py --base origin/main --tests --build
# Один коммит / только индекс
python3 .codex/skills/project-code-review/scripts/run_checks.py --commit HEAD --tests --build
python3 .codex/skills/project-code-review/scripts/run_checks.py --staged --tests --build
# Только при наличии согласованной тестовой PostgreSQL в TEST_DATABASE_URL
python3 .codex/skills/project-code-review/scripts/run_checks.py --worktree --tests --integration
```

Режимы взаимоисключающие. `--base REF --head REF` использует merge-base; `--commit` — первого родителя, включая root commit. Ref разрешается в SHA до выполнения diff. `--base`/`--commit` требуют чистое дерево и HEAD целевой версии; `--staged` отказывается работать при unstaged/untracked, чтобы тесты не проверяли другое содержимое. `--worktree` включает новые файлы и агрегирует staged/unstaged. Если нужны отдельные изменения индекса при наличии других файлов, подготовь отдельную копию нужного snapshot, не stash/reset пользовательской работы.

| Когда | Статические команды (перед тестами/сборкой) |
| --- | --- |
| Всегда | `git diff --check` выбранного diff (для root commit — `git diff-tree --root --check`) |
| Любой путь `backend/**` | `gofmt -l` исходников приложения, исключая vendor; `go vet ./...` из backend |
| Любой путь `frontend/**` | `npm run lint`; локальный `node_modules/.bin/next typegen`, затем `node_modules/.bin/tsc --noEmit --incremental false` из frontend |

`next typegen` обязателен до tsc: на чистом checkout Next.js ещё не создал `LayoutProps` и другие route helpers. Это требование локальных docs `frontend/node_modules/next/dist/docs/01-app/03-api-reference/06-cli/next.md`. Typegen загружает next.config в production build phase и создаёт игнорируемые `.next/types` и `next-env.d.ts`; при его сбое TypeScript отмечается `blocked`, остальные независимые шаги продолжаются.

Переименования анализируются как удаление/добавление, включая перенос между проектами. Не запускай Go/Next команды при изменениях только `.codex/` или корневых docs. Изменение общих правил/инструментов с влиянием на приложения может требовать отдельных ручных проверок, даже если prefix-routing их не выбрал.

`--tests` добавляет `go test -count=1 -timeout=120s -cover ./...` и/или `npm test`. `--build` добавляет `npm run build` для frontend. Тесты и build исполняют код из checkout: сначала оцени изменённые scripts/config на неожиданные действия и соблюдай доступные разрешения; для недоверенного PR используй изолированную среду без секретов. Статический анализ тоже загружает конфигурацию проекта.

Без `--integration` скрипт удаляет `TEST_DATABASE_URL` только из среды дочерних процессов; тест PostgreSQL явно отмечен `skipped`. `--integration` включает Go tests с переданной переменной; если она отсутствует, результат `blocked`. Не используй production DB. При изменении миграций/store недостаточно unit tests: выполни доступный интеграционный тест либо явно укажи ограничение. Успех `go-tests` с `--integration` проверяй по логу: текущий `TestMonitorPostgres` вызывается из общего набора; если тест позже переименуют/изменят условия skip, обнови workflow.

Go использует локальный toolchain (`GOTOOLCHAIN=local`), cached modules (`GOPROXY=off`) и `-mod=readonly`, чтобы не скачивать/не обновлять зависимости. Отсутствующие dependencies/toolchain надо подготовить отдельно в разрешённой среде. Скрипт не выполняет `npm install`, `npx`, `go install`, `gofmt -w`, автофиксы или OpenAPI codegen. Команды typegen/тестов/сборки могут создавать обычные cache/build outputs; Next.js может обновить `next-env.d.ts`/настройки. Скрипт сравнивает SHA-256 содержимого diff/индекса и новых файлов до/после запуска: изменение snapshot помечается `blocked`; изучи его без автоматического отката. Игнорируемые Git cache/build outputs в fingerprint не входят.

Статические команды проверяют всё затронутое приложение, не только строки diff. При сбое одного шага выполняются остальные независимые шаги. Недостающий tool/bin — `blocked`; ненулевой код команды — `failed` (причину кода/окружения уточняй по логу). Таймаут — `blocked`; default 300 секунд на команду, настраивается `--timeout N`; зависшие процессы завершаются вместе с группой.

Логи и `report.json` сохраняются во временном каталоге `uptime-review-*` вне рабочего дерева. JSON содержит режим, SHA, пути изменений, применимые проекты, команды, cwd, статусы и пути логов; stdout печатает статусы и адрес отчёта. Для локального режима target SHA — исходный HEAD, а не коммит локальных изменений. Логи могут содержать данные из тестов: читай локально и редактируй чувствительные значения перед цитированием.

Коды выхода: `0` — нет failed/blocked среди запрошенных проверок; `1` — есть failed; `2` — есть blocked (в том числе вместе с failed). `skipped` и `not_applicable` не означают успешный тест. `git diff --check` не охватывает untracked: новые файлы проверяются языковыми инструментами и вручную. Gofmt/ESLint/tsc/vet не являются полноценными сканерами секретов, уязвимых зависимостей и всего dead code. Эти вопросы требуют чеклиста и изучения usages; дополнительные сканеры применяй при наличии и необходимости, не выдавай их отсутствие за выполненный аудит.

Go `-cover` даёт фактическое покрытие исполняемых пакетов, но не гарантирует покрытие изменённых ветвей. У Vitest сейчас нет установленного coverage provider: оцени сценарии по тестам и не позволяй CLI автоматически его устанавливать.

Проверка самого runner без приложений/БД/сети:

```sh
python3 -B -m unittest discover -s .codex/skills/project-code-review/scripts -p 'test_*.py' -v
```
