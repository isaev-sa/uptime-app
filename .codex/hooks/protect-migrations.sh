#!/usr/bin/env bash
set -euo pipefail

if ! patch=$(jq -er '.tool_input.command | select(type == "string")'); then
  printf '%s\n' 'Не удалось прочитать содержимое патча из события Codex.' >&2
  exit 2
fi

if ! root=$(git rev-parse --show-toplevel); then
  printf '%s\n' 'Не удалось определить корень репозитория.' >&2
  exit 2
fi

if ! listing=$(mktemp); then
  printf '%s\n' 'Не удалось подготовить проверку миграций.' >&2
  exit 2
fi
trap 'rm -f -- "$listing"' EXIT
if ! git -C "$root" ls-tree -r -z --name-only --full-tree HEAD -- backend/internal/storage/migrations > "$listing"; then
  printf '%s\n' 'Не удалось получить список закоммиченных миграций.' >&2
  exit 2
fi

while IFS= read -r line; do
  case "$line" in
    '*** Add File: '*) file=${line#'*** Add File: '} ;;
    '*** Update File: '*) file=${line#'*** Update File: '} ;;
    '*** Delete File: '*) file=${line#'*** Delete File: '} ;;
    '*** Move to: '*) file=${line#'*** Move to: '} ;;
    *) continue ;;
  esac
  if [[ -z "$file" ]] || ! absolute=$(realpath -ms -- "$file") || ! resolved=$(realpath -m -- "$file"); then
    printf '%s\n' 'Не удалось определить путь файла в патче.' >&2
    exit 2
  fi
  while IFS= read -r -d '' committed; do
    if [[ "$absolute" == "$root/$committed" || "$resolved" == "$root/$committed" ]]; then
      printf 'Нельзя изменять или удалять закоммиченную миграцию: %s. Создайте новую миграцию.\n' "$file" >&2
      exit 2
    fi
  done < "$listing"
done <<< "$patch"
