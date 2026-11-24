#!/usr/bin/env bash
set -u

root=$(git rev-parse --show-toplevel 2>/dev/null) || { echo 'ERROR: run inside the repository' >&2; exit 2; }
backend=0
frontend=0
for arg in "$@"; do
  case "$arg" in
    --backend) backend=1 ;;
    --frontend) frontend=1 ;;
    *) echo "Usage: $0 [--backend] [--frontend]" >&2; exit 2 ;;
  esac
done
if (( ! backend && ! frontend )); then
  echo 'Specify --backend and/or --frontend based on the review scope.' >&2
  exit 2
fi
failed=0
run_check() {
  local label=$1
  shift
  echo "== $label =="
  if "$@"; then echo "PASS: $label"; else echo "FAIL: $label"; failed=1; fi
}

if (( backend )); then
  if command -v go >/dev/null && command -v gofmt >/dev/null; then
    check_format() {
      local output
      output=$(cd "$root/backend" && find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 -r gofmt -l) || return 1
      if [[ -n $output ]]; then printf '%s\n' "$output"; return 1; fi
    }
    run_check 'Go formatting' check_format
    run_check 'go vet ./...' bash -c 'cd "$1/backend" && go vet ./...' _ "$root"
  else
    echo 'SKIP: Go formatting and vet (Go tools unavailable)'
  fi
fi

if (( frontend )); then
  if ! command -v npm >/dev/null || ! command -v node >/dev/null; then
    echo 'SKIP: frontend checks (Node.js or npm unavailable)'
  elif [[ ! -d $root/frontend/node_modules ]]; then
    echo 'SKIP: frontend checks (dependencies not installed; npm ci was not run)'
  else
    has_script() { node -e 'const p = require(process.argv[1]); process.exit(p.scripts && p.scripts[process.argv[2]] ? 0 : 1)' "$root/frontend/package.json" "$1"; }
    if has_script lint; then
      run_check 'frontend lint' npm --prefix "$root/frontend" run lint
    else
      echo 'SKIP: frontend lint (no lint script configured)'
    fi
    if has_script typecheck; then
      run_check 'frontend typecheck' npm --prefix "$root/frontend" run typecheck
    elif [[ -x $root/frontend/node_modules/.bin/tsc && -f $root/frontend/tsconfig.json ]]; then
      run_check 'frontend typecheck' "$root/frontend/node_modules/.bin/tsc" --noEmit -p "$root/frontend/tsconfig.json"
    else
      echo 'SKIP: frontend typecheck (not configured)'
    fi
    echo 'SKIP: frontend production build (not a static check; run separately when appropriate)'
  fi
fi
exit "$failed"
