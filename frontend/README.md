# Frontend

Для одновременного запуска фронтенда, бэкенда и PostgreSQL из корня проекта необходимы Docker Compose, Node.js и npm:

```sh
export JWT_SECRET='replace-with-a-random-secret-at-least-32-bytes-long'
./scripts/dev.sh
```

Скрипт установит зависимости фронтенда, если они отсутствуют, запустит бэкенд и базу через Docker Compose, затем запустит Next.js. Фронтенд доступен на `http://localhost:3000`, API — на `http://localhost:8080`. `Ctrl+C` останавливает фронтенд; контейнеры продолжают работать и останавливаются командой `docker compose stop`. Данные PostgreSQL сохраняются.

Для отдельного запуска фронтенда выполните `npm ci` и `npm run dev` из каталога `frontend/`. Для производственной сборки используйте `npm run build`.

По умолчанию фронтенд обращается к `http://localhost:8080`. Для другого адреса задайте `NEXT_PUBLIC_API_URL` до сборки или запуска Next.js. Origin фронтенда должен совпадать с `ALLOWED_ORIGIN` бэкенда. В браузере фронтенд и API должны находиться на одном сайте, чтобы работала cookie `SameSite=Lax`; для HTTPS на бэкенде включите `COOKIE_SECURE=true`.

После регистрации или входа access-токен хранится только в памяти страницы, а refresh-токен — в `HttpOnly` cookie бэкенда. При перезагрузке страница обновляет сессию с помощью cookie; access-токен обновляется до истечения срока действия. Для запуска API и PostgreSQL см. `backend/README.md`.
