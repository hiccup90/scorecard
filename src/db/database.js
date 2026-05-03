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
    category   TEXT    DEFAULT '其他',
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

  CREATE TABLE IF NOT EXISTS clock_requests (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    item_id         INTEGER NOT NULL REFERENCES point_items(id),
    points_at_time  INTEGER NOT NULL,
    streak_bonus    INTEGER DEFAULT 0,
    status          TEXT    DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'reversed')),
    auto_approved   INTEGER DEFAULT 0,
    expires_at      DATETIME,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    confirmed_at    DATETIME,
    reversed_at     DATETIME,
    reversed_by     INTEGER REFERENCES users(id),
    reverse_reason  TEXT
  );

  CREATE TABLE IF NOT EXISTS streaks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    item_id     INTEGER NOT NULL REFERENCES point_items(id),
    streak_days INTEGER DEFAULT 0,
    last_date   TEXT    NOT NULL,
    UNIQUE(user_id, item_id)
  );
`);

// ── 迁移：给旧表加新字段 ──
function addColumnIfNotExists(table, column, type) {
  const cols = db.prepare(`PRAGMA table_info(${table})`).all();
  if (!cols.find(c => c.name === column)) {
    db.exec(`ALTER TABLE ${table} ADD COLUMN ${column} ${type}`);
  }
}

try { addColumnIfNotExists('point_items', 'category', "TEXT DEFAULT '其他'"); } catch {}
try { addColumnIfNotExists('clock_requests', 'streak_bonus', 'INTEGER DEFAULT 0'); } catch {}
try { addColumnIfNotExists('clock_requests', 'auto_approved', 'INTEGER DEFAULT 0'); } catch {}
try { addColumnIfNotExists('clock_requests', 'expires_at', 'DATETIME'); } catch {}
try { addColumnIfNotExists('clock_requests', 'confirmed_at', 'DATETIME'); } catch {}
try { addColumnIfNotExists('clock_requests', 'reversed_at', 'DATETIME'); } catch {}
try { addColumnIfNotExists('clock_requests', 'reversed_by', 'INTEGER'); } catch {}
try { addColumnIfNotExists('clock_requests', 'reverse_reason', 'TEXT'); } catch {}

// 旧表 status 约束可能不含 confirmed/reversed，迁移数据
try {
  const oldRows = db.prepare("SELECT * FROM clock_requests WHERE status = 'fulfilled'").all();
  if (oldRows.length > 0) {
    db.prepare("UPDATE clock_requests SET status = 'confirmed' WHERE status = 'fulfilled'").run();
  }
} catch {}

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
    'INSERT INTO point_items (label, points, icon, color, category, sort_order) VALUES (?,?,?,?,?,?)'
  );
  const defaults = [
    ['写作业', 5, '📚', '#4CAF50', '语文', 1],
    ['阅读', 3, '📖', '#2196F3', '语文', 2],
    ['练字', 3, '✍️', '#FF9800', '语文', 3],
    ['口算', 3, '🔢', '#E91E63', '数学', 4],
    ['复习', 3, '📝', '#9C27B0', '数学', 5],
    ['英语朗读', 3, '🗣️', '#00BCD4', '英语', 6],
    ['背单词', 3, '🔤', '#795548', '英语', 7],
    ['运动', 3, '🏃', '#FF5722', '生活', 8],
    ['收书包', 2, '🎒', '#607D8B', '生活', 9],
    ['家务', 4, '🏠', '#8BC34A', '生活', 10],
    ['练琴', 5, '🎹', '#CDDC39', '才艺', 11],
  ];
  for (const row of defaults) insert.run(...row);
} else {
  // 给旧数据补 category（如果没有分类的记录）
  const noCat = db.prepare("SELECT id FROM point_items WHERE category IS NULL OR category = '其他'").all();
  if (noCat.length > 0) {
    const mapping = {
      '写作业': '语文', '阅读': '语文',
      '运动': '生活', '收书包': '生活', '家务': '生活',
      '练琴': '才艺',
    };
    const upd = db.prepare('UPDATE point_items SET category = ? WHERE id = ?');
    const allItems = db.prepare('SELECT id, label FROM point_items').all();
    for (const item of allItems) {
      if (mapping[item.label]) {
        upd.run(mapping[item.label], item.id);
      }
    }
  }
}

export default db;
