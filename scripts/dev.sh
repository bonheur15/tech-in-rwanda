#!/usr/bin/env bash
set -Eeuo pipefail
project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$project_dir"
cleanup(){ trap - EXIT INT TERM; for pid in "${api_pid:-}" "${web_pid:-}";do [[ -n "$pid" ]]&&kill "$pid" 2>/dev/null||true;done;wait "${api_pid:-}" "${web_pid:-}" 2>/dev/null||true; }
trap 'status=$?; cleanup; exit "$status"' EXIT INT TERM
mkdir -p .data/media .data/tmp
echo "Astro SSR: http://127.0.0.1:4321"; echo "Go API:   http://127.0.0.1:8787"
APP_ADDR=127.0.0.1:8787 DATABASE_PATH=.data/blog.sqlite3 MEDIA_DIR=.data/media UPLOAD_TEMP_DIR=.data/tmp PUBLIC_ORIGIN=http://127.0.0.1:4321 go run ./backend/cmd/api & api_pid=$!
INTERNAL_API_URL=http://127.0.0.1:8787 npm run dev -- --host 127.0.0.1 & web_pid=$!
set +e; wait -n "$api_pid" "$web_pid"; status=$?; set -e; exit "$status"
