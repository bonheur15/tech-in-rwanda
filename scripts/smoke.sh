#!/usr/bin/env bash
set -Eeuo pipefail
project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)";cd "$project_dir";tmp_dir="$(mktemp -d)";api_log="$tmp_dir/api.log";web_log="$tmp_dir/web.log"
cleanup() { trap - EXIT INT TERM; kill "${api_pid:-}" "${web_pid:-}" 2>/dev/null || true; wait "${api_pid:-}" "${web_pid:-}" 2>/dev/null || true; rm -rf "$tmp_dir"; }
trap 'status=$?; cleanup; exit "$status"' EXIT INT TERM
APP_ADDR=127.0.0.1:18081 DATABASE_PATH="$tmp_dir/blog.sqlite3" MEDIA_DIR="$tmp_dir/media" UPLOAD_TEMP_DIR="$tmp_dir/uploads" ./bin/rfs-api >"$api_log" 2>&1 & api_pid=$!
HOST=127.0.0.1 PORT=18080 INTERNAL_API_URL=http://127.0.0.1:18081 node dist/server/entry.mjs >"$web_log" 2>&1 & web_pid=$!
ready=false
for _ in {1..80}; do if curl -fsS http://127.0.0.1:18080/api/v1/healthz >/dev/null 2>&1; then ready=true; break; fi; sleep .1; done
if [[ "$ready" != true ]]; then cat "$api_log" "$web_log"; exit 1; fi
curl -fsS http://127.0.0.1:18080/ | grep -q "Rwanda Free Space"
curl -fsS http://127.0.0.1:18080/robots.txt | grep -q "Disallow: /admin/"
kill -0 "$api_pid";kill -0 "$web_pid";echo "SSR plus Go API smoke test passed."
