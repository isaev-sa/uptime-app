#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd -- "$script_dir/../../../../backend"

go tool swag init -g cmd/server/main.go --parseInternal --output docs --outputTypes json,yaml
npx --yes @redocly/cli@1.34.5 build-docs docs/swagger.json --output docs/redoc.html
