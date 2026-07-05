import React, { useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  Award,
  BookOpen,
  CalendarDays,
  Check,
  ChevronRight,
  ClipboardCheck,
  Clock3,
  Coins,
  Gift,
  History,
  Home,
  ListChecks,
  Loader2,
  Lock,
  PenLine,
  Plus,
  RotateCcw,
  Settings,
  ShieldCheck,
  Sparkles,
  Target,
  Timer,
  Trophy,
  X,
} from 'lucide-react';
import './styles.css';

const API = '/api/v1';
const CHILD_ID = 1;
const PARENT_ID = 2;
const CAT_ORDER = ['语文', '数学', '英语', '科学', '生活', '才艺'];

async function request(path, options = {}) {
  const token = sessionStorage.getItem('scorecard-token') || '';
  const res = await fetch(API + path, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { 'X-Auth-Token': token } : {}),
    },
    ...options,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || '请求失败');
  return data;
}

function today() {
  return new Date().toISOString().slice(0, 10);
}

function offsetDate(days) {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

function asArray(value) {
  return Array.isArray(value) ? value : [];
}

function App() {
  const [mode, setMode] = useState(location.pathname.startsWith('/admin') ? 'admin' : 'child');
  useEffect(() => history.replaceState(null, '', mode === 'admin' ? '/admin' : '/'), [mode]);
  return mode === 'admin' ? <AdminApp onChild={() => setMode('child')} /> : <ChildApp onAdmin={() => setMode('admin')} />;
}

function ChildApp({ onAdmin }) {
  const [verified, setVerified] = useState(localStorage.getItem('scorecard-child-ok') === '1');
  const [pin, setPin] = useState('');
  const [err, setErr] = useState('');
  const [tab, setTab] = useState('checkin');
  const [summary, setSummary] = useState(null);
  const [activities, setActivities] = useState([]);
  const [checkins, setCheckins] = useState([]);
  const [transactions, setTransactions] = useState([]);
  const [rewards, setRewards] = useState([]);
  const [redemptions, setRedemptions] = useState([]);
  const [activityDate, setActivityDate] = useState(today());
  const [activeCat, setActiveCat] = useState('全部');
  const [toast, setToast] = useState('');
  const [loadingID, setLoadingID] = useState(null);

  async function login(e) {
    e.preventDefault();
    setErr('');
    try {
      await request('/auth/child/login', { method: 'POST', body: { pin } });
      localStorage.setItem('scorecard-child-ok', '1');
      setVerified(true);
    } catch (error) {
      setErr(error.message);
    }
  }

  async function refresh() {
    const [sum, acts, cks, txs, rws, reds] = await Promise.all([
      request('/children/1/summary'),
      request('/activities'),
      request('/checkins'),
      request('/transactions'),
      request('/rewards'),
      request('/redemptions'),
    ]);
    setSummary(sum || {});
    setActivities(asArray(acts));
    setCheckins(asArray(cks));
    setTransactions(asArray(txs));
    setRewards(asArray(rws));
    setRedemptions(asArray(reds));
  }

  useEffect(() => { if (verified) refresh().catch(e => setToast(e.message)); }, [verified]);
  useToastTimer(toast, setToast);

  async function submitCheckin(activity) {
    setLoadingID(activity.id);
    try {
      await request('/checkins', { method: 'POST', body: { user_id: CHILD_ID, activity_id: activity.id, activity_date: activityDate } });
      setToast(activityDate === today() ? '已提交，等待家长审核' : '补签已提交，等待家长审核');
      refresh();
    } catch (error) {
      setToast(error.message);
    } finally {
      setLoadingID(null);
    }
  }

  async function redeem(reward) {
    if (!confirm(`确定兑换「${reward.name}」吗？需要 ${reward.cost} 分。`)) return;
    try {
      await request('/redemptions', { method: 'POST', body: { user_id: CHILD_ID, reward_id: reward.id } });
      setToast(reward.auto_approve ? '兑换成功' : '兑换已提交');
      refresh();
    } catch (error) {
      setToast(error.message);
    }
  }

  const safeActivities = asArray(activities);
  const safeCheckins = asArray(checkins);
  const safeRewards = asArray(rewards);
  const safeRedemptions = asArray(redemptions);
  const safeTransactions = asArray(transactions);
  const categories = useMemo(() => sortCategories([...new Set(safeActivities.map(a => a.category || '其他'))]), [safeActivities]);
  const filtered = activeCat === '全部' ? safeActivities : safeActivities.filter(a => a.category === activeCat);
  const pendingCheckins = safeCheckins.filter(c => c.status === 'pending');
  const todayPending = pendingCheckins.slice(0, 3);

  if (!verified) return <Login title="欢迎打卡" icon={<ShieldCheck />} pin={pin} setPin={setPin} err={err} onSubmit={login} />;

  return <main className="child-app">
    {toast && <Toast>{toast}</Toast>}
    <section className="child-hero">
      <div className="hero-topline"><span><Sparkles />积分打卡</span><button className="ghost-icon parent-entry" onClick={onAdmin} title="家长管理"><Settings size={19} /></button></div>
      <div className="hero-label">当前积分</div>
      <div className="hero-balance">{summary?.balance ?? 0}</div>
      <div className="hero-stats">
        <MiniStat icon={<Coins />} label="今日" value={signed(summary?.today_total ?? 0)} />
        <MiniStat icon={<Clock3 />} label="待审核" value={summary?.pending_count ?? 0} />
        <MiniStat icon={<Trophy />} label="最长连续" value={`${summary?.max_streak_days ?? 0}天`} />
      </div>
    </section>

    {tab === 'checkin' && <section className="child-page">
      <div className="daily-strip">
        <div><span><Target /></span><strong>{activityDate === today() ? '今日任务' : '补签任务'}</strong><small>{filtered.length} 个项目可选</small></div>
        <div><span><Clock3 /></span><strong>{pendingCheckins.length}</strong><small>待审核</small></div>
      </div>
      <div className="date-panel">
        <div><strong>{activityDate === today() ? '今天打卡' : '补签打卡'}</strong><span>{formatDateLabel(activityDate)}</span></div>
        <div className="date-controls"><button type="button" className="secondary" onClick={() => setActivityDate(today())}>今天</button><button type="button" className="secondary" onClick={() => setActivityDate(offsetDate(-1))}>昨天</button><label><CalendarDays size={17} /><input type="date" value={activityDate} max={today()} onChange={e => setActivityDate(e.target.value)} /></label></div>
      </div>

      {todayPending.length > 0 && <div className="pending-strip">
        {todayPending.map(c => <span key={c.id}>{iconNode(c.activity_icon)} {c.activity_label}</span>)}
      </div>}

      <Segmented value={activeCat} onChange={setActiveCat} options={['全部', ...categories]} />
      {filtered.length ? <div className="activity-grid">{filtered.map(a => <button key={a.id} className="activity-card" data-category={a.category} onClick={() => submitCheckin(a)} disabled={loadingID === a.id}>
        <span className="activity-icon">{loadingID === a.id ? <Loader2 className="spin" /> : iconNode(a.icon)}</span>
        <span className="activity-category">{a.category}</span>
        <strong>{a.label}</strong>
        <small>{scoreText(a)}</small>
        <span className="mode-pill">{modeLabel(a.score_mode)}</span>
      </button>)}</div> : <EmptyState text="这个分类还没有打卡项" />}
      <SectionTitle icon={<History />} title="最近打卡" />
      <CheckinList items={safeCheckins.slice(0, 8)} />
    </section>}

    {tab === 'rewards' && <section className="child-page">
      <SectionTitle icon={<Gift />} title="积分兑换" />
      {safeRewards.length ? <div className="reward-grid">{safeRewards.map(r => <button key={r.id} className="reward-card" onClick={() => redeem(r)}>
        <span><Gift /></span><strong>{r.name}</strong><small>{r.description || (r.auto_approve ? '自动通过' : '需要审批')}</small><div className="reward-footer"><b>{r.cost} 分</b><em>{r.auto_approve ? '自动' : '审批'}</em></div>
      </button>)}</div> : <EmptyState text="还没有可兑换奖励" />}
      <SectionTitle icon={<ListChecks />} title="兑换记录" />
      <RedemptionList items={safeRedemptions} />
    </section>}

    {tab === 'records' && <section className="child-page">
      <SectionTitle icon={<History />} title="积分流水" />
      <TransactionList items={safeTransactions} />
    </section>}

    <nav className="bottom-nav">
      <button className={tab === 'checkin' ? 'active' : ''} onClick={() => setTab('checkin')}><ClipboardCheck />打卡</button>
      <button className={tab === 'rewards' ? 'active' : ''} onClick={() => setTab('rewards')}><Gift />奖励</button>
      <button className={tab === 'records' ? 'active' : ''} onClick={() => setTab('records')}><History />记录</button>
    </nav>
  </main>;
}

function AdminApp({ onChild }) {
  const [token, setToken] = useState(sessionStorage.getItem('scorecard-token') || '');
  const [pin, setPin] = useState('');
  const [err, setErr] = useState('');
  const [tab, setTab] = useState('review');
  const [summary, setSummary] = useState(null);
  const [checkins, setCheckins] = useState([]);
  const [activities, setActivities] = useState([]);
  const [rewards, setRewards] = useState([]);
  const [redemptions, setRedemptions] = useState([]);
  const [transactions, setTransactions] = useState([]);
  const [draftActivity, setDraftActivity] = useState(emptyActivity());
  const [draftReward, setDraftReward] = useState(emptyReward());
  const [adjust, setAdjust] = useState({ change: '', reason: '' });
  const [toast, setToast] = useState('');
  const [reviewing, setReviewing] = useState(null);
  const [rejecting, setRejecting] = useState(null);
  const [reversing, setReversing] = useState(null);

  async function login(e) {
    e.preventDefault();
    setErr('');
    try {
      const res = await request('/auth/parent/login', { method: 'POST', body: { pin } });
      sessionStorage.setItem('scorecard-token', res.token);
      setToken(res.token);
    } catch (error) {
      setErr(error.message);
    }
  }

  async function refresh() {
    const [sum, cks, acts, rws, reds, txs] = await Promise.all([
      request('/children/1/summary'),
      request('/admin/checkins'),
      request('/admin/activities'),
      request('/admin/rewards'),
      request('/admin/redemptions'),
      request('/transactions'),
    ]);
    setSummary(sum || {});
    setCheckins(asArray(cks));
    setActivities(asArray(acts));
    setRewards(asArray(rws));
    setRedemptions(asArray(reds));
    setTransactions(asArray(txs));
  }

  useEffect(() => { if (token) refresh().catch(e => setToast(e.message)); }, [token]);
  useToastTimer(toast, setToast);

  async function approveCheckin(body) {
    try {
      await request(`/admin/checkins/${reviewing.id}/approve`, { method: 'POST', body: { ...body, parent_id: PARENT_ID } });
      setReviewing(null);
      setToast('打卡已通过');
      refresh();
    } catch (e) {
      setToast(e.message);
    }
  }

  async function act(path, body, ok, after) {
    try {
      await request(path, { method: 'POST', body });
      setToast(ok);
      after?.();
      refresh();
    } catch (e) {
      setToast(e.message);
    }
  }

  async function saveActivity(e) {
    e.preventDefault();
    const body = {
      ...draftActivity,
      base_points: Number(draftActivity.base_points),
      sort_order: Number(draftActivity.sort_order),
      enabled: draftActivity.id ? Boolean(draftActivity.enabled) : true,
    };
    try {
      await request(body.id ? `/admin/activities/${body.id}` : '/admin/activities', { method: body.id ? 'PUT' : 'POST', body });
      setDraftActivity(emptyActivity());
      setToast('打卡项已保存');
      refresh();
    } catch (error) {
      setToast(error.message);
    }
  }

  async function saveReward(e) {
    e.preventDefault();
    const body = {
      ...draftReward,
      cost: Number(draftReward.cost),
      stock: Number(draftReward.stock),
      enabled: draftReward.id ? Boolean(draftReward.enabled) : true,
    };
    try {
      await request(body.id ? `/admin/rewards/${body.id}` : '/admin/rewards', { method: body.id ? 'PUT' : 'POST', body });
      setDraftReward(emptyReward());
      setToast('奖励已保存');
      refresh();
    } catch (error) {
      setToast(error.message);
    }
  }

  async function doAdjust(e) {
    e.preventDefault();
    try {
      await request('/admin/adjustments', { method: 'POST', body: { user_id: CHILD_ID, parent_id: PARENT_ID, change: Number(adjust.change), reason: adjust.reason } });
      setAdjust({ change: '', reason: '' });
      setToast('已调分');
      refresh();
    } catch (error) {
      setToast(error.message);
    }
  }

  if (!token) return <Login title="家长验证" icon={<Lock />} pin={pin} setPin={setPin} err={err} onSubmit={login} />;

  const safeCheckins = asArray(checkins);
  const safeActivities = asArray(activities);
  const safeRewards = asArray(rewards);
  const safeRedemptions = asArray(redemptions);
  const safeTransactions = asArray(transactions);
  const pendingCheckins = safeCheckins.filter(c => c.status === 'pending');
  const recentCheckins = safeCheckins.filter(c => c.status !== 'pending').slice(0, 8);
  const pendingRedemptions = safeRedemptions.filter(r => r.status === 'pending');

  return <main className="admin-app">
    {toast && <Toast>{toast}</Toast>}
    <aside className="admin-sidebar">
      <div className="brand"><ShieldCheck /><span>Scorecard</span></div>
      <button className={tab === 'review' ? 'active' : ''} onClick={() => setTab('review')}><ClipboardCheck />审核</button>
      <button className={tab === 'activities' ? 'active' : ''} onClick={() => setTab('activities')}><BookOpen />打卡项</button>
      <button className={tab === 'rewards' ? 'active' : ''} onClick={() => setTab('rewards')}><Gift />奖励</button>
      <button className={tab === 'ledger' ? 'active' : ''} onClick={() => setTab('ledger')}><History />流水</button>
      <button className="sidebar-link" onClick={onChild}><Home />孩子端</button>
    </aside>
    <section className="admin-main">
      <header className="admin-header">
        <div><span className="admin-kicker">家长工作台</span><h1>{tabLabel(tab)}</h1><p>账务、审核与配置总览</p></div>
        <button className="secondary refresh-btn" onClick={refresh}><RotateCcw size={17} />刷新</button>
      </header>
      <div className="admin-stats">
        <Stat icon={<Coins />} label="当前积分" value={summary?.balance ?? 0} tone="blue" />
        <Stat icon={<Timer />} label="今日变动" value={signed(summary?.today_total ?? 0)} tone="amber" />
        <Stat icon={<ClipboardCheck />} label="待处理" value={(summary?.pending_count ?? 0) + pendingRedemptions.length} tone="red" />
      </div>

      {tab === 'review' && <section className="admin-section">
        <SectionTitle icon={<ClipboardCheck />} title="打卡审核" aside={`${pendingCheckins.length} 条待处理`} />
        <AdminCheckins items={pendingCheckins} empty="暂无待审核打卡" onApprove={setReviewing} onReject={setRejecting} />
        <SectionTitle icon={<Gift />} title="兑换审批" aside={`${pendingRedemptions.length} 条待处理`} />
        <AdminRedemptions items={pendingRedemptions} onApprove={r => act(`/admin/redemptions/${r.id}/approve`, { parent_id: PARENT_ID }, '兑换已通过')} onReject={r => act(`/admin/redemptions/${r.id}/reject`, { parent_id: PARENT_ID, note: '家长驳回' }, '兑换已驳回')} />
        <SectionTitle icon={<History />} title="最近处理" />
        <AdminCheckins items={recentCheckins} empty="暂无处理记录" onReverse={setReversing} />
      </section>}

      {tab === 'activities' && <section className="admin-section split-layout">
        <div><SectionTitle icon={<BookOpen />} title="打卡项配置" /><ActivityForm draft={draftActivity} setDraft={setDraftActivity} onSubmit={saveActivity} /></div>
        <div><SectionTitle icon={<ListChecks />} title="项目列表" /><ActivityTable items={safeActivities} onEdit={setDraftActivity} /></div>
      </section>}

      {tab === 'rewards' && <section className="admin-section split-layout">
        <div><SectionTitle icon={<Gift />} title="奖励配置" /><RewardForm draft={draftReward} setDraft={setDraftReward} onSubmit={saveReward} /></div>
        <div><SectionTitle icon={<Award />} title="奖励列表" /><RewardTable items={safeRewards} onEdit={setDraftReward} /></div>
      </section>}

      {tab === 'ledger' && <section className="admin-section">
        <SectionTitle icon={<PenLine />} title="手工调分" />
        <form className="adjust-form" onSubmit={doAdjust}>
          <input type="number" placeholder="分值，如 5 或 -3" value={adjust.change} onChange={e => setAdjust({ ...adjust, change: e.target.value })} />
          <input placeholder="原因" value={adjust.reason} onChange={e => setAdjust({ ...adjust, reason: e.target.value })} />
          <button><Check size={17} />确认</button>
        </form>
        <SectionTitle icon={<History />} title="积分流水" />
        <TransactionList items={safeTransactions} />
      </section>}
    </section>

    {reviewing && <ReviewModal checkin={reviewing} onClose={() => setReviewing(null)} onSubmit={approveCheckin} />}
    {rejecting && <ReasonModal title="驳回打卡" action="驳回" defaultReason="家长驳回" onClose={() => setRejecting(null)} onSubmit={reason => act(`/admin/checkins/${rejecting.id}/reject`, { parent_id: PARENT_ID, reason }, '已驳回', () => setRejecting(null))} />}
    {reversing && <ReasonModal title="撤回打卡" action="撤回" defaultReason="家长撤回" onClose={() => setReversing(null)} onSubmit={reason => act(`/admin/checkins/${reversing.id}/reverse`, { parent_id: PARENT_ID, reason }, '已撤回', () => setReversing(null))} />}
  </main>;
}

function ReviewModal({ checkin, onClose, onSubmit }) {
  const [level, setLevel] = useState('pass');
  const [minutes, setMinutes] = useState(30);
  const [counts, setCounts] = useState(true);
  const [note, setNote] = useState('');
  return <Modal title="通过打卡" onClose={onClose}>
    <div className="review-target"><span>{iconNode(checkin.activity_icon)}</span><div><strong>{checkin.activity_label}</strong><small>{checkin.activity_date} · {checkin.source === 'makeup' ? '补签' : '正常打卡'} · {scoreText({ score_mode: checkin.score_mode, base_points: checkin.base_points })}</small></div></div>
    {checkin.score_mode === 'quality' && <div className="field"><label>质量档位</label><Segmented value={level} onChange={setLevel} options={[['pass','及格'], ['good','良好'], ['excellent','优秀']]} /></div>}
    {checkin.score_mode === 'duration' && <div className="field"><label>时长分钟</label><input type="number" min="10" step="10" value={minutes} onChange={e => setMinutes(Number(e.target.value))} /></div>}
    <label className="checkbox-line"><input type="checkbox" checked={counts} onChange={e => setCounts(e.target.checked)} />计入连续打卡</label>
    <div className="field"><label>备注</label><input value={note} onChange={e => setNote(e.target.value)} placeholder="可选" /></div>
    <div className="modal-actions"><button className="secondary" onClick={onClose}>取消</button><button onClick={() => onSubmit({ review_level: level, review_minutes: minutes, counts_for_streak: counts, note })}><Check size={17} />通过</button></div>
  </Modal>;
}

function ReasonModal({ title, action, defaultReason, onClose, onSubmit }) {
  const [reason, setReason] = useState(defaultReason);
  return <Modal title={title} onClose={onClose}>
    <div className="field"><label>原因</label><input value={reason} onChange={e => setReason(e.target.value)} /></div>
    <div className="modal-actions"><button className="secondary" onClick={onClose}>取消</button><button className="danger" onClick={() => onSubmit(reason)}>{action}</button></div>
  </Modal>;
}

function Modal({ title, children, onClose }) {
  return <div className="modal-backdrop" onMouseDown={onClose}>
    <div className="modal" onMouseDown={e => e.stopPropagation()}>
      <div className="modal-head"><h2>{title}</h2><button className="ghost-icon dark" onClick={onClose}><X size={18} /></button></div>
      {children}
    </div>
  </div>;
}

function Login({ title, icon, pin, setPin, err, onSubmit }) {
  return <main className="login-page"><form className="login-card" onSubmit={onSubmit}>
    <div className="login-icon">{icon}</div>
    <h1>{title}</h1>
    <input autoFocus type="password" inputMode="numeric" value={pin} onChange={e => setPin(e.target.value)} placeholder="PIN" />
    <div className="error-line">{err}</div>
    <button>进入</button>
  </form></main>;
}

function SectionTitle({ icon, title, aside }) {
  return <div className="section-title"><span>{icon}</span><h2>{title}</h2>{aside && <em>{aside}</em>}</div>;
}

function MiniStat({ icon, label, value }) {
  return <div className="mini-stat"><span>{icon}</span><div><b>{value}</b><small>{label}</small></div></div>;
}

function Stat({ icon, label, value, tone = 'blue' }) {
  return <div className="stat-card" data-tone={tone}><span>{icon}</span><div><b>{value}</b><small>{label}</small></div></div>;
}

function Segmented({ value, onChange, options }) {
  return <div className="segmented">{options.map(opt => {
    const val = Array.isArray(opt) ? opt[0] : opt;
    const label = Array.isArray(opt) ? opt[1] : opt;
    return <button key={val} className={value === val ? 'active' : ''} onClick={() => onChange(val)} type="button">{label}</button>;
  })}</div>;
}

function CheckinList({ items }) {
  items = asArray(items);
  if (!items.length) return <EmptyState text="暂无打卡记录" />;
  return <div className="record-list">{items.map(c => <RecordRow key={c.id} icon={iconNode(c.activity_icon)} title={c.activity_label} meta={`${c.activity_date} · ${status(c.status)} · ${c.source === 'makeup' ? '补签' : '正常'}`} value={c.awarded_points + c.streak_bonus || '-'} badge={c.source === 'makeup' ? '补签' : null} />)}</div>;
}

function RedemptionList({ items }) {
  items = asArray(items);
  if (!items.length) return <EmptyState text="暂无兑换记录" />;
  return <div className="record-list">{items.map(r => <RecordRow key={r.id} icon={<Gift />} title={r.reward_name} meta={status(r.status)} value={`${r.cost_at_time}分`} />)}</div>;
}

function TransactionList({ items }) {
  items = asArray(items);
  if (!items.length) return <EmptyState text="暂无积分流水" />;
  return <div className="record-list">{items.map(t => <RecordRow key={t.id} icon={t.change > 0 ? <Plus /> : <ChevronRight />} title={t.reason} meta={`${formatTime(t.created_at)} · ${sourceLabel(t.source_type)}`} value={signed(t.change)} tone={t.change > 0 ? 'positive' : 'negative'} />)}</div>;
}

function RecordRow({ icon, title, meta, value, tone, badge }) {
  return <div className="record-row">
    <span className="record-icon">{icon}</span>
    <div className="record-main"><strong>{title}</strong><small>{meta}</small></div>
    {badge && <span className="badge soft">{badge}</span>}
    <b className={tone || ''}>{value}</b>
  </div>;
}

function AdminCheckins({ items, empty, onApprove, onReject, onReverse }) {
  items = asArray(items);
  if (!items.length) return <EmptyState text={empty} />;
  return <div className="admin-list">{items.map(c => <div key={c.id} className="admin-item">
    <span className="record-icon">{iconNode(c.activity_icon)}</span>
    <div className="admin-item-main"><div className="item-title-line"><strong>{c.activity_label}</strong><StatusBadge value={c.status} /></div><small>{c.user_name} · {c.activity_date} · {c.source === 'makeup' ? '补签' : '正常'} · {scoreText({ score_mode: c.score_mode, base_points: c.base_points })}</small></div>
    <div className="row-actions">{c.status === 'pending' && <><button onClick={() => onApprove(c)}><Check size={16} />通过</button><button className="danger" onClick={() => onReject(c)}><X size={16} />驳回</button></>}{c.status === 'approved' && <button className="danger" onClick={() => onReverse(c)}><RotateCcw size={16} />撤回</button>}</div>
  </div>)}</div>;
}

function AdminRedemptions({ items, onApprove, onReject }) {
  items = asArray(items);
  if (!items.length) return <EmptyState text="暂无待审批兑换" />;
  return <div className="admin-list">{items.map(r => <div key={r.id} className="admin-item">
    <span className="record-icon"><Gift /></span>
    <div className="admin-item-main"><div className="item-title-line"><strong>{r.reward_name}</strong><StatusBadge value={r.status} /></div><small>{r.user_name} · {r.cost_at_time} 分</small></div>
    <div className="row-actions"><button onClick={() => onApprove(r)}><Check size={16} />通过</button><button className="danger" onClick={() => onReject(r)}><X size={16} />驳回</button></div>
  </div>)}</div>;
}

function ActivityForm({ draft, setDraft, onSubmit }) {
  return <form className="config-panel" onSubmit={onSubmit}>
    <div className="field"><label>名称</label><input value={draft.label} onChange={e => setDraft({ ...draft, label: e.target.value })} /></div>
    <div className="form-pair"><div className="field"><label>基础分</label><input type="number" value={draft.base_points} onChange={e => setDraft({ ...draft, base_points: e.target.value })} /></div><div className="field"><label>类型</label><select value={draft.score_mode} onChange={e => setDraft({ ...draft, score_mode: e.target.value })}><option value="default">默认</option><option value="quality">质量</option><option value="duration">时长</option></select></div></div>
    <div className="form-pair"><div className="field"><label>分类</label><input value={draft.category} onChange={e => setDraft({ ...draft, category: e.target.value })} /></div><div className="field"><label>排序</label><input type="number" value={draft.sort_order} onChange={e => setDraft({ ...draft, sort_order: e.target.value })} /></div></div>
    <div className="field"><label>图标</label><IconPicker value={draft.icon} onChange={icon => setDraft({ ...draft, icon })} /></div>
    <button><Check size={17} />{draft.id ? '保存修改' : '新增打卡项'}</button>
  </form>;
}

function IconPicker({ value, onChange }) {
  const icons = ['book', 'read', 'pen', 'math', 'note', 'voice', 'letters', 'run', 'bag', 'home', 'music', 'star'];
  return <div className="icon-picker">{icons.map(ic => <button key={ic} type="button" className={value === ic ? 'active' : ''} onClick={() => onChange(ic)} title={ic}>{iconNode(ic)}</button>)}</div>;
}

function ActivityTable({ items, onEdit }) {
  items = asArray(items);
  if (!items.length) return <EmptyState text="暂无打卡项" />;
  return <div className="admin-list compact-list">{items.map(a => <div key={a.id} className="admin-item"><span className="record-icon">{iconNode(a.icon)}</span><div className="admin-item-main"><strong>{a.label}</strong><small>{a.category} · {scoreText(a)} · {a.enabled ? '启用' : '停用'}</small></div><button className="secondary" onClick={() => onEdit(a)}><PenLine size={16} />编辑</button></div>)}</div>;
}

function RewardForm({ draft, setDraft, onSubmit }) {
  return <form className="config-panel" onSubmit={onSubmit}>
    <div className="field"><label>奖励名称</label><input value={draft.name} onChange={e => setDraft({ ...draft, name: e.target.value })} /></div>
    <div className="form-pair"><div className="field"><label>积分</label><input type="number" value={draft.cost} onChange={e => setDraft({ ...draft, cost: e.target.value })} /></div><div className="field"><label>库存</label><input type="number" value={draft.stock} onChange={e => setDraft({ ...draft, stock: e.target.value })} /></div></div>
    <div className="field"><label>描述</label><input value={draft.description} onChange={e => setDraft({ ...draft, description: e.target.value })} /></div>
    <label className="checkbox-line"><input type="checkbox" checked={draft.auto_approve} onChange={e => setDraft({ ...draft, auto_approve: e.target.checked })} />自动通过</label>
    <button><Check size={17} />{draft.id ? '保存修改' : '新增奖励'}</button>
  </form>;
}

function RewardTable({ items, onEdit }) {
  items = asArray(items);
  if (!items.length) return <EmptyState text="暂无奖励" />;
  return <div className="admin-list compact-list">{items.map(r => <div key={r.id} className="admin-item"><span className="record-icon"><Gift /></span><div className="admin-item-main"><strong>{r.name}</strong><small>{r.cost} 分 · {r.auto_approve ? '自动通过' : '需审批'} · 库存 {r.stock < 0 ? '无限' : r.stock}</small></div><button className="secondary" onClick={() => onEdit(r)}><PenLine size={16} />编辑</button></div>)}</div>;
}

function StatusBadge({ value }) {
  return <span className={`badge ${value}`}>{status(value)}</span>;
}

function EmptyState({ text }) {
  return <div className="empty-state"><ListChecks /><span>{text}</span></div>;
}

function Toast({ children }) {
  return <div className="toast">{children}</div>;
}

function useToastTimer(toast, setToast) {
  useEffect(() => {
    if (!toast) return undefined;
    const t = setTimeout(() => setToast(''), 1800);
    return () => clearTimeout(t);
  }, [toast, setToast]);
}

function sortCategories(categories) {
  return CAT_ORDER.filter(c => categories.includes(c)).concat(categories.filter(c => !CAT_ORDER.includes(c)));
}

function signed(n) { return n > 0 ? `+${n}` : String(n); }
function status(s) { return { pending: '待审核', approved: '已通过', rejected: '已驳回', reversed: '已撤回', fulfilled: '已完成' }[s] || s; }
function sourceLabel(s) { return { checkin: '打卡', checkin_reversal: '撤回', redemption: '兑换', adjustment: '调分' }[s] || s; }
function scoreText(a) { return a.score_mode === 'quality' ? `基础分 ${a.base_points} · 质量` : a.score_mode === 'duration' ? `基础分 ${a.base_points} · 时长` : `基础分 ${a.base_points}`; }
function modeLabel(mode) { return mode === 'quality' ? '质量审核' : mode === 'duration' ? '按时长' : '固定分'; }
function iconNode(name) { return ({ book: '📚', read: '📖', pen: '✍️', math: '🔢', note: '📝', voice: '🗣️', letters: '🔤', run: '🏃', bag: '🎒', home: '🏠', music: '🎹', star: '⭐' }[name] || name || '⭐'); }
function formatTime(value) { return new Date(value.replace(' ', 'T') + 'Z').toLocaleString('zh-CN'); }
function formatDateLabel(value) { return value === today() ? '今日' : value; }
function tabLabel(k) { return { review: '审核台', activities: '打卡项', rewards: '奖励', ledger: '积分流水' }[k]; }
function emptyActivity() { return { label: '', base_points: 1, score_mode: 'default', icon: 'star', color: '#3B82F6', category: '生活', sort_order: 0, enabled: true }; }
function emptyReward() { return { name: '', cost: 10, description: '', stock: -1, auto_approve: true, enabled: true }; }

createRoot(document.getElementById('root')).render(<App />);
