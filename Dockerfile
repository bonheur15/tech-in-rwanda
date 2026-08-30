# syntax=docker/dockerfile:1.7

FROM node:24-alpine AS web-dependencies
WORKDIR /source
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

FROM web-dependencies AS web-build
COPY astro.config.mjs tsconfig.json ./
COPY public ./public
COPY src ./src
RUN npm run build

FROM golang:1.25-alpine AS go-build
WORKDIR /source
COPY go.mod ./
COPY backend ./backend
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/rwanda-free-space ./backend/cmd/api

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=go-build --chown=65532:65532 /out/rwanda-free-space /app/rwanda-free-space
COPY --from=web-build --chown=65532:65532 /source/dist /app/public

ENV APP_ADDR=:8080
ENV STATIC_DIR=/app/public
EXPOSE 8080
USER 65532:65532
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD ["/app/rwanda-free-space", "healthcheck"]
ENTRYPOINT ["/app/rwanda-free-space"]
