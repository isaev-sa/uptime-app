# Backend

## Локальный запуск

Требуются Docker и Docker Compose. Из корня репозитория задайте секрет длиной не менее 32 байт и запустите сервисы:

```sh
export JWT_SECRET='replace-with-a-random-secret-at-least-32-bytes-long'
docker compose up --build
```

PostgreSQL сохраняет данные в томе `postgres_data`. При запуске бэкенд применяет новые версионированные SQL-миграции из `internal/storage/migrations`; повторное применение уже выполненных миграций не происходит. Ошибка миграции останавливает запуск. Для изменения схемы добавляйте новый SQL-файл с последовательным именем, не изменяя уже применённые файлы.

Для запуска без Compose задайте `DATABASE_URL` (PostgreSQL URL), `JWT_SECRET` и выполните `go run ./cmd/server` из `backend/`. Параметры окружения:

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `DATABASE_URL` | обязательна | Строка подключения к PostgreSQL; в Compose формируется автоматически. |
| `JWT_SECRET` | обязательна | Секрет подписи JWT, не менее 32 байт; используйте случайное значение. |
| `LISTEN_ADDR` | `:8080` | Адрес HTTP-сервера. |
| `ALLOWED_ORIGIN` | `http://localhost:3000` | Единственный разрешённый origin клиента. |
| `UPLOAD_DIR` | `uploads/files` | Каталог общих файлов; в Compose находится в постоянном томе `avatar_data`. |
| `AVATAR_DIR` | `uploads/avatars` | Каталог для аватаров; в Compose используется постоянный том `avatar_data`. |
| `COOKIE_SECURE` | `false` | Установите `true` при работе по HTTPS. |
| `ACCESS_TTL` | `15m` | Срок действия access-токена в формате Go duration. |
| `REFRESH_TTL` | `720h` | Срок действия refresh-токена в формате Go duration. |
| `POSTGRES_PASSWORD` | `uptime_local` | Пароль локального PostgreSQL в Compose; замените вне локальной разработки. |

Для браузерного клиента используйте `credentials: 'include'`. Cookie имеет `SameSite=Lax`, поэтому фронтенд и API должны быть на одном сайте; порты могут различаться. Для production необходимы HTTPS, `COOKIE_SECURE=true`, соответствующий `ALLOWED_ORIGIN` и безопасные секреты. При изменении пароля существующего тома PostgreSQL обновите пароль в самой базе: переменная инициализации меняет его только при первом создании тома.

## API

Общая загрузка: `POST /files` с заголовком `Authorization: Bearer <access_token>` и multipart-полем `file`. Принимается любой тип файла размером до 20 МиБ; ответ HTTP 201 содержит `url`. По этому URL файл доступен без авторизации как скачивание, поэтому не загружайте туда конфиденциальные данные. Имя на диске генерируется сервером. Например:

```sh
curl -H "Authorization: Bearer $ACCESS_TOKEN" -F 'file=@document.pdf' http://localhost:8080/files
```

Запросы авторизации и изменения профиля принимают `Content-Type: application/json`. `POST /profile/avatar` принимает авторизованный multipart-запрос с полем `avatar` (JPEG или PNG до 5 МиБ, до 4096 × 4096 пикселей). Возвращаемый `avatar_url` доступен через `GET /avatars/...` и присутствует в профиле и ответах авторизации. Файлы сохраняются локально; храните каталог вместе с резервной копией базы данных. Регистрация сразу создаёт сессию и возвращает HTTP 201; вход и обновление возвращают HTTP 200. Ответ содержит `access_token`, `token_type`, `expires_at`, `email` и `name` пользователя. Refresh-токен возвращается только в `HttpOnly` cookie; при каждом обновлении он заменяется. Сохраните новую cookie для следующего запроса. `POST /auth/refresh/logout` отзывает текущую refresh-сессию и удаляет cookie; ранее выданный access-токен действует до истечения своего срока.

```sh
curl -i -c cookies.txt -H 'Content-Type: application/json' \
  -d '{"name":"Анна","email":"user@example.com","password":"password123"}' \
  http://localhost:8080/auth/register
curl -i -c cookies.txt -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"password123"}' \
  http://localhost:8080/auth/login
curl -i -b cookies.txt -c cookies.txt -H 'Content-Type: application/json' \
  -d '{}' http://localhost:8080/auth/refresh
```

При регистрации имя обязательно и должно содержать от 1 до 100 символов после удаления пробелов по краям. Имена существующих пользователей при миграции заполняются частью email до `@`. Пароль должен содержать от 8 до 72 байт в UTF-8. При повторной регистрации возвращается 409, при неверном входе или refresh-токене — 401, при некорректных данных регистрации — 400.

### Точки мониторинга

`POST /monitors` принимает bearer-токен и JSON с `url` и `interval_seconds`. На одного пользователя допускается до 1000 точек; при превышении возвращается HTTP 409. `GET /monitors` возвращает объект с массивом `items` (до 50 записей) и признаком `has_more`. Чтобы получить следующую страницу, передайте `before` и `before_id` из полей `created_at` и `id` последней записи предыдущей страницы. Обе операции доступны только владельцу точек.

## Проверки

Из `backend/` выполните `go test ./...` и `go build ./...`. Для интеграционного теста укажите `TEST_DATABASE_URL` как PostgreSQL URL для тестовой базы и запустите `go test ./...`. Тест создаёт и удаляет отдельную схему и требует права `CREATE` на базу; без переменной он пропускается.
