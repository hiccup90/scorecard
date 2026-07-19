# Runbook

## Readonly database / migrate failed

`attempt to write a readonly database` is almost always **directory permissions**, not schema breakage.

SQLite needs write access to the **folder** (for WAL: `*.db-wal`, `*.db-shm`), not only the `.db` file.

**Docker compose (mounted `./data`):**

```bash
# after pull/rebuild — entrypoint should auto-chown; if still failing:
sudo chown -R 10001:10001 ./data
# or match your host user
sudo chown -R "$(id -u):$(id -g)" ./data
docker compose up -d --build
```

**Local binary:**

```bash
mkdir -p ./data
# ensure current user owns it
chown -R "$(id -u):$(id -g)" ./data
```

Old DBs from previous versions remain compatible: migrations mark v1 applied if tables already exist.

## Backup (SQLite)

Stop writes briefly if possible, then:

```bash
sqlite3 ./data/scorecard.db ".backup './data/scorecard-backup.db'"
# or cold copy when process stopped:
cp ./data/scorecard.db ./data/scorecard.db.bak
# include WAL if present
cp ./data/scorecard.db-wal ./data/scorecard.db-wal.bak 2>/dev/null || true
```

Restore: stop service, replace files, start service.

## Rotate PIN

Set new `ADMIN_PIN` / `CHILD_PIN` and restart. Existing sessions remain valid until TTL; optional:

```sql
DELETE FROM sessions;
```

## Migrations

SQL files live in `internal/migrate/sql/` (embedded) and mirrored under `migrations/`.
Add `003_xxx.sql` with next integer prefix; deploy restarts apply automatically.

## Logs

JSON to stdout (`LOG_LEVEL=info`). Correlate with `X-Request-ID` / field `request_id`.

## Health

- Liveness: `GET /healthz`
- Readiness: `GET /readyz` (SQLite ping)
