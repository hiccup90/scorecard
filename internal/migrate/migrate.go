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
	// Upgrade older schema_migrations (version-only) if needed.
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
		// If core tables already exist from pre-versioned bootstrap, mark v1 applied without re-exec.
		if version == 1 {
			var tables int
			_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&tables)
			if tables > 0 {
				if _, err := db.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?,?)`, version, path.Base(name)); err != nil {
					return err
				}
				continue
			}
		}
		body, err := sqlFS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?,?)`, version, path.Base(name)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
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
