#!/usr/bin/env bash
set -Eeuo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$project_dir"

port="${SMOKE_PORT:-18080}"
log_file="$(mktemp)"

cleanup() {
  trap - EXIT INT TERM
  if [[ -n "${server_pid:-}" ]] && kill -0 "$server_pid" 2>/dev/null; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -f "$log_file"
}
trap 'status=$?; cleanup; exit "$status"' EXIT INT TERM

APP_ADDR="127.0.0.1:$port" STATIC_DIR=dist ./bin/rwanda-free-space >"$log_file" 2>&1 &
server_pid=$!

for _ in {1..40}; do
  if curl -fsS "http://127.0.0.1:$port/api/v1/healthz" >/dev/null; then
    break
  fi
  if ! kill -0 "$server_pid" 2>/dev/null; then
    cat "$log_file"
    exit 1
  fi
  sleep 0.1
done

curl -fsS "http://127.0.0.1:$port/" | grep -q "Rwanda Free Space"
curl -fsS "http://127.0.0.1:$port/server-time/" | grep -q "Go API client test"
first_time="$(curl -fsS "http://127.0.0.1:$port/api/v1/server-time" | sed -n 's/.*"iso":"\([^"]*\)".*/\1/p')"
sleep 0.05
second_time="$(curl -fsS "http://127.0.0.1:$port/api/v1/server-time" | sed -n 's/.*"iso":"\([^"]*\)".*/\1/p')"

if [[ -z "$first_time" || -z "$second_time" || "$first_time" == "$second_time" ]]; then
  echo "server-time endpoint did not produce two fresh timestamps" >&2
  exit 1
fi

echo "Smoke test passed: static pages and fresh Go API responses are available on port $port."
