export const CAT_ORDER = ['语文', '数学', '英语', '科学', '生活', '才艺'];

export function today() {
  // local calendar date (avoid UTC shift)
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

export function offsetDate(days) {
  const d = new Date();
  d.setDate(d.getDate() + days);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

export function asArray(value) {
  return Array.isArray(value) ? value : [];
}

export function signed(n) {
  return n > 0 ? `+${n}` : String(n);
}

export function status(s) {
  return {
    pending: '待审核',
    approved: '已通过',
    rejected: '已驳回',
    reversed: '已撤回',
    fulfilled: '已完成',
  }[s] || s;
}

export function sourceLabel(s) {
  return {
    checkin: '打卡',
    checkin_reversal: '撤回',
    redemption: '兑换',
    adjustment: '调分',
  }[s] || s;
}

export function scoreText(a) {
  return a.score_mode === 'quality'
    ? `基础分 ${a.base_points} · 质量`
    : a.score_mode === 'duration'
      ? `基础分 ${a.base_points} · 时长`
      : `基础分 ${a.base_points}`;
}

export function formatTime(value) {
  if (!value) return '';
  // SQLite CURRENT_TIMESTAMP is UTC-ish; display in local
  const normalized = value.includes('T') ? value : value.replace(' ', 'T');
  const hasZone = /Z$|[+-]\d{2}:?\d{2}$/.test(normalized);
  return new Date(hasZone ? normalized : `${normalized}Z`).toLocaleString('zh-CN');
}

export function formatDateLabel(value) {
  return value === today() ? '今日' : value;
}

export function tabLabel(k) {
  return { review: '审核台', activities: '打卡项', rewards: '奖励', ledger: '积分流水' }[k];
}

export function sortCategories(categories) {
  return CAT_ORDER.filter((c) => categories.includes(c)).concat(categories.filter((c) => !CAT_ORDER.includes(c)));
}

export function emptyActivity() {
  return {
    label: '',
    base_points: 1,
    score_mode: 'default',
    icon: 'star',
    color: '#3B82F6',
    category: '生活',
    sort_order: 0,
    enabled: true,
  };
}

export function emptyReward() {
  return {
    name: '',
    cost: 10,
    description: '',
    stock: -1,
    auto_approve: true,
    enabled: true,
  };
}
