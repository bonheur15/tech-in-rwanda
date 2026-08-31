# Rwanda Free Space

Rwanda Free Space is a same-origin publishing platform for constructive criticism of technology in Rwanda. Astro SSR owns public rendering and the React/TipTap writing workspace. Go owns authentication, authorization, editorial state, media, comments, persistence, and operational tooling.

There are deliberately no passwords, JWTs, analytics libraries, page-view records, or reader-behavior tracking.

## Local development

```sh
npm install
cp .env.example .env
make bootstrap-superadmin EMAIL=editor@example.com HANDLE=editor NAME="First Editor"
make dev
```

Open `http://127.0.0.1:4321/` for the publication. Staff enter the private newsroom directly at `/admin`; it is intentionally absent from public navigation. Development OTP codes are printed only by the Go terminal mailer. The terminal sender is rejected when `APP_ENV=production`.

The newsroom includes drafts with two-second last-save-wins autosave, manual checkpoints, side-by-side version inspection, restore, immediate or approval publishing, forking, taxonomy, thumbnails and positioned inline media, review and comment moderation queues, reader moderation, author access modes, sessions, audit history, and profile settings. Reader accounts at `/account` provide passwordless sign-in, bookmarks, comments, privacy/avatar controls, sessions, and the two supported deletion modes.

`make dev` starts Astro on 4321 and Go on 8787 with coordinated shutdown. Astro proxies `/api/*` and `/media/*`, so browsers always use a same-origin API. Local data lives under `.data/`.

## Data and security model

- SQLite enables WAL, foreign keys, a five-second busy timeout, bounded connections, and embedded forward-only migrations.
- Staff and reader tokens are opaque. Only HMAC-SHA256 digests are stored.
- Staff and reader sessions use separate cookies and CSRF secrets, so one email can safely use both capabilities in one browser.
- Production requires HTTPS, secure `__Host-` cookies, SMTP, real Turnstile credentials, and independent 32-character or longer session and OTP peppers.
- Reader Turnstile tokens are verified server-side and are never trusted from client state.
- Published pages read immutable versions through `published_version_id`; autosave can never alter the live article.
- Uploads accept decoded JPEG/PNG only, enforce 15 MiB and 30 MP limits, strip metadata by re-encoding, and create four responsive derivatives.

Run `make check` for generated-client drift, formatting, vet, race tests, frontend tests, Astro checks, production builds, the compiled two-process smoke test, and Docker acceptance. Docker must be available for this complete gate. Use `make smoke` alone for a faster local check of the compiled two-process topology.

## Operations

```sh
make migrate-status
make backup FILE=rfs-backup.tar.gz
make verify-backup FILE=rfs-backup.tar.gz
make media-check
make media-cleanup
make recover-account EMAIL=editor@example.com
```

Backups use SQLite `VACUUM INTO` for a consistent snapshot and include a SHA-256 media manifest. Verification also runs SQLite integrity and foreign-key checks. Keep backup archives outside the application volume. Media cleanup only purges assets that have been orphaned for at least seven days and are still unreferenced.

## Docker

The production image leaves runtime identity and filesystem isolation to the hosting platform. `tini` starts a signal-aware supervisor, Go listens only on `127.0.0.1:8081`, and Astro is the sole public listener on `:8080`. The health check crosses Astro's proxy and verifies the Go database connection.

```sh
cp .env.example .env
# Set APP_ENV=production, PUBLIC_ORIGIN, SMTP values, real Turnstile keys, and strong peppers.
make docker-build
make docker-run
```

Mount one persistent volume at `/data`. This SQLite and local-media topology supports exactly one active application container.
