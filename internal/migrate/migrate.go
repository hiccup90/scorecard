package migrate

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// Up applies versioned SQLite migrations embedded under sql/.
func Up(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return err
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}
	if err := ensureMigrationColumns(db); err != nil {
		return err
	}

	var names []string
	err := fs.WalkDir(sqlFS, "sql", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".sql") {
			names = append(names, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(names)

	for _, name := range names {
		version, ok := parseVersion(path.Base(name))
		if !ok {
			continue
		}
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=?`, version).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		// Legacy DBs already have core tables: still run IF NOT EXISTS SQL so new
		// tables (e.g. sessions) are created, then mark version applied.
		body, err := sqlFS.ReadFile(name)
		if err != nil {
			return err
		}
		// Skip pure no-op seed placeholder without failing.
		sqlText := strings.TrimSpace(string(body))
		if sqlText == "" || sqlText == "SELECT 1;" {
			if _, err := db.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?,?)`, version, path.Base(name)); err != nil {
				return err
			}
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(sqlText); err != nil {
			_ = tx.Rollback()
			// If objects already exist with incompatible definitions, try to continue
			// only after ensuring critical new tables exist.
			if !isBenignExistsErr(err) {
				_ = ensureSessionsTable(db)
				// re-check: if sessions now exists, mark applied for legacy upgrade path
				if hasTable(db, "sessions") && version == 1 {
					if _, err2 := db.Exec(`INSERT OR IGNORE INTO schema_migrations (version, name) VALUES (?,?)`, version, path.Base(name)); err2 != nil {
						return fmt.Errorf("migrate %s: %w (also ensure sessions: ok)", name, err)
					}
					continue
				}
				return fmt.Errorf("migrate %s: %w", name, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?,?)`, version, path.Base(name)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	// Always ensure critical tables for DBs upgraded from pre-auth versions.
	if err := ensureSessionsTable(db); err != nil {
		return err
	}
	return nil
}

func ensureSessionsTable(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('child','parent')),
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor_user_id INTEGER,
			action TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id INTEGER,
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("ensure schema: %w", err)
		}
	}
	return nil
}

func hasTable(db *sql.DB, name string) bool {
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	return n > 0
}

func isBenignExistsErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists")
}

func ensureMigrationColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(schema_migrations)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		cols[name] = true
	}
	if !cols["name"] {
		if _, err := db.Exec(`ALTER TABLE schema_migrations ADD COLUMN name TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

func parseVersion(base string) (int, bool) {
	parts := strings.SplitN(base, "_", 2)
	if len(parts) < 1 {
		return 0, false
	}
	v, err := strconv.Atoi(parts[0])
	return v, err == nil
}
