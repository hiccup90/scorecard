package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hiccup90/scorecard/internal/migrate"
	_ "github.com/mattn/go-sqlite3"
)

// DB is the SQLite handle with timezone helpers.
type DB struct {
	*sql.DB
	Loc *time.Location
}

func Open(path string, loc *time.Location) (*DB, error) {
	if loc == nil {
		loc = time.Local
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir %s: %w (check permissions)", dir, err)
	}
	// Probe writability early (SQLite WAL needs create rights in the directory).
	if err := ensureWritableDir(dir); err != nil {
		return nil, err
	}

	// SQLite: foreign keys, busy timeout, WAL applied in migrate.Up
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	// Single writer — safe for embedded SQLite family app
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	db.SetMaxIdleConns(1)

	if err := migrate.Up(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w (if readonly: chown the data directory so the process can write)", path, err)
	}

	wrapped := &DB{DB: db, Loc: loc}
	if err := wrapped.seed(); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}
	return wrapped, nil
}

func (db *DB) seed() error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := db.Exec(`INSERT INTO users (name, role) VALUES ('孩子','child'), ('家长','parent')`); err != nil {
			return err
		}
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM activities`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	defaults := []struct {
		label, mode, icon, color, category string
		points, order                      int
	}{
		{"写作业", "quality", "book", "#2E7D32", "语文", 2, 1},
		{"阅读", "duration", "read", "#1565C0", "语文", 1, 2},
		{"练字", "quality", "pen", "#EF6C00", "语文", 1, 3},
		{"口算", "quality", "math", "#AD1457", "数学", 1, 4},
		{"复习", "quality", "note", "#6A1B9A", "数学", 1, 5},
		{"英语朗读", "quality", "voice", "#00838F", "英语", 1, 6},
		{"背单词", "quality", "letters", "#5D4037", "英语", 1, 7},
		{"运动", "duration", "run", "#D84315", "生活", 1, 8},
		{"收书包", "default", "bag", "#455A64", "生活", 2, 9},
		{"家务", "default", "home", "#558B2F", "生活", 3, 10},
		{"练琴", "duration", "music", "#827717", "才艺", 1, 11},
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO activities (label, base_points, score_mode, icon, color, category, sort_order) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, row := range defaults {
		if _, err := stmt.Exec(row.label, row.points, row.mode, row.icon, row.color, row.category, row.order); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) Today() string {
	return time.Now().In(db.Loc).Format("2006-01-02")
}

func (db *DB) Now() time.Time {
	return time.Now().In(db.Loc)
}

func (db *DB) Ping() error {
	return db.DB.Ping()
}

func ensureWritableDir(dir string) error {
	probe := filepath.Join(dir, ".scorecard-write-test")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("database directory not writable: %s (%w). Fix: chown/chmod the directory (Docker: sudo chown -R 10001:10001 ./data or rebuild with entrypoint)", dir, err)
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return nil
}
