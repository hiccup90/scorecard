#!/bin/sh
set -eu

DB_PATH="${DB_PATH:-/app/data/scorecard.db}"
DATA_DIR="$(dirname "$DB_PATH")"

mkdir -p "$DATA_DIR"

# If started as root (common with volume mounts), fix ownership then drop privileges.
if [ "$(id -u)" = "0" ]; then
  chown -R appuser:appuser "$DATA_DIR" 2>/dev/null || true
  # Also ensure parent is traversable
  chmod u+rwx "$DATA_DIR" 2>/dev/null || true
  if command -v su-exec >/dev/null 2>&1; then
    exec su-exec appuser /app/scorecard "$@"
  fi
  if command -v gosu >/dev/null 2>&1; then
    exec gosu appuser /app/scorecard "$@"
  fi
  # fallback: run as root if no su-exec (not ideal, but boots)
  exec /app/scorecard "$@"
fi

# Non-root: fail fast with a clear message if the data dir is not writable.
if [ ! -w "$DATA_DIR" ]; then
  echo "scorecard: data directory is not writable: $DATA_DIR" >&2
  echo "  fix: sudo chown -R \$(id -u):\$(id -g) ./data" >&2
  echo "  or:  sudo chown -R 10001:10001 ./data   # container appuser" >&2
  exit 1
fi

exec /app/scorecard "$@"
