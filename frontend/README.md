# Uptime frontend

## Локальная авторизация

Скопируйте `.env.example` в `.env.local` и укажите адрес backend API:

```sh
NEXT_PUBLIC_API_URL=http://localhost:8080
```

Backend должен быть запущен с `FRONTEND_ORIGIN=http://localhost:3000`, чтобы браузер мог передавать refresh-cookie в запросах к API.

## Создание точек мониторинга

На `/dashboard` нажмите «Добавить сайт», укажите HTTP(S) URL и положительный целый интервал в секундах, минутах или часах. После нажатия «Создать» сайт появится в списке и останется там после перезагрузки страницы. Точки доступны только создавшему их пользователю. На этом этапе опрос сайтов ещё не выполняется.

Перед запуском backend примените миграции, включая `000004_create_monitors`:

```sh
cd ../backend
docker compose up -d postgres
docker compose run --rm migrate
```

## Проверки

```sh
npm test        # Vitest: интервалы и форма создания сайта
npm run lint
npm run build
```

### Автоматическая проверка в Codex

В `.codex/hooks.json` настроен хук `PostToolUse`: после создания, изменения,
удаления или переименования файлов в `frontend/` через `apply_patch`, `Edit`
или `Write` он выполняет `npm run lint` для всего фронтенда. Проверка работает
и при запуске Codex из подкаталога. Ошибки ESLint и ошибки запуска возвращаются
Codex для исправления; автоматического изменения файлов нет.

Установите зависимости командой `npm install` в `frontend/`. В Codex откройте
`/hooks`, проверьте и разрешите новый хук; после изменения определения хука
потребуется повторное разрешение. Если хук не появился в текущей сессии,
перезапустите Codex в доверенном проекте. Подробнее — в
[официальной документации хуков](https://learn.chatgpt.com/docs/hooks).

Хук реагирует на инструменты редактирования Codex. Изменения из IDE или через
shell-команды не вызывают эту проверку: после них запустите `npm run lint` вручную.

Тесты backend запускаются из `backend/` командой `go test ./...`. Для интеграционного теста с PostgreSQL задайте `TEST_DATABASE_URL` и запустите `go test ./internal/server -run TestMonitorPostgres -v`. Тест применяет миграции в отдельной схеме внутри транзакции и откатывает её после проверки.

## Запуск

This is a [Next.js](https://nextjs.org) project bootstrapped with [`create-next-app`](https://nextjs.org/docs/app/api-reference/cli/create-next-app).

## Getting Started

First, run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/app/building-your-application/optimizing/fonts) to automatically optimize and load [Geist](https://vercel.com/font), a new font family for Vercel.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js) - your feedback and contributions are welcome!

## Deploy on Vercel

The easiest way to deploy your Next.js app is to use the [Vercel Platform](https://vercel.com/new?utm_medium=default-template&filter=next.js&utm_source=create-next-app&utm_campaign=create-next-app-readme) from the creators of Next.js.

Check out our [Next.js deployment documentation](https://nextjs.org/docs/app/building-your-application/deploying) for more details.
