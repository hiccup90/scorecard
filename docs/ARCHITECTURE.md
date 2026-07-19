# Architecture

## Stack

- **API**: Go `net/http`, layered packages
- **DB**: **SQLite** (single-file, WAL, `CGO` + `mattn/go-sqlite3`) — suitable for single-family self-host
- **Web**: React + Vite SPA (child + parent in one app)
- **Mobile**: Android WebView shell

## Package layout

```text
cmd/scorecard/           process entry, graceful shutdown
internal/config/         env config
internal/database/       SQLite open + seed
internal/migrate/        embedded SQL migrations (sql/*.sql)
internal/domain/         pure domain types & scoring rules
internal/server/         HTTP handlers (transport)
internal/platform/       middleware (request id, slog, recover, headers)
web/src/api/             frontend API client
web/src/lib/             format helpers
web/src/pages/           (UI modules can grow here)
migrations/              human-readable SQL mirror of embed
```

## Data model (ledger-first)

- Balance = `SUM(point_transactions.change)` — never a mutable balance column.
- Checkin workflow: `pending → approved|rejected`, `approved → reversed` (compensating txn).
- Sessions stored in SQLite `sessions` table (survives restart).

## Why SQLite

- Zero external DB ops for home deployment
- Single writer + `MaxOpenConns(1)` matches app concurrency
- Easy backup: copy `scorecard.db` (+ WAL if present)

If multi-tenant / high concurrency is needed later, swap the store implementation; keep domain + HTTP stable.

## Observability

- JSON logs via `log/slog`
- `X-Request-ID` on every response
- `/healthz`, `/readyz`, `/api/v1/version`

## Security baseline

- PIN auth → bearer token header `X-Auth-Token`
- No trust of body `user_id` / `parent_id`
- Path-safe static file serving
- Production rejects default PIN unless `ALLOW_DEFAULT_PIN=1`
- Non-root container user
