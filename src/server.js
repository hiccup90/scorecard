import express from 'express';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';
import db from './db/database.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const app = express();

app.use(express.json());
app.use(express.static(join(__dirname, '../public')));

// ─── 工具 ───────────────────────────────────────────────────────────
const getBalance = (userId) => {
  const row = db.prepare(
    'SELECT COALESCE(SUM(change), 0) as total FROM point_log WHERE user_id = ?'
  ).get(userId);
  return row.total;
};

const todayStart = () => {
  return new Date().toISOString().slice(0, 10) + ' 00:00:00';
};

// ─── 积分项 ────────────────────────────────────────────────────────
app.get('/api/point-items', (req, res) => {
  const items = db.prepare(
    'SELECT * FROM point_items WHERE enabled = 1 ORDER BY sort_order'
  ).all();
  res.json(items);
});

app.post('/api/point-items', (req, res) => {
  const { label, points, icon, color } = req.body;
  const max = db.prepare('SELECT MAX(sort_order) as m FROM point_items').get();
  const result = db.prepare(
    'INSERT INTO point_items (label, points, icon, color, sort_order) VALUES (?,?,?,?,?)'
  ).run(label, points, icon || '✨', color || '#6C63FF', (max?.m ?? 0) + 1);
  res.json({ id: result.lastInsertRowid });
});

app.put('/api/point-items/:id', (req, res) => {
  const { label, points, icon, color, enabled, sort_order } = req.body;
  db.prepare(
    'UPDATE point_items SET label=?, points=?, icon=?, color=?, enabled=?, sort_order=? WHERE id=?'
  ).run(label, points, icon, color, enabled ? 1 : 0, sort_order, req.params.id);
  res.json({ ok: true });
});

app.delete('/api/point-items/:id', (req, res) => {
  db.prepare('DELETE FROM point_items WHERE id = ?').run(req.params.id);
  res.json({ ok: true });
});

// ─── 打卡加分 ──────────────────────────────────────────────────────
app.post('/api/clock', (req, res) => {
  const { user_id, item_id } = req.body;
  if (!user_id || !item_id) return res.status(400).json({ error: '缺少参数' });

  const item = db.prepare('SELECT * FROM point_items WHERE id = ?').get(item_id);
  if (!item) return res.status(404).json({ error: '积分项不存在' });

  db.prepare(
    'INSERT INTO point_log (user_id, item_id, change, reason, created_by) VALUES (?,?,?,?,?)'
  ).run(user_id, item_id, item.points, item.label, user_id);

  res.json({
    ok: true,
    points_added: item.points,
    new_balance: getBalance(user_id),
  });
});

// ─── 手工调分 ──────────────────────────────────────────────────────
app.post('/api/adjust', (req, res) => {
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

// ─── 奖励列表 ──────────────────────────────────────────────────────
app.get('/api/rewards', (req, res) => {
  const rewards = db.prepare(
    'SELECT * FROM rewards WHERE enabled = 1 ORDER BY cost'
  ).all();
  res.json(rewards);
});

app.post('/api/rewards', (req, res) => {
  const { name, cost, description, stock, auto_approve } = req.body;
  const result = db.prepare(
    'INSERT INTO rewards (name, cost, description, stock, auto_approve) VALUES (?,?,?,?,?)'
  ).run(name, cost, description || '', stock ?? -1, auto_approve ? 1 : 0);
  res.json({ id: result.lastInsertRowid });
});

app.put('/api/rewards/:id', (req, res) => {
  const { name, cost, description, stock, auto_approve, enabled } = req.body;
  db.prepare(
    'UPDATE rewards SET name=?, cost=?, description=?, stock=?, auto_approve=?, enabled=? WHERE id=?'
  ).run(name, cost, description, stock ?? -1, auto_approve ? 1 : 0, enabled ? 1 : 0, req.params.id);
  res.json({ ok: true });
});

app.delete('/api/rewards/:id', (req, res) => {
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
    // 需要审批
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
app.get('/api/redemptions', (req, res) => {
  const rows = db.prepare(`
    SELECT rr.*, r.name as reward_name, u.name as user_name
    FROM redemption_requests rr
    JOIN rewards r ON rr.reward_id = r.id
    JOIN users u ON rr.user_id = u.id
    ORDER BY rr.created_at DESC
  `).all();
  res.json(rows);
});

app.post('/api/redemptions/:id/approve', (req, res) => {
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

app.post('/api/redemptions/:id/reject', (req, res) => {
  db.prepare(
    'UPDATE redemption_requests SET status=? WHERE id=?'
  ).run('rejected', req.params.id);
  res.json({ ok: true });
});

// ─── 用户 ──────────────────────────────────────────────────────────
app.get('/api/users', (req, res) => {
  res.json(db.prepare('SELECT * FROM users').all());
});

app.post('/api/users', (req, res) => {
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
  const today = todayStart();

  const todayRows = db.prepare(
    'SELECT SUM(change) as total FROM point_log WHERE user_id = ? AND created_at >= ?'
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

  res.json({
    balance,
    today_total: todayRows.total || 0,
    top_item: topItem || null,
  });
});

// SPA fallback
app.get('*', (req, res) => {
  res.sendFile(join(__dirname, '../public/index.html'));
});

const PORT = process.env.PORT || 3003;
app.listen(PORT, () => {
  console.log(`Scorecard running on http://localhost:${PORT}`);
});
