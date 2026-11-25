#!/usr/bin/env bash
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  printf '%s\n' 'Для хука ESLint необходим jq.' >&2
  exit 2
fi

if ! patch=$(jq -er '.tool_input.command | select(type == "string")'); then
  printf '%s\n' 'Не удалось прочитать содержимое патча из события Codex.' >&2
  exit 2
fi

root=$(git rev-parse --show-toplevel)
files=()
while IFS= read -r line; do
  case "$line" in
    '*** Add File: '*) file=${line#'*** Add File: '} ;;
    '*** Update File: '*) file=${line#'*** Update File: '} ;;
    '*** Move to: '*) file=${line#'*** Move to: '} ;;
    *) continue ;;
  esac
  if [[ ! -f "$file" || -L "$file" ]]; then
    continue
  fi
  file=$(realpath "$file")
  if [[ "$file" == "$root/frontend/"* ]]; then
    case "$file" in
      *.js|*.jsx|*.mjs|*.cjs|*.ts|*.tsx|*.mts|*.cts) files+=("$file") ;;
    esac
  fi
done <<< "$patch"

if (( ${#files[@]} == 0 )); then
  exit 0
fi

if [[ ! -x "$root/frontend/node_modules/.bin/eslint" ]]; then
  printf '%s\n' 'Для проверки фронтенда установите зависимости: cd frontend && npm ci.' >&2
  exit 2
fi

cd "$root/frontend"
if ! ./node_modules/.bin/eslint -- "${files[@]}"; then
  printf '%s\n' 'ESLint обнаружил ошибки во фронтенде.' >&2
  exit 2
fi
