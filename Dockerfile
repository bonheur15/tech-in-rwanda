# syntax=docker/dockerfile:1.7
FROM node:24-alpine AS web-dependencies
WORKDIR /source
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
  npm ci
FROM web-dependencies AS web-build
ENV ASTRO_TELEMETRY_DISABLED=1
COPY astro.config.mjs tsconfig.json ./
COPY public ./public
COPY src ./src
RUN npm run build
FROM web-dependencies AS web-runtime-dependencies
RUN npm prune --omit=dev
FROM golang:1.25-alpine AS go-build
WORKDIR /source
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download
COPY backend ./backend
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/rfs-api ./backend/cmd/api && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/blogctl ./backend/cmd/blogctl
FROM node:24-alpine AS runtime
RUN apk add --no-cache tini && addgroup -S -g 10001 rfs && adduser -S -D -H -u 10001 -G rfs rfs
WORKDIR /app
COPY --from=web-build /source/dist ./dist
COPY --from=web-runtime-dependencies /source/node_modules ./node_modules
COPY --from=web-runtime-dependencies /source/package.json ./package.json
COPY --from=go-build /out/rfs-api /out/blogctl ./bin/
COPY scripts/container-entrypoint.sh ./bin/container-entrypoint.sh
RUN chmod 0555 ./bin/container-entrypoint.sh && mkdir -p /data/database /data/media /data/tmp && chown -R 10001:10001 /app /data
ENV APP_ENV=production APP_ADDR=127.0.0.1:8081 DATABASE_PATH=/data/database/blog.sqlite3 MEDIA_DIR=/data/media UPLOAD_TEMP_DIR=/data/tmp HOST=0.0.0.0 PORT=8080 INTERNAL_API_URL=http://127.0.0.1:8081
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=20s --timeout=5s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/api/v1/healthz || exit 1
USER 10001:10001
ENTRYPOINT ["/sbin/tini", "--", "/app/bin/container-entrypoint.sh"]
