import { useEffect, useState } from 'react';
import {
  Award,
  BookOpen,
  Check,
  ClipboardCheck,
  Coins,
  Gift,
  History,
  Home,
  ListChecks,
  Lock,
  PenLine,
  RotateCcw,
  ShieldCheck,
  Timer,
} from 'lucide-react';
import { api, getParentToken, hasParentSession, setParentToken } from '../api/client';
import {
  ActivityForm,
  ActivityTable,
  AdminCheckins,
  AdminRedemptions,
  Login,
  ReasonModal,
  ReviewModal,
  RewardForm,
  RewardTable,
  SectionTitle,
  Stat,
  Toast,
  TransactionList,
} from '../components/ui';
import { asArray, emptyActivity, emptyReward, signed, tabLabel } from '../lib/format';
import { useToastTimer } from '../lib/hooks';

export default function AdminApp({ onChild }) {
  const [token, setToken] = useState(hasParentSession() ? getParentToken() : '');
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
      const res = await api.parentLogin(pin);
      setParentToken(res.token);
      setToken(res.token);
    } catch (error) {
      setErr(error.message);
    }
  }

  async function refresh() {
    const [sum, cks, acts, rws, reds, txs] = await Promise.all([
      api.summary(),
      api.adminCheckins(),
      api.adminActivities(),
      api.adminRewards(),
      api.adminRedemptions(),
      api.transactions(),
    ]);
    setSummary(sum || {});
    setCheckins(asArray(cks));
    setActivities(asArray(acts));
    setRewards(asArray(rws));
    setRedemptions(asArray(reds));
    setTransactions(asArray(txs));
  }

  useEffect(() => {
    if (!token) return undefined;
    let cancelled = false;
    refresh().catch((e) => {
      if (!cancelled) {
        if (e.status === 401) setToken('');
        setToast(e.message);
      }
    });
    return () => { cancelled = true; };
  }, [token]);

  useToastTimer(toast, setToast);

  async function run(action, okMsg, after) {
    try {
      await action();
      setToast(okMsg);
      after?.();
      await refresh();
    } catch (e) {
      if (e.status === 401) setToken('');
      setToast(e.message);
    }
  }

  async function approveCheckin(body) {
    await run(() => api.approveCheckin(reviewing.id, body), '打卡已通过', () => setReviewing(null));
  }

  async function saveActivity(e) {
    e.preventDefault();
    const body = {
      ...draftActivity,
      base_points: Number(draftActivity.base_points),
      sort_order: Number(draftActivity.sort_order),
      enabled: draftActivity.id ? Boolean(draftActivity.enabled) : true,
    };
    await run(async () => {
      await api.saveActivity(body);
      setDraftActivity(emptyActivity());
    }, '打卡项已保存');
  }

  async function saveReward(e) {
    e.preventDefault();
    const body = {
      ...draftReward,
      cost: Number(draftReward.cost),
      stock: Number(draftReward.stock),
      enabled: draftReward.id ? Boolean(draftReward.enabled) : true,
    };
    await run(async () => {
      await api.saveReward(body);
      setDraftReward(emptyReward());
    }, '奖励已保存');
  }

  async function doAdjust(e) {
    e.preventDefault();
    await run(async () => {
      await api.adjust({ change: Number(adjust.change), reason: adjust.reason });
      setAdjust({ change: '', reason: '' });
    }, '已调分');
  }

  if (!token) {
    return <Login title="家长验证" icon={<Lock />} pin={pin} setPin={setPin} err={err} onSubmit={login} />;
  }

  const safeCheckins = asArray(checkins);
  const safeActivities = asArray(activities);
  const safeRewards = asArray(rewards);
  const safeRedemptions = asArray(redemptions);
  const safeTransactions = asArray(transactions);
  const pendingCheckins = safeCheckins.filter((c) => c.status === 'pending');
  const recentCheckins = safeCheckins.filter((c) => c.status !== 'pending').slice(0, 8);
  const pendingRedemptions = safeRedemptions.filter((r) => r.status === 'pending');

  return (
    <main className="admin-app">
      {toast && <Toast>{toast}</Toast>}
      <aside className="admin-sidebar">
        <div className="brand"><ShieldCheck /><span>Scorecard</span></div>
        <button type="button" className={tab === 'review' ? 'active' : ''} onClick={() => setTab('review')}><ClipboardCheck />审核</button>
        <button type="button" className={tab === 'activities' ? 'active' : ''} onClick={() => setTab('activities')}><BookOpen />打卡项</button>
        <button type="button" className={tab === 'rewards' ? 'active' : ''} onClick={() => setTab('rewards')}><Gift />奖励</button>
        <button type="button" className={tab === 'ledger' ? 'active' : ''} onClick={() => setTab('ledger')}><History />流水</button>
        <button type="button" className="sidebar-link" onClick={onChild}><Home />孩子端</button>
      </aside>
      <section className="admin-main">
        <header className="admin-header">
          <div>
            <span className="admin-kicker">家长工作台</span>
            <h1>{tabLabel(tab)}</h1>
            <p>账务、审核与配置总览</p>
          </div>
          <button type="button" className="secondary refresh-btn" onClick={() => refresh().catch((e) => setToast(e.message))}>
            <RotateCcw size={17} />刷新
          </button>
        </header>
        <div className="admin-stats">
          <Stat icon={<Coins />} label="当前积分" value={summary?.balance ?? 0} tone="blue" />
          <Stat icon={<Timer />} label="今日变动" value={signed(summary?.today_total ?? 0)} tone="amber" />
          <Stat icon={<ClipboardCheck />} label="待处理" value={(summary?.pending_count ?? 0) + pendingRedemptions.length} tone="red" />
        </div>

        {tab === 'review' && (
          <section className="admin-section">
            <div className="review-board">
              <div><span><ClipboardCheck /></span><strong>{pendingCheckins.length}</strong><small>打卡待审核</small></div>
              <div><span><Gift /></span><strong>{pendingRedemptions.length}</strong><small>兑换待审批</small></div>
            </div>
            <SectionTitle icon={<ClipboardCheck />} title="打卡审核" aside={`${pendingCheckins.length} 条待处理`} />
            <AdminCheckins items={pendingCheckins} empty="暂无待审核打卡" onApprove={setReviewing} onReject={setRejecting} />
            <SectionTitle icon={<Gift />} title="兑换审批" aside={`${pendingRedemptions.length} 条待处理`} />
            <AdminRedemptions
              items={pendingRedemptions}
              onApprove={(r) => run(() => api.approveRedemption(r.id), '兑换已通过')}
              onReject={(r) => run(() => api.rejectRedemption(r.id, { note: '家长驳回' }), '兑换已驳回')}
            />
            <SectionTitle icon={<History />} title="最近处理" />
            <AdminCheckins items={recentCheckins} empty="暂无处理记录" onReverse={setReversing} />
          </section>
        )}

        {tab === 'activities' && (
          <section className="admin-section split-layout">
            <div>
              <SectionTitle icon={<BookOpen />} title="打卡项配置" />
              <ActivityForm draft={draftActivity} setDraft={setDraftActivity} onSubmit={saveActivity} />
            </div>
            <div>
              <SectionTitle icon={<ListChecks />} title="项目列表" />
              <ActivityTable items={safeActivities} onEdit={setDraftActivity} />
            </div>
          </section>
        )}

        {tab === 'rewards' && (
          <section className="admin-section split-layout">
            <div>
              <SectionTitle icon={<Gift />} title="奖励配置" />
              <RewardForm draft={draftReward} setDraft={setDraftReward} onSubmit={saveReward} />
            </div>
            <div>
              <SectionTitle icon={<Award />} title="奖励列表" />
              <RewardTable items={safeRewards} onEdit={setDraftReward} />
            </div>
          </section>
        )}

        {tab === 'ledger' && (
          <section className="admin-section">
            <SectionTitle icon={<PenLine />} title="手工调分" />
            <form className="adjust-form" onSubmit={doAdjust}>
              <input type="number" placeholder="分值，如 5 或 -3" value={adjust.change} onChange={(e) => setAdjust({ ...adjust, change: e.target.value })} />
              <input placeholder="原因" value={adjust.reason} onChange={(e) => setAdjust({ ...adjust, reason: e.target.value })} />
              <button type="submit"><Check size={17} />确认</button>
            </form>
            <SectionTitle icon={<History />} title="积分流水" />
            <TransactionList items={safeTransactions} />
          </section>
        )}
      </section>

      {reviewing && <ReviewModal checkin={reviewing} onClose={() => setReviewing(null)} onSubmit={approveCheckin} />}
      {rejecting && (
        <ReasonModal
          title="驳回打卡"
          action="驳回"
          defaultReason="家长驳回"
          onClose={() => setRejecting(null)}
          onSubmit={(reason) => run(() => api.rejectCheckin(rejecting.id, { reason }), '已驳回', () => setRejecting(null))}
        />
      )}
      {reversing && (
        <ReasonModal
          title="撤回打卡"
          action="撤回"
          defaultReason="家长撤回"
          onClose={() => setReversing(null)}
          onSubmit={(reason) => run(() => api.reverseCheckin(reversing.id, { reason }), '已撤回', () => setReversing(null))}
        />
      )}
    </main>
  );
}
