#!/usr/bin/env bash
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  printf '%s\n' 'Для хука форматирования необходим jq.' >&2
  exit 2
fi

if ! patch=$(jq -er '.tool_input.command | select(type == "string")'); then
  printf '%s\n' 'Не удалось прочитать содержимое патча из события Codex.' >&2
  exit 2
fi

files=()
while IFS= read -r line; do
  case "$line" in
    '*** Add File: '*) file=${line#'*** Add File: '} ;;
    '*** Update File: '*) file=${line#'*** Update File: '} ;;
    '*** Move to: '*) file=${line#'*** Move to: '} ;;
    *) continue ;;
  esac
  if [[ "$file" == *.go && -f "$file" && ! -L "$file" ]]; then
    files+=("$file")
  fi
done <<< "$patch"

if (( ${#files[@]} == 0 )); then
  exit 0
fi

if ! command -v gofmt >/dev/null 2>&1; then
  printf '%s\n' 'Для форматирования Go-файлов необходим gofmt.' >&2
  exit 2
fi

if ! gofmt -w -- "${files[@]}"; then
  printf '%s\n' 'Не удалось отформатировать Go-файлы.' >&2
  exit 2
fi
