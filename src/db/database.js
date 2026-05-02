import Database from 'better-sqlite3';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const db = new Database(join(__dirname, '../../data/scorecard.db'));

db.pragma('journal_mode = WAL');
db.pragma('foreign_keys = ON');

// 初始化表
db.exec(`
  CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    role       TEXT    NOT NULL CHECK (role IN ('child', 'parent')),
    password   TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE IF NOT EXISTS point_items (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    label      TEXT    NOT NULL,
    points     INTEGER NOT NULL DEFAULT 0,
    icon       TEXT    DEFAULT '✨',
    color      TEXT    DEFAULT '#6C63FF',
    sort_order INTEGER DEFAULT 0,
    enabled    INTEGER DEFAULT 1
  );

  CREATE TABLE IF NOT EXISTS point_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    item_id    INTEGER REFERENCES point_items(id),
    change     INTEGER NOT NULL,
    reason     TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER REFERENCES users(id)
  );

  CREATE TABLE IF NOT EXISTS rewards (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL,
    cost         INTEGER NOT NULL,
    description  TEXT,
    stock        INTEGER DEFAULT -1,
    auto_approve INTEGER DEFAULT 1,
    enabled      INTEGER DEFAULT 1
  );

  CREATE TABLE IF NOT EXISTS redemption_requests (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    reward_id   INTEGER NOT NULL REFERENCES rewards(id),
    cost_at_time INTEGER NOT NULL,
    status      TEXT    DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'fulfilled')),
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    approved_at DATETIME,
    approved_by INTEGER REFERENCES users(id)
  );
`);

// 默认用户（不重复插入）
const userCount = db.prepare('SELECT COUNT(*) as c FROM users').get();
if (userCount.c === 0) {
  const insertUser = db.prepare(
    'INSERT INTO users (name, role, password) VALUES (?,?,?)'
  );
  insertUser.run('孩子', 'child', null);
  insertUser.run('家长', 'parent', null);
}

// 默认积分项（不重复插入）
const itemCount = db.prepare('SELECT COUNT(*) as c FROM point_items').get();
if (itemCount.c === 0) {
  const insert = db.prepare(
    'INSERT INTO point_items (label, points, icon, color, sort_order) VALUES (?,?,?,?,?)'
  );
  const defaults = [
    ['写作业', 5, '📚', '#4CAF50', 1],
    ['阅读', 3, '📖', '#2196F3', 2],
    ['运动', 3, '🏃', '#FF9800', 3],
    ['收书包', 2, '🎒', '#9C27B0', 4],
    ['家务', 4, '🏠', '#E91E63', 5],
    ['练琴', 5, '🎹', '#00BCD4', 6],
  ];
  for (const row of defaults) insert.run(...row);
}

export default db;
