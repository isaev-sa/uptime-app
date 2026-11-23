#!/usr/bin/env bash
set -euo pipefail

project_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_dir"

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
  printf '%s\n' 'Для запуска необходимы Docker и Docker Compose.' >&2
  exit 1
fi
if ! command -v npm >/dev/null 2>&1; then
  printf '%s\n' 'Для запуска фронтенда необходимы Node.js и npm.' >&2
  exit 1
fi

if [[ ! -x frontend/node_modules/.bin/next ]]; then
  npm ci --prefix frontend
fi

docker compose up -d --build
printf '\n%s\n' 'Бэкенд: http://localhost:8080' 'Фронтенд: http://localhost:3000' 'Для остановки фронтенда нажмите Ctrl+C. Контейнеры остановите командой docker compose stop.'
exec npm run dev --prefix frontend
