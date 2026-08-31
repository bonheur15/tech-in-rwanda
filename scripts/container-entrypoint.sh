#!/bin/sh
set -eu
/app/bin/rfs-api & api_pid=$!
node /app/dist/server/entry.mjs & web_pid=$!
stopping=0
shutdown() { stopping=1; trap - TERM INT; kill -TERM "$api_pid" "$web_pid" 2>/dev/null || true; wait "$api_pid" "$web_pid" 2>/dev/null || true; }
trap shutdown TERM INT EXIT
while kill -0 "$api_pid" 2>/dev/null && kill -0 "$web_pid" 2>/dev/null; do sleep 1; done
if [ "$stopping" -eq 1 ]; then trap - EXIT; exit 0; fi
exit 1
