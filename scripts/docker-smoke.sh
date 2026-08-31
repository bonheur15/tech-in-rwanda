#!/usr/bin/env bash
set -Eeuo pipefail

image="${IMAGE:-rwanda-free-space:local}"
suffix="$$-$RANDOM"
container="rfs-smoke-$suffix"
volume="rfs-smoke-data-$suffix"

cleanup() {
  trap - EXIT INT TERM
  docker rm -f "$container" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
}
trap 'status=$?; cleanup; exit "$status"' EXIT INT TERM

docker build -t "$image" .
docker volume create "$volume" >/dev/null
docker run -d --name "$container" -p 127.0.0.1::8080 -v "$volume:/data" \
  -e APP_ENV=production \
  -e PUBLIC_ORIGIN=https://rwanda-free-space.example \
  -e SESSION_PEPPER=smoke-session-pepper-with-at-least-32-characters \
  -e OTP_PEPPER=smoke-otp-pepper-with-at-least-32-characters \
  -e MAIL_MODE=smtp -e SMTP_ADDRESS=mail.example:587 \
  -e SMTP_USERNAME=smoke -e SMTP_PASSWORD=smoke -e SMTP_FROM=smoke@example.com \
  -e TURNSTILE_SECRET=smoke-non-test-turnstile-secret \
  -e PUBLIC_TURNSTILE_SITE_KEY=smoke-non-test-site-key \
  "$image" >/dev/null

port="$(docker port "$container" 8080/tcp | awk -F: 'NR==1 {print $NF}')"
healthy=false
for _ in {1..100}; do
  if [[ "$(docker inspect -f '{{.State.Health.Status}}' "$container")" == healthy ]] && curl -fsS "http://127.0.0.1:$port/api/v1/healthz" >/dev/null; then
    healthy=true
    break
  fi
  sleep .2
done
if [[ "$healthy" != true ]]; then
  docker logs "$container"
  exit 1
fi

docker exec "$container" test -f /data/database/blog.sqlite3
docker stop -t 15 "$container" >/dev/null
[[ "$(docker inspect -f '{{.State.ExitCode}}' "$container")" == 0 ]]
echo "Docker supervision, health, persistence and graceful shutdown smoke test passed."
