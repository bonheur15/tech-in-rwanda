# Rwanda Free Space

An Astro and Tailwind editorial frontend backed by a small Go API. Development uses two processes for fast frontend refreshes. Production uses one non-root container where Go serves both the API and Astro's compiled static files.

## Architecture

- `src/`: static Astro frontend and generated TypeScript API client.
- `backend/internal/features/`: API features grouped with their contracts and handlers.
- `backend/internal/platform/`: configuration, request metadata, and HTTP infrastructure.
- `backend/cmd/gen-client/`: generator that turns the Go contract into `src/lib/api/generated.ts`.
- `backend/cmd/api/`: production API entrypoint with graceful shutdown and a container healthcheck command.

The public API starts at `/api/v1`. The demonstration endpoint is `GET /api/v1/server-time`; liveness is `GET /api/v1/healthz`.

## Development

Install the frontend dependencies once, then start both servers:

```sh
npm install
make dev
```

Open `http://127.0.0.1:4321/server-time/`. Astro runs on port 4321 and Go runs on port 8080. Stopping `make dev` stops both processes.

When a Go API contract changes, regenerate the browser client:

```sh
make generate
```

Run the full verification gate with `make check`, or build and smoke-test the production topology with `make smoke`.

## Docker

```sh
make docker-build
make docker-run
```

Then open `http://localhost:8080/server-time/`. The multi-stage image builds Astro with Node, compiles a static Go binary, and copies only the binary and built site into a distroless non-root runtime image.
