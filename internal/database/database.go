package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	wrapped := &DB{DB: db}
	if err := wrapped.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := wrapped.seed(); err != nil {
		db.Close()
		return nil, err
	}
	return wrapped, nil
}

func (db *DB) migrate() error {
	stmts := []string{
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('child','parent')),
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS activities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			base_points INTEGER NOT NULL DEFAULT 0,
			score_mode TEXT NOT NULL DEFAULT 'default' CHECK (score_mode IN ('default','quality','duration')),
			icon TEXT NOT NULL DEFAULT '*',
			color TEXT NOT NULL DEFAULT '#3B82F6',
			category TEXT NOT NULL DEFAULT '生活',
			sort_order INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS checkins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			activity_id INTEGER NOT NULL REFERENCES activities(id),
			activity_date TEXT NOT NULL,
			submitted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','reversed')),
			source TEXT NOT NULL DEFAULT 'normal' CHECK (source IN ('normal','makeup','parent_created')),
			base_points INTEGER NOT NULL DEFAULT 0,
			score_mode TEXT NOT NULL DEFAULT 'default',
			review_level TEXT,
			review_minutes INTEGER,
			awarded_points INTEGER NOT NULL DEFAULT 0,
			streak_bonus INTEGER NOT NULL DEFAULT 0,
			counts_for_streak INTEGER NOT NULL DEFAULT 1,
			review_note TEXT,
			reviewed_at TEXT,
			reviewed_by INTEGER REFERENCES users(id),
			reversed_at TEXT,
			reversed_by INTEGER REFERENCES users(id),
			reverse_reason TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS point_transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			change INTEGER NOT NULL,
			reason TEXT NOT NULL,
			source_type TEXT NOT NULL,
			source_id INTEGER,
			created_by INTEGER REFERENCES users(id),
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			reverses_transaction_id INTEGER REFERENCES point_transactions(id)
		)`,
		`CREATE TABLE IF NOT EXISTS streaks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			activity_id INTEGER NOT NULL REFERENCES activities(id),
			streak_days INTEGER NOT NULL DEFAULT 0,
			last_date TEXT NOT NULL,
			UNIQUE(user_id, activity_id)
		)`,
		`CREATE TABLE IF NOT EXISTS rewards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			cost INTEGER NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			stock INTEGER NOT NULL DEFAULT -1,
			auto_approve INTEGER NOT NULL DEFAULT 1,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS redemptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			reward_id INTEGER NOT NULL REFERENCES rewards(id),
			cost_at_time INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','fulfilled','rejected','reversed')),
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			reviewed_at TEXT,
			reviewed_by INTEGER REFERENCES users(id),
			review_note TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor_user_id INTEGER REFERENCES users(id),
			action TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id INTEGER,
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
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
	if count == 0 {
		defaults := []struct {
			label, mode, icon, color, category string
			points, order                 int
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
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func Today() string {
	return time.Now().Format("2006-01-02")
}
