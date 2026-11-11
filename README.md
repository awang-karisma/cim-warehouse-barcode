# Warehouse Barcode

Standalone barcode label printing workstation with Accurate Online inventory and transaction document integration.

## Overview

Warehouse Barcode generates and prints barcode labels for warehouse products, purchase orders, delivery orders, and stock opname sheets. It features:

- **Custom Label Layout Designer**: Interactive drag-and-drop designer for label elements (barcode, barcode text, item name, code, price, unit, company header, separator lines, custom text).
- **Page Design Presets**: Built-in stickers (Tom & Jerry, thermal rolls) and customizable grid layouts with SQLite persistence.
- **Symbologies**: CODE128, CODE39, EAN-13, QR Code, and Data Matrix.
- **Accurate Online Integration**: Real-time item lookup, PO/DO/RI/SO transaction line item imports, multi-database support, and MFA/TOTP flow.
- **Authentication**: Optional Google OAuth with role-based access control (`guest`, `user`, `admin`, `superadmin`).

## Tech Stack

- **Backend**: Go 1.27, Fiber v3, Huma v2 (OpenAPI 3.1 & Scalar UI), SQLite (`modernc.org/sqlite` pure Go).
- **Frontend**: SolidJS, Vite, Tailwind CSS v4, Kobalte UI, JsBarcode, datamatrix-svg-ts, qrcode-generator.

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `SERVER_PORT` / `PORT` | HTTP port | `8080` |
| `DB_PATH` | Path to SQLite database file | `data/barcode.db` |
| `GOOGLE_CLIENT_ID` | Google OAuth Client ID (empty disables auth) | `""` |
| `JWT_SECRET` | Secret key for signing session tokens | Auto-generated on startup |
| `SUPERADMIN_EMAILS` | Comma-separated list of superadmin emails | `""` |
| `ACCURATE_PROVIDER` | Accurate provider: `web`, `official`, or empty | `""` |
| `ACCURATE_EMAIL` | Accurate Online account email (for `web` provider) | `""` |
| `ACCURATE_PASSWORD` | Accurate Online account password (for `web` provider) | `""` |
| `ACCURATE_API_TOKEN` | Accurate API token (for `official` provider) | `""` |
| `ACCURATE_API_SIGNATURE_SECRET` | Accurate API signature secret (for `official` provider) | `""` |

## Local Development

### Prerequisites

- Go 1.27+
- Node.js 22+
- pnpm 12+ (`corepack enable pnpm`)

### Running the Services

1. **Backend Server**:
   ```bash
   cp .env.example .env
   make dev
   ```
   Server listens on `http://localhost:8080`.
   - API Docs: `http://localhost:8080/docs`
   - OpenAPI Schema: `http://localhost:8080/openapi`

2. **Frontend Dev Server**:
   ```bash
   cd web
   pnpm install
   pnpm dev
   ```
   Vite dev server starts on `http://localhost:3000` with proxies configured for `/api`, `/auth`, and `/health`.

### Authentication Bypass (Local Dev)

Authentication is optional and bypassed by default in local environments:

- **Activation**: Leave `GOOGLE_CLIENT_ID` empty or unset in `.env` (the default in `.env.example`).
- **Backend behavior**:
  - `GET /auth/config` responds with `{"auth_enabled": false}`.
  - Fiber route middleware (`authMiddleware`) and Huma OpenAPI middleware (`humaAuthMiddleware`) skip cookie and JWT token verification.
  - Role enforcement (`requireRole`) permits all requests without claims, allowing direct access to all API and admin routes.
- **Frontend behavior**:
  - `AuthProvider` queries `GET /auth/config` on initial load.
  - When `auth_enabled` is `false`, it automatically provisions an active developer session:
    - **Name**: `Developer`
    - **Role**: `superadmin`
  - Login and pending-approval gates are bypassed, providing immediate access to the print queue, label designer, preset management, and administrative views without external OAuth credentials.
- **Enabling authentication**: Set a valid `GOOGLE_CLIENT_ID` in `.env` (along with `SUPERADMIN_EMAILS` and optional `JWT_SECRET`). The backend will enforce token verification and the frontend will require Google OAuth sign-in.

## Production Build & Embedding

To build a single self-contained binary with the SolidJS SPA embedded:

```bash
make embed
```

The output binary is placed at `bin/barcode-server`. Run it directly:

```bash
./bin/barcode-server
```

## Docker

Build and run using Docker Compose:

```bash
docker compose up -d --build
```

The service runs at `http://localhost:8080` with data stored in the `barcode-data` named volume (`/data/barcode.db`).

## API Endpoints

### System & Health
- `GET /health` — Service readiness probe
- `GET /api/v1/system` — Runtime diagnostics and database table counts
- `GET /docs` — Interactive Scalar API documentation

### Authentication & Users
- `GET /auth/config` — Public authentication config
- `POST /auth/google/callback` — Google OAuth ID token exchange
- `GET /auth/me` — Current authenticated user profile
- `POST /auth/logout` — Invalidate session
- `PUT /api/v1/user/locale` — Update language preference (`en` or `id`)
- `GET /api/v1/admin/users` — List registered users (Admin only)
- `PUT /api/v1/admin/users/{email}/role` — Update user role (Admin only)
- `DELETE /api/v1/admin/users/{email}` — Delete user (Admin only)

### Roles & Permissions (RBAC)
- `GET /api/v1/admin/roles` — List system and custom roles (`role:read`)
- `POST /api/v1/admin/roles` — Create custom role (`role:create`)
- `PUT /api/v1/admin/roles/{id}` — Update role details (`role:update`)
- `DELETE /api/v1/admin/roles/{id}` — Delete unused custom role (`role:delete`)
- `GET /api/v1/admin/roles/{id}/permissions` — Get role permission list (`role:read`)
- `PUT /api/v1/admin/roles/{id}/permissions` — Update role permissions (`role:update`)
- `GET /api/v1/admin/permissions` — List all registered system permissions (`role:read`)

### Page Designs & Presets
- `GET /api/v1/page-designs` — List custom page designs
- `POST /api/v1/page-designs` — Create custom page design
- `PUT /api/v1/page-designs/{id}` — Update custom page design
- `DELETE /api/v1/page-designs/{id}` — Delete custom page design
- `GET /api/v1/label-layout-presets` — List reusable element arrangements
- `POST /api/v1/label-layout-presets` — Create reusable element arrangement
- `PUT /api/v1/label-layout-presets/{id}` — Update reusable element arrangement
- `DELETE /api/v1/label-layout-presets/{id}` — Delete reusable element arrangement

### Accurate Online Integration
- `GET /api/v1/accurate/status` — Accurate connection status
- `POST /api/v1/accurate/login` — Sign in / submit TOTP MFA
- `POST /api/v1/accurate/logout` — Disconnect active session
- `GET /api/v1/accurate/databases` — List accessible databases
- `POST /api/v1/accurate/database` — Select active database
- `GET /api/v1/accurate/items` — Search inventory items
- `GET /api/v1/accurate/sources` — List supported document sources
- `GET /api/v1/accurate/sources/{source}/documents` — List transaction documents
- `GET /api/v1/accurate/sources/{source}/documents/{id}/items` — Fetch document line items
