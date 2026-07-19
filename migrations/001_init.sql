PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('child','parent')),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS activities (
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
);

CREATE TABLE IF NOT EXISTS checkins (
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
);

CREATE TABLE IF NOT EXISTS point_transactions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id),
	change INTEGER NOT NULL,
	reason TEXT NOT NULL,
	source_type TEXT NOT NULL,
	source_id INTEGER,
	created_by INTEGER REFERENCES users(id),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	reverses_transaction_id INTEGER REFERENCES point_transactions(id)
);

CREATE TABLE IF NOT EXISTS streaks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id),
	activity_id INTEGER NOT NULL REFERENCES activities(id),
	streak_days INTEGER NOT NULL DEFAULT 0,
	last_date TEXT NOT NULL,
	UNIQUE(user_id, activity_id)
);

CREATE TABLE IF NOT EXISTS rewards (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	cost INTEGER NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	stock INTEGER NOT NULL DEFAULT -1,
	auto_approve INTEGER NOT NULL DEFAULT 1,
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS redemptions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id),
	reward_id INTEGER NOT NULL REFERENCES rewards(id),
	cost_at_time INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','fulfilled','rejected','reversed')),
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	reviewed_at TEXT,
	reviewed_by INTEGER REFERENCES users(id),
	review_note TEXT
);

CREATE TABLE IF NOT EXISTS audit_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	actor_user_id INTEGER REFERENCES users(id),
	action TEXT NOT NULL,
	entity_type TEXT NOT NULL,
	entity_id INTEGER,
	detail TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	token TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id),
	role TEXT NOT NULL CHECK (role IN ('child','parent')),
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_checkins_user_status ON checkins(user_id, status);
CREATE INDEX IF NOT EXISTS idx_checkins_activity_date ON checkins(user_id, activity_id, activity_date);
CREATE INDEX IF NOT EXISTS idx_tx_user_created ON point_transactions(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_redemptions_status ON redemptions(status);
