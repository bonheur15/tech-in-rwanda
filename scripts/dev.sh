#!/usr/bin/env bash
set -Eeuo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_dir"

api_address="${APP_ADDR:-127.0.0.1:8787}"
api_origin="${PUBLIC_API_BASE_URL:-http://$api_address}"
allowed_origins="${APP_ALLOWED_ORIGINS:-http://localhost:4321,http://127.0.0.1:4321}"

cleanup() {
  trap - EXIT INT TERM
  for pid in "${api_pid:-}" "${web_pid:-}"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
  wait "${api_pid:-}" "${web_pid:-}" 2>/dev/null || true
}

trap 'status=$?; cleanup; exit "$status"' EXIT INT TERM

echo "Go API:      http://$api_address"
echo "Astro site:  http://127.0.0.1:4321"
echo "API test:    http://127.0.0.1:4321/server-time/"

APP_ADDR="$api_address" APP_ALLOWED_ORIGINS="$allowed_origins" go run ./backend/cmd/api &
api_pid=$!

PUBLIC_API_BASE_URL="$api_origin" npm run dev -- --host 127.0.0.1 &
web_pid=$!

set +e
wait -n "$api_pid" "$web_pid"
status=$?
set -e
exit "$status"
