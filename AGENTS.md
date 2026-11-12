# Repository Guidelines

## Project Overview

Warehouse Barcode is a standalone warehouse barcode label printing workstation integrated with **Accurate Online** (Indonesian ERP SaaS). Full-stack monorepo: Go backend (`fiber/v3` + `huma/v2` REST, pure-Go SQLite) and SolidJS SPA frontend (`web/`). Features a drag-and-drop label designer, CODE128/CODE39/EAN-13/QR/Data Matrix generation, Accurate inventory search and line-item import from 6 document types, and optional Google OAuth with roles `guest`/`user`/`admin`/`superadmin`.

## Architecture & Data Flow

Module: `github.com/awang-karisma/cim-warehouse-barcode` (Go 1.27, `CGO_ENABLED=0` everywhere).

Request flow: SolidJS SPA → Fiber v3 engine (`pkg/api/server.go`) → either raw Fiber routes (`auth_handlers.go`) or Huma v2 operations (`handlers.go`, `accurate_handlers.go`) → `pkg/storage` (SQLite) and/or `pkg/accurate` integration layer.

Key patterns:
- **Dual HTTP layer**: Huma v2 generates OpenAPI 3.1 + Scalar UI at `/docs`, `/openapi`; raw Fiber handlers cover auth cookie flows and admin user CRUD. Central `fiber.Config.ErrorHandler` emits `{"error": ...}`.
- **Auth**: `warehouse_session` cookie (HS256 JWT, 24h). Middleware enforces per-operation `minRole` via a role hierarchy. **Empty `GOOGLE_CLIENT_ID` = auth bypass mode** (backend skips checks; frontend mocks a `superadmin` session).
- **Accurate integration**: `Provider` supports isolated web and official accounts via `accounts.Pool`; each `session.Manager` and `adapter.Registry` is account-scoped. Credentials and session secrets in `accurate_accounts` are AES-256-GCM encrypted with `ACCURATE_ENCRYPTION_KEY`. The legacy `accurate_session` table is retained but unused.
- **Provider selection & DI**: `cmd/barcode-server/main.go` injects the storage-backed account pool through `api.Config`. Official provider functional options (`WithBaseURL`, `WithHTTPClient`, `WithSignatureSecret`, `WithNow`, `WithDisabledSource`) remain test seams.
- **Frontend embedding**: build tag `embed_frontend` (`cmd/barcode-server/embed.go` / `embed_stub.go`). Without it, the server serves `web/dist` from disk if present.
- **Migrations**: no CLI — `storage.New()` runs `CREATE TABLE IF NOT EXISTS` + non-destructive `ALTER TABLE` on startup (`pkg/storage/storage.go`). SQLite runs in WAL mode with 5000ms busy timeout via DSN pragmas.

## Key Directories

| Path | Purpose |
|---|---|
| `cmd/barcode-server/` | Entry point, SPA embed build tags, graceful shutdown |
| `pkg/api/` | Fiber/Huma server, middleware, all HTTP handlers |
| `pkg/auth/` | JWT + Google ID token verification |
| `pkg/storage/` | SQLite connection, migrations, CRUD |
| `pkg/model/` | Domain entities + enums (`vocabulary.go`) |
| `pkg/config/` | Viper/godotenv env loading + validation |
| `pkg/accurate/` | Accurate Online client, providers, session, adapters |
| `web/` | SolidJS + Vite + Tailwind v4 SPA (pnpm workspace) |

## Development Commands

Backend (Makefile):
```bash
make dev          # go run ./cmd/barcode-server (backend on :8080)
make server       # build binary to bin/barcode-server (with version ldflags)
make test         # go test -v ./...
make docker       # docker build -t warehouse-barcode .
make clean
```

Frontend (`web/`):
```bash
pnpm install
pnpm dev          # Vite on :3000, proxies /api /auth /health → localhost:8080
pnpm build        # → web/dist
```

Full single-binary build (embeds SPA):
```bash
make embed        # builds web, stages dist into cmd/barcode-server/dist, compiles with -tags embed_frontend
```

No lint/vet/fmt Makefile targets and no `.golangci.yml`; use `go vet ./...` / `go fmt ./...` directly. No CI workflows exist.

## Code Conventions & Common Patterns

- **Errors**: sentinel errors (`storage.ErrNotFound`, `accurate.ErrSessionExpired`, `accurate.ErrUntrustedHost`, `auth.ErrMissingJWTSecret`) checked with `errors.Is`; map to Huma errors (`huma.Error404NotFound`, etc.) in handlers. Official provider scrubs secrets from errors via `sanitizeError()`.
- **JSON tags**: `snake_case` for persisted models (`paper_width_mm`), `camelCase` for Accurate-upstream payload models (`itemId`), Huma tags `path:"id" query:"q" default:"1"`.
- **Enums**: type-prefixed constants (`RoleSuperadmin`, `ProviderWeb`, `SourceReceiveItem`) defined in `pkg/model/vocabulary.go` and `pkg/accurate/vocabulary.go`.
- **Concurrency**: shared state guarded by `sync.RWMutex` (session manager, adapter registry, cookie jar, challenge store). Auth claims propagate through `context.Context` (`authClaimsKey`), read via `claimsFromContext`.
- **Config**: env-only (`.env` via godotenv + Viper `AutomaticEnv`). See `.env.example`. `JWT_SECRET` auto-generates when empty; `ACCURATE_ENCRYPTION_KEY` is a stable 32-byte hex key required for encrypted stored credentials/session data.
- **DB schema**: plural `snake_case` tables; singleton system tables enforce `id INTEGER PRIMARY KEY CHECK (id = 1)`. Accurate sessions are persisted per account; legacy `accurate_session` is unused.

## Important Files

- `cmd/barcode-server/main.go` — bootstrap: config → encrypted SQLite storage → account provider pool → API server; SIGINT/SIGTERM graceful shutdown
- `pkg/api/server.go` — middleware, role enforcement, route wiring, static SPA serving
- `pkg/accurate/provider.go` — `Provider` interface, `FlexFloat` (tolerant numeric unmarshal), normalized domain types
- `pkg/accurate/session/session.go` — session lifecycle + SQLite persistence
- `pkg/storage/storage.go` — migrations + CRUD
- `web/src/stores/barcodeConfig.ts` — built-in sticker/layout presets
- `web/src/auth.tsx` — dev auth bypass / session context

## Runtime/Tooling Preferences

- **Backend**: Go 1.27.2, pure-Go only (`modernc.org/sqlite`), never introduce CGO dependencies.
- **Frontend**: Node.js 22+, **pnpm 12+** (enforced via `packageManager` in `web/package.json`; use `corepack enable pnpm`). Bun/npm are not used.
- **Docker**: 3-stage build (node:22-alpine → golang:1.27-alpine with `-tags embed_frontend` → alpine:3.21 runtime), data volume at `/data/barcode.db`.

## Testing & QA

- Go stdlib `testing` only — no testify/ginkgo; assertions via `t.Fatalf`/`t.Errorf` and `t.Run` subtests.
- Blackbox convention: test files use package `<pkg>_test`.
- All tests are hermetic: `httptest.NewServer` mock HTTP servers and `t.TempDir()` SQLite DBs; no external services, no build tags for unit vs integration.
- Run: `make test`, or targeted `go test -v ./pkg/accurate/...`, `go test -v -run TestName ./pkg/api`.
- Test seams: `Client.SetIdPHost()` redirects the Accurate IdP to a local mock (localhost/127.0.0.1 are trusted for non-HTTPS); official provider functional options inject base URL, HTTP client, HMAC secret, and deterministic clock.
- No coverage config; `go test -coverprofile=coverage.out ./...` works. Frontend has no test runner.
