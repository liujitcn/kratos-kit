#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec pnpm --dir "${project_root}/frontend/taro-app" "$@"
