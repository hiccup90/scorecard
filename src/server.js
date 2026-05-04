import express from 'express';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';
import db from './db/database.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const app = express();

app.use(express.json());
app.use(express.static(join(__dirname, '../public')));

// ─── 认证 ─────────────────────────────────────────────────────────
const ADMIN_PIN = process.env.ADMIN_PIN || '1234';
const CHILD_PIN = process.env.CHILD_PIN || ADMIN_PIN; // 默认同管理员 PIN
const authTokens = new Map(); // token -> expiresAt
const TOKEN_TTL = 24 * 60 * 60 * 1000; // 24h

function auth(req, res, next) {
  const token = req.headers['x-auth-token'];
  if (!token || !authTokens.has(token) || Date.now() > authTokens.get(token)) {
    authTokens.delete(token);
    return res.status(401).json({ error: '需要登录' });
  }
  next();
}

// 每小时清理过期 token
setInterval(() => {
  const now = Date.now();
  for (const [t, exp] of authTokens) { if (now > exp) authTokens.delete(t); }
}, 3600000);

app.post('/api/auth/login', (req, res) => {
  const { pin } = req.body;
  if (pin !== ADMIN_PIN) return res.status(403).json({ error: 'PIN 码错误' });
  const token = Math.random().toString(36).slice(2) + Math.random().toString(36).slice(2) + Date.now().toString(36);
  authTokens.set(token, Date.now() + TOKEN_TTL);
  res.json({ ok: true, token });
});

app.post('/api/auth/verify', auth, (req, res) => {
  res.json({ ok: true });
});

// 孩子端一次性验证（不签 token，只校验 PIN）
app.post('/api/auth/verify-child', (req, res) => {
  const { pin } = req.body;
  if (pin !== CHILD_PIN) return res.status(403).json({ error: 'PIN 码错误' });
  res.json({ ok: true });
});

// ─── 工具 ───────────────────────────────────────────────────────────
const getBalance = (userId) => {
  const row = db.prepare(
    'SELECT COALESCE(SUM(change), 0) as total FROM point_log WHERE user_id = ?'
  ).get(userId);
  return row.total;
};

const todayKey = () => new Date().toISOString().slice(0, 10);

const addHours = (date, hours) => {
  const d = new Date(date);
  d.setHours(d.getHours() + hours);
  return d.toISOString();
};

// ─── 连续打卡计算 ─────────────────────────────────────────────────
function calcStreak(userId, itemId) {
  const today = todayKey();
  const streak = db.prepare(
    'SELECT * FROM streaks WHERE user_id = ? AND item_id = ?'
  ).get(userId, itemId);

  if (!streak) {
    // 首次打卡，连续 1 天
    db.prepare(
      'INSERT INTO streaks (user_id, item_id, streak_days, last_date) VALUES (?,?,1,?)'
    ).run(userId, itemId, today);
    return { days: 1, bonus: 0 };
  }

  const lastDate = streak.last_date;
  if (lastDate === today) {
    // 今天已打过，返回当前连续天数（不应该到这里，因为有去重）
    return { days: streak.streak_days, bonus: 0 };
  }

  // 判断是否连续：昨天打了 = 连续，否则断签重来
  const yesterday = new Date();
  yesterday.setDate(yesterday.getDate() - 1);
  const yesterdayKey = yesterday.toISOString().slice(0, 10);

  let newDays;
  if (lastDate === yesterdayKey) {
    newDays = streak.streak_days + 1;
  } else {
    newDays = 1; // 断签重来
  }

  db.prepare(
    'UPDATE streaks SET streak_days = ?, last_date = ? WHERE user_id = ? AND item_id = ?'
  ).run(newDays, today, userId, itemId);

  // 额外加分：连续第2天起，每天+1，封顶+6（连续7天+）
  const bonus = Math.min(Math.max(newDays - 1, 0), 6);
  return { days: newDays, bonus };
}

// 回退连续打卡（驳回/撤回时用）
function revertStreak(userId, itemId) {
  const streak = db.prepare(
    'SELECT * FROM streaks WHERE user_id = ? AND item_id = ?'
  ).get(userId, itemId);
  if (!streak) return;

  if (streak.streak_days <= 1) {
    db.prepare('DELETE FROM streaks WHERE user_id = ? AND item_id = ?').run(userId, itemId);
    return;
  }

  // 找到上一个 confirmed 打卡日期
  const prev = db.prepare(`
    SELECT date(created_at) as prev_date 
    FROM clock_requests 
    WHERE user_id = ? AND item_id = ? AND status = 'confirmed'
    ORDER BY created_at DESC LIMIT 1
  `).get(userId, itemId);

  if (prev) {
    db.prepare(
      'UPDATE streaks SET streak_days = streak_days - 1, last_date = ? WHERE user_id = ? AND item_id = ?'
    ).run(prev.prev_date, userId, itemId);
  } else {
    db.prepare('DELETE FROM streaks WHERE user_id = ? AND item_id = ?').run(userId, itemId);
  }
}

// ─── 积分项（带分类） ─────────────────────────────────────────────
app.get('/api/point-items', (req, res) => {
  const items = db.prepare(
    'SELECT * FROM point_items WHERE enabled = 1 ORDER BY sort_order'
  ).all();
  res.json(items);
});

app.get('/api/categories', (req, res) => {
  const cats = db.prepare(
    "SELECT category FROM point_items WHERE enabled = 1 GROUP BY category ORDER BY MIN(sort_order)"
  ).all();
  res.json(cats.map(c => c.category));
});

app.post('/api/point-items', auth, (req, res) => {
  const { label, points, icon, color, category } = req.body;
  const max = db.prepare('SELECT MAX(sort_order) as m FROM point_items').get();
  const result = db.prepare(
    'INSERT INTO point_items (label, points, icon, color, category, sort_order) VALUES (?,?,?,?,?,?)'
  ).run(label, points, icon || '✨', color || '#6C63FF', category || '生活', (max?.m ?? 0) + 1);
  res.json({ id: result.lastInsertRowid });
});

app.put('/api/point-items/:id', auth, (req, res) => {
  const { label, points, icon, color, category, enabled, sort_order } = req.body;
  db.prepare(
    'UPDATE point_items SET label=?, points=?, icon=?, color=?, category=?, enabled=?, sort_order=? WHERE id=?'
  ).run(label, points, icon, color, category || '生活', enabled ? 1 : 0, sort_order, req.params.id);
  res.json({ ok: true });
});

app.delete('/api/point-items/:id', auth, (req, res) => {
  db.prepare('DELETE FROM point_items WHERE id = ?').run(req.params.id);
  res.json({ ok: true });
});

// 移动打卡项排序（上移/下移）— 同分类内交换后重排序号
app.post('/api/point-items/:id/move', auth, (req, res) => {
  const { direction } = req.body;
  const item = db.prepare('SELECT id, category FROM point_items WHERE id = ?').get(req.params.id);
  if (!item) return res.status(404).json({ error: '不存在' });

  const catItems = db.prepare('SELECT id FROM point_items WHERE category = ? ORDER BY sort_order, id').all(item.category);
  const idx = catItems.findIndex(r => r.id == req.params.id);
  const swapIdx = direction === 'up' ? idx - 1 : idx + 1;
  if (swapIdx < 0 || swapIdx >= catItems.length) return res.json({ ok: true, moved: false });

  [catItems[idx], catItems[swapIdx]] = [catItems[swapIdx], catItems[idx]];

  db.transaction(() => {
    const upd = db.prepare('UPDATE point_items SET sort_order = ? WHERE id = ?');
    catItems.forEach((r, i) => upd.run(i, r.id));
  })();

  res.json({ ok: true, moved: true });
});

// ─── 打卡（即时满足 + 事后审核） ──────────────────────────────────
app.post('/api/clock', (req, res) => {
  const { user_id, item_id } = req.body;
  if (!user_id || !item_id) return res.status(400).json({ error: '缺少参数' });

  const item = db.prepare('SELECT * FROM point_items WHERE id = ?').get(item_id);
  if (!item) return res.status(404).json({ error: '积分项不存在' });

  // 今天该用户对该积分项是否已打卡（confirmed 或 pending 都算）
  const today = todayKey();
  const todayDone = db.prepare(
    `SELECT id FROM clock_requests WHERE user_id = ? AND item_id = ? 
     AND date(created_at) = ? AND status != 'reversed'`
  ).get(user_id, item_id, today);
  if (todayDone) return res.status(400).json({ error: '今天已经打过卡了' });

  // 计算连续打卡 + 立即加分 + 创建待审记录（同一事务）
  let totalPoints, streakResult;
  db.transaction(() => {
    streakResult = calcStreak(user_id, item_id);
    totalPoints = item.points + streakResult.bonus;

    // 写入积分流水（立即生效）
    const reason = streakResult.bonus > 0
      ? `${item.label}（连续${streakResult.days}天+${streakResult.bonus}奖励）`
      : item.label;
    db.prepare(
      'INSERT INTO point_log (user_id, item_id, change, reason, created_by) VALUES (?,?,?,?,?)'
    ).run(user_id, item_id, totalPoints, reason, user_id);

    // 创建打卡记录（24h 内家长可撤回）
    db.prepare(
      `INSERT INTO clock_requests 
       (user_id, item_id, points_at_time, streak_bonus, status, auto_approved, expires_at)
       VALUES (?,?,?,?,?,?,?)`
    ).run(user_id, item_id, totalPoints, streakResult.bonus, 'pending', 0, addHours(new Date(), 24));
  })();

  res.json({
    ok: true,
    points_added: totalPoints,
    streak: streakResult.days,
    streak_bonus: streakResult.bonus,
    new_balance: getBalance(user_id),
  });
});

// ─── 打卡记录（家长端） ──────────────────────────────────────────
app.get('/api/clock-requests', auth, (req, res) => {
  const rows = db.prepare(`
    SELECT cr.*, u.name as user_name, pi.label as item_label, pi.icon, pi.color, pi.category
    FROM clock_requests cr
    JOIN users u ON cr.user_id = u.id
    JOIN point_items pi ON cr.item_id = pi.id
    ORDER BY cr.created_at DESC
  `).all();
  res.json(rows);
});

// 家长撤回打卡（24h 内可撤回）
app.post('/api/clock-requests/:id/reverse', auth, (req, res) => {
  const row = db.prepare('SELECT * FROM clock_requests WHERE id = ?').get(req.params.id);
  if (!row) return res.status(404).json({ error: '不存在' });
  if (row.status === 'reversed') return res.status(400).json({ error: '已经撤回过了' });

  const { parent_id, reason } = req.body;

  db.transaction(() => {
    // 扣回积分
    db.prepare(
      'INSERT INTO point_log (user_id, item_id, change, reason, created_by) VALUES (?,?,?,?,?)'
    ).run(row.user_id, row.item_id, -row.points_at_time, `撤回打卡：${reason || '家长撤回'}`, parent_id);

    // 回退连续打卡
    revertStreak(row.user_id, row.item_id);

    // 更新打卡记录状态
    db.prepare(
      `UPDATE clock_requests SET status='reversed', reversed_at=datetime('now'), 
       reversed_by=?, reverse_reason=? WHERE id=?`
    ).run(parent_id, reason || '家长撤回', row.id);
  })();

  res.json({ ok: true, new_balance: getBalance(row.user_id) });
});

// 家长通过打卡审核
app.post('/api/clock-requests/:id/approve', auth, (req, res) => {
  const row = db.prepare('SELECT * FROM clock_requests WHERE id = ?').get(req.params.id);
  if (!row) return res.status(404).json({ error: '不存在' });
  if (row.status === 'confirmed') return res.status(400).json({ error: '已通过' });
  if (row.status === 'reversed') return res.status(400).json({ error: '已撤回' });

  db.prepare(
    `UPDATE clock_requests SET status='confirmed', confirmed_at=datetime('now') WHERE id=?`
  ).run(req.params.id);

  res.json({ ok: true });
});

// 家长驳回打卡（撤回积分）— 仅限 pending 状态
app.post('/api/clock-requests/:id/reject', auth, (req, res) => {
  const row = db.prepare('SELECT * FROM clock_requests WHERE id = ?').get(req.params.id);
  if (!row) return res.status(404).json({ error: '不存在' });
  if (row.status !== 'pending') return res.status(400).json({ error: '只能驳回待审核的打卡' });

  const { parent_id, reason } = req.body;

  db.transaction(() => {
    db.prepare(
      'INSERT INTO point_log (user_id, item_id, change, reason, created_by) VALUES (?,?,?,?,?)'
    ).run(row.user_id, row.item_id, -row.points_at_time, `驳回打卡：${reason || '家长驳回'}`, parent_id);

    // 回退连续打卡
    revertStreak(row.user_id, row.item_id);

    db.prepare(
      `UPDATE clock_requests SET status='reversed', reversed_at=datetime('now'), 
       reversed_by=?, reverse_reason=? WHERE id=?`
    ).run(parent_id, reason || '家长驳回', row.id);
  })();

  res.json({ ok: true, new_balance: getBalance(row.user_id) });
});

// 批量通过打卡审核
app.post('/api/clock-requests/approve-all', auth, (req, res) => {
  const { parent_id } = req.body;
  const pending = db.prepare(
    "SELECT * FROM clock_requests WHERE status = 'pending' AND user_id = 1"
  ).all();

  for (const row of pending) {
    db.prepare(
      `UPDATE clock_requests SET status='confirmed', confirmed_at=datetime('now') WHERE id=?`
    ).run(row.id);
  }

  res.json({ ok: true, count: pending.length });
});

// ─── 手工调分 ──────────────────────────────────────────────────────
app.post('/api/adjust', auth, (req, res) => {
  const { user_id, change, reason, parent_id } = req.body;
  if (!user_id || change === undefined) {
    return res.status(400).json({ error: '缺少参数' });
  }
  db.prepare(
    'INSERT INTO point_log (user_id, change, reason, created_by) VALUES (?,?,?,?)'
  ).run(user_id, change, reason || '家长调整', parent_id);
  res.json({ ok: true, new_balance: getBalance(user_id) });
});

// ─── 查询余额 ──────────────────────────────────────────────────────
app.get('/api/balance/:userId', (req, res) => {
  res.json({ user_id: req.params.userId, balance: getBalance(req.params.userId) });
});

// ─── 积分流水 ──────────────────────────────────────────────────────
app.get('/api/logs/:userId', (req, res) => {
  const logs = db.prepare(`
    SELECT pl.*, pi.label as item_label, pi.icon, pi.color
    FROM point_log pl
    LEFT JOIN point_items pi ON pl.item_id = pi.id
    WHERE pl.user_id = ?
    ORDER BY pl.created_at DESC
    LIMIT 50
  `).all(req.params.userId);
  res.json(logs);
});

// ─── 连续打卡查询 ─────────────────────────────────────────────────
app.get('/api/streaks/:userId', (req, res) => {
  const streaks = db.prepare(
    'SELECT * FROM streaks WHERE user_id = ?'
  ).all(req.params.userId);
  res.json(streaks);
});

// ─── 奖励列表 ──────────────────────────────────────────────────────
app.get('/api/rewards', (req, res) => {
  const rewards = db.prepare(
    'SELECT * FROM rewards WHERE enabled = 1 ORDER BY cost'
  ).all();
  res.json(rewards);
});

app.post('/api/rewards', auth, (req, res) => {
  const { name, cost, description, stock, auto_approve } = req.body;
  const result = db.prepare(
    'INSERT INTO rewards (name, cost, description, stock, auto_approve) VALUES (?,?,?,?,?)'
  ).run(name, cost, description || '', stock ?? -1, auto_approve ? 1 : 0);
  res.json({ id: result.lastInsertRowid });
});

app.put('/api/rewards/:id', auth, (req, res) => {
  const { name, cost, description, stock, auto_approve, enabled } = req.body;
  db.prepare(
    'UPDATE rewards SET name=?, cost=?, description=?, stock=?, auto_approve=?, enabled=? WHERE id=?'
  ).run(name, cost, description, stock ?? -1, auto_approve ? 1 : 0, enabled ? 1 : 0, req.params.id);
  res.json({ ok: true });
});

app.delete('/api/rewards/:id', auth, (req, res) => {
  db.prepare('DELETE FROM rewards WHERE id = ?').run(req.params.id);
  res.json({ ok: true });
});

// ─── 兑换申请 ──────────────────────────────────────────────────────
app.post('/api/redeem', (req, res) => {
  const { user_id, reward_id } = req.body;
  if (!user_id || !reward_id) return res.status(400).json({ error: '缺少参数' });

  const reward = db.prepare('SELECT * FROM rewards WHERE id = ?').get(reward_id);
  if (!reward) return res.status(404).json({ error: '奖励不存在' });
  if (reward.stock === 0) return res.status(400).json({ error: '奖励库存不足' });

  const balance = getBalance(user_id);
  if (balance < reward.cost) {
    return res.status(400).json({ error: '积分不足', balance, cost: reward.cost });
  }

  if (reward.auto_approve) {
    db.transaction(() => {
      db.prepare(
        'INSERT INTO point_log (user_id, change, reason, created_by) VALUES (?,?,?,?)'
      ).run(user_id, -reward.cost, `兑换「${reward.name}」`, user_id);
      db.prepare(
        'INSERT INTO redemption_requests (user_id, reward_id, cost_at_time, status) VALUES (?,?,?,?)'
      ).run(user_id, reward_id, reward.cost, 'fulfilled');
      if (reward.stock > 0) {
        db.prepare('UPDATE rewards SET stock = stock - 1 WHERE id = ?').run(reward_id);
      }
    })();
  } else {
    db.prepare(
      'INSERT INTO redemption_requests (user_id, reward_id, cost_at_time, status) VALUES (?,?,?,?)'
    ).run(user_id, reward_id, reward.cost, 'pending');
  }

  res.json({
    ok: true,
    new_balance: getBalance(user_id),
    status: reward.auto_approve ? 'fulfilled' : 'pending',
  });
});

// ─── 审批兑换 ──────────────────────────────────────────────────────
app.get('/api/redemptions', auth, (req, res) => {
  const rows = db.prepare(`
    SELECT rr.*, r.name as reward_name, u.name as user_name
    FROM redemption_requests rr
    JOIN rewards r ON rr.reward_id = r.id
    JOIN users u ON rr.user_id = u.id
    ORDER BY rr.created_at DESC
  `).all();
  res.json(rows);
});

app.post('/api/redemptions/:id/approve', auth, (req, res) => {
  const row = db.prepare('SELECT * FROM redemption_requests WHERE id = ?').get(req.params.id);
  if (!row) return res.status(404).json({ error: '不存在' });
  if (row.status !== 'pending') return res.status(400).json({ error: '状态不对' });

  const reward = db.prepare('SELECT stock, name FROM rewards WHERE id = ?').get(row.reward_id);
  if (!reward) return res.status(404).json({ error: '奖励不存在' });
  if (reward.stock === 0) return res.status(400).json({ error: '奖励库存不足，无法审批' });

  const balance = getBalance(row.user_id);
  if (balance < row.cost_at_time) {
    return res.status(400).json({ error: '积分不足，无法审批' });
  }

  db.transaction(() => {
    db.prepare(
      'INSERT INTO point_log (user_id, change, reason, created_by) VALUES (?,?,?,?)'
    ).run(row.user_id, -row.cost_at_time, `兑换「${reward.name}」`, req.body.parent_id);
    db.prepare(
      'UPDATE redemption_requests SET status=?, approved_at=datetime("now"), approved_by=? WHERE id=?'
    ).run('fulfilled', req.body.parent_id, req.params.id);
    if (reward.stock > 0) {
      db.prepare('UPDATE rewards SET stock = stock - 1 WHERE id = ?').run(row.reward_id);
    }
  })();

  res.json({ ok: true, new_balance: getBalance(row.user_id) });
});

app.post('/api/redemptions/:id/reject', auth, (req, res) => {
  db.prepare(
    'UPDATE redemption_requests SET status=? WHERE id=?'
  ).run('rejected', req.params.id);
  res.json({ ok: true });
});

// ─── 用户 ──────────────────────────────────────────────────────────
app.get('/api/users', auth, (req, res) => {
  res.json(db.prepare('SELECT * FROM users').all());
});

app.post('/api/users', auth, (req, res) => {
  const { name, role, password } = req.body;
  const result = db.prepare(
    'INSERT INTO users (name, role, password) VALUES (?,?,?)'
  ).run(name, role, password || null);
  res.json({ id: result.lastInsertRowid });
});

// ─── 统计 ──────────────────────────────────────────────────────────
app.get('/api/stats/:userId', (req, res) => {
  const userId = req.params.userId;
  const balance = getBalance(userId);
  const today = todayKey();

  const todayRows = db.prepare(
    'SELECT SUM(change) as total FROM point_log WHERE user_id = ? AND date(created_at) = ?'
  ).get(userId, today);

  const topItem = db.prepare(`
    SELECT pi.label, SUM(pl.change) as total
    FROM point_log pl
    JOIN point_items pi ON pl.item_id = pi.id
    WHERE pl.user_id = ?
    GROUP BY pl.item_id
    ORDER BY total DESC
    LIMIT 1
  `).get(userId);

  // 当前最长连续打卡
  const topStreak = db.prepare(
    'SELECT MAX(streak_days) as max_streak FROM streaks WHERE user_id = ?'
  ).get(userId);

  // 待审核打卡数
  const pendingClock = db.prepare(
    "SELECT COUNT(*) as c FROM clock_requests WHERE user_id = ? AND status = 'pending'"
  ).get(userId);

  res.json({
    balance,
    today_total: todayRows.total || 0,
    top_item: topItem || null,
    max_streak: topStreak?.max_streak || 0,
    pending_clock: pendingClock.c,
  });
});

// /admin 路由 — 必须在 SPA fallback 前
app.get('/admin', (req, res) => {
  res.sendFile(join(__dirname, '../public/admin.html'));
});

// SPA fallback
app.get('*', (req, res) => {
  res.sendFile(join(__dirname, '../public/index.html'));
});

const PORT = process.env.PORT || 3003;
app.listen(PORT, () => {
  console.log(`Scorecard running on http://localhost:${PORT}`);
});
