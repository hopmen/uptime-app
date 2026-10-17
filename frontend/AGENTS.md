<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

# Правила фронтенда

## Структура

Фронтенд использует Next.js App Router и TypeScript. Маршруты, layouts и глобальные стили размещайте в `src/app/`, статические файлы — в `public/`. Имена директорий маршрутов должны быть в нижнем регистре. Компоненты и стили, относящиеся к одному маршруту, храните рядом с ним; в общую директорию компонентов переносите только действительно переиспользуемый UI.

## Команды

Запускайте эти команды из `frontend/`:

```sh
npm run dev    # Запуск локальной разработки
npm run lint   # Проверка ESLint
npm run build  # Проверка типов и production-сборка
```

После изменения зависимостей выполните `npm install`. Не коммитьте `node_modules/` и `.next/`.

## Стиль и тестирование

Используйте TypeScript. Компоненты именуйте в `PascalCase`; переменные, функции и хуки — в `camelCase`. Следуйте ESLint и действующим соглашениям App Router. Не добавляйте тестовый фреймворк без документирования его скриптов в `package.json`; тесты храните рядом с проверяемым кодом. Перед pull request с изменениями фронтенда выполните `npm run lint` и `npm run build`.
