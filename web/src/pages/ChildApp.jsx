import { useEffect, useMemo, useState } from 'react';
import {
  CalendarDays,
  Check,
  ClipboardCheck,
  Clock3,
  Coins,
  Gift,
  History,
  ListChecks,
  Loader2,
  LogOut,
  Settings,
  ShieldCheck,
  Sparkles,
  Target,
  Trophy,
} from 'lucide-react';
import { api, clearChildSession, hasChildSession, setChildToken } from '../api/client';
import {
  CheckinList,
  EmptyState,
  Login,
  MiniStat,
  RedemptionList,
  SectionTitle,
  Segmented,
  Toast,
  TransactionList,
} from '../components/ui';
import {
  asArray,
  formatDateLabel,
  offsetDate,
  scoreText,
  signed,
  sortCategories,
  today,
} from '../lib/format';
import { useToastTimer } from '../lib/hooks';
import { iconNode, modeLabel } from '../lib/icons';

export default function ChildApp({ onAdmin }) {
  const [verified, setVerified] = useState(hasChildSession());
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
      const res = await api.childLogin(pin);
      setChildToken(res.token);
      setVerified(true);
    } catch (error) {
      setErr(error.message);
    }
  }

  async function refresh() {
    const [sum, acts, cks, txs, rws, reds] = await Promise.all([
      api.summary(),
      api.activities(),
      api.checkins(),
      api.transactions(),
      api.rewards(),
      api.redemptions(),
    ]);
    setSummary(sum || {});
    setActivities(asArray(acts));
    setCheckins(asArray(cks));
    setTransactions(asArray(txs));
    setRewards(asArray(rws));
    setRedemptions(asArray(reds));
  }

  useEffect(() => {
    if (!verified) return undefined;
    let cancelled = false;
    refresh().catch((e) => {
      if (!cancelled) {
        if (e.status === 401) setVerified(false);
        setToast(e.message);
      }
    });
    return () => { cancelled = true; };
  }, [verified]);

  useToastTimer(toast, setToast);

  async function submitCheckin(activity) {
    setLoadingID(activity.id);
    try {
      await api.createCheckin({ activity_id: activity.id, activity_date: activityDate });
      setToast(activityDate === today() ? '已提交，等待家长审核' : '补签已提交，等待家长审核');
      await refresh();
    } catch (error) {
      if (error.status === 401) setVerified(false);
      setToast(error.message);
    } finally {
      setLoadingID(null);
    }
  }

  async function redeem(reward) {
    if (!window.confirm(`确定兑换「${reward.name}」吗？需要 ${reward.cost} 分。`)) return;
    try {
      await api.createRedemption({ reward_id: reward.id });
      setToast(reward.auto_approve ? '兑换成功' : '兑换已提交');
      await refresh();
    } catch (error) {
      if (error.status === 401) setVerified(false);
      setToast(error.message);
    }
  }

  async function cancelCheckin(checkin) {
    if (!window.confirm(`取消待审核打卡「${checkin.activity_label}」？`)) return;
    try {
      await api.cancelCheckin(checkin.id);
      setToast('已取消打卡');
      await refresh();
    } catch (error) {
      if (error.status === 401) setVerified(false);
      setToast(error.message);
    }
  }

  async function logout() {
    try {
      await api.logout();
    } catch {
      // ignore network errors on logout
    }
    clearChildSession();
    setVerified(false);
    setPin('');
  }

  const safeActivities = asArray(activities);
  const safeCheckins = asArray(checkins);
  const safeRewards = asArray(rewards);
  const safeRedemptions = asArray(redemptions);
  const safeTransactions = asArray(transactions);
  const balance = summary?.balance ?? 0;
  const categories = useMemo(
    () => sortCategories([...new Set(safeActivities.map((a) => a.category || '其他'))]),
    [safeActivities],
  );
  const filtered = activeCat === '全部' ? safeActivities : safeActivities.filter((a) => a.category === activeCat);
  const pendingCheckins = safeCheckins.filter((c) => c.status === 'pending');
  const todayPending = pendingCheckins.slice(0, 3);
  const dateCheckinCounts = useMemo(() => {
    const counts = new Map();
    safeCheckins.forEach((c) => {
      if (c.activity_date !== activityDate || c.status === 'rejected' || c.status === 'reversed') return;
      counts.set(c.activity_id, (counts.get(c.activity_id) || 0) + 1);
    });
    return counts;
  }, [safeCheckins, activityDate]);
  const submittedOnDate = [...dateCheckinCounts.values()].reduce((sum, n) => sum + n, 0);
  const sortedRewards = useMemo(() => [...safeRewards].sort((a, b) => a.cost - b.cost), [safeRewards]);
  const targetReward = sortedRewards.find((r) => r.cost > balance) || sortedRewards[sortedRewards.length - 1] || null;
  const targetProgress = targetReward ? Math.min(100, Math.round((balance / Math.max(targetReward.cost, 1)) * 100)) : 0;
  const approvedCheckins = safeCheckins.filter((c) => c.status === 'approved');
  const fulfilledRedemptions = safeRedemptions.filter((r) => r.status === 'fulfilled');
  const earnedTotal = safeTransactions.filter((t) => t.change > 0).reduce((sum, t) => sum + t.change, 0);
  const spentTotal = Math.abs(safeTransactions.filter((t) => t.change < 0).reduce((sum, t) => sum + t.change, 0));

  if (!verified) {
    return (
      <Login
        title="欢迎打卡"
        icon={<ShieldCheck />}
        pin={pin}
        setPin={setPin}
        err={err}
        onSubmit={login}
        hint="输入孩子端 PIN（默认与 ADMIN_PIN 相同；未单独设置 CHILD_PIN 时）。"
      />
    );
  }

  return (
    <main className="child-app">
      {toast && <Toast>{toast}</Toast>}
      <aside className="child-sidebar">
        <section className="child-hero">
          <div className="hero-topline">
            <span><Sparkles />积分打卡</span>
            <div className="hero-actions">
              <button type="button" className="ghost-icon" onClick={logout} title="退出登录"><LogOut size={18} /></button>
              <button type="button" className="ghost-icon parent-entry" onClick={onAdmin} title="家长管理"><Settings size={19} /></button>
            </div>
          </div>
          <div className="hero-label">当前积分</div>
          <div className="hero-balance">{summary?.balance ?? 0}</div>
          <div className="hero-stats">
            <MiniStat icon={<Coins />} label="今日" value={signed(summary?.today_total ?? 0)} />
            <MiniStat icon={<Clock3 />} label="待审核" value={summary?.pending_count ?? 0} />
            <MiniStat icon={<Trophy />} label="最长连续" value={`${summary?.max_streak_days ?? 0}天`} />
          </div>
        </section>

        <section className="goal-card">
          <div className="goal-head">
            <span><Trophy /></span>
            <div>
              <strong>{targetReward ? '下一个小目标' : '今天从小任务开始'}</strong>
              <small>{targetReward ? targetReward.name : '完成打卡后这里会出现奖励进度'}</small>
            </div>
          </div>
          {targetReward ? (
            <>
              <div className="goal-progress"><i style={{ width: `${targetProgress}%` }} /></div>
              <div className="goal-foot">
                <b>{balance}/{targetReward.cost} 分</b>
                <button type="button" className="tiny-action" onClick={() => setTab('rewards')}>
                  {balance >= targetReward.cost ? '去兑换' : `还差 ${targetReward.cost - balance} 分`}
                </button>
              </div>
            </>
          ) : (
            <button type="button" className="tiny-action wide" onClick={() => setTab('checkin')}>开始打卡</button>
          )}
        </section>

        <nav className="bottom-nav">
          <button type="button" className={tab === 'checkin' ? 'active' : ''} onClick={() => setTab('checkin')}><ClipboardCheck />打卡</button>
          <button type="button" className={tab === 'rewards' ? 'active' : ''} onClick={() => setTab('rewards')}><Gift />奖励</button>
          <button type="button" className={tab === 'records' ? 'active' : ''} onClick={() => setTab('records')}><History />记录</button>
        </nav>
      </aside>

      {tab === 'checkin' && (
        <section className="child-page">
          <div className="daily-strip">
            <div><span><Target /></span><strong>{activityDate === today() ? '今日任务' : '补签任务'}</strong><small>{filtered.length} 个项目可选</small></div>
            <div><span><Check /></span><strong>{submittedOnDate}</strong><small>{activityDate === today() ? '今日已提交' : '这天已提交'}</small></div>
          </div>
          <div className="date-panel">
            <div><strong>{activityDate === today() ? '今天打卡' : '补签打卡'}</strong><span>{formatDateLabel(activityDate)}</span></div>
            <div className="date-controls">
              <button type="button" className="secondary" onClick={() => setActivityDate(today())}>今天</button>
              <button type="button" className="secondary" onClick={() => setActivityDate(offsetDate(-1))}>昨天</button>
              <label>
                <CalendarDays size={17} />
                <input type="date" value={activityDate} max={today()} onChange={(e) => setActivityDate(e.target.value)} />
              </label>
            </div>
          </div>

          {todayPending.length > 0 && (
            <div className="pending-strip">
              {todayPending.map((c) => <span key={c.id}>{iconNode(c.activity_icon)} {c.activity_label}</span>)}
            </div>
          )}

          <Segmented value={activeCat} onChange={setActiveCat} options={['全部', ...categories]} />
          {filtered.length ? (
            <div className="activity-grid">
              {filtered.map((a) => {
                const submittedCount = dateCheckinCounts.get(a.id) || 0;
                return (
                  <button
                    key={a.id}
                    type="button"
                    className="activity-card"
                    data-category={a.category}
                    data-submitted={submittedCount > 0 ? 'yes' : 'no'}
                    onClick={() => submitCheckin(a)}
                    disabled={loadingID === a.id}
                  >
                    {submittedCount > 0 && <span className="submitted-ribbon"><Check />已提交{submittedCount > 1 ? ` ${submittedCount}` : ''}</span>}
                    <span className="activity-icon">{loadingID === a.id ? <Loader2 className="spin" /> : iconNode(a.icon)}</span>
                    <span className="activity-category">{a.category}</span>
                    <strong>{a.label}</strong>
                    <small>{scoreText(a)}</small>
                    <span className="mode-pill">{modeLabel(a.score_mode)}</span>
                  </button>
                );
              })}
            </div>
          ) : (
            <EmptyState icon={<Target />} title="这个分类还没有任务" text="换个分类看看，或请家长添加新的学习打卡项" />
          )}
          <SectionTitle icon={<History />} title="最近打卡" />
          <CheckinList items={safeCheckins.slice(0, 8)} onCancel={cancelCheckin} />
        </section>
      )}

      {tab === 'rewards' && (
        <section className="child-page">
          <SectionTitle icon={<Gift />} title="积分兑换" />
          {safeRewards.length ? (
            <div className="reward-grid">
              {safeRewards.map((r) => {
                const canRedeem = balance >= r.cost;
                const progress = Math.min(100, Math.round((balance / Math.max(r.cost, 1)) * 100));
                return (
                  <button key={r.id} type="button" className="reward-card" data-ready={canRedeem ? 'yes' : 'no'} onClick={() => redeem(r)}>
                    <span className="reward-icon"><Gift /></span>
                    <strong>{r.name}</strong>
                    <small>{r.description || (r.auto_approve ? '自动通过' : '需要审批')}</small>
                    <div className="reward-progress"><i style={{ width: `${progress}%` }} /></div>
                    <div className="reward-footer">
                      <b>{r.cost} 分</b>
                      <em>{canRedeem ? '可兑换' : `差 ${r.cost - balance} 分`}</em>
                    </div>
                  </button>
                );
              })}
            </div>
          ) : (
            <EmptyState icon={<Gift />} title="奖励货架还是空的" text="请家长添加几个想兑换的小奖励" />
          )}
          <SectionTitle icon={<ListChecks />} title="兑换记录" />
          <RedemptionList items={safeRedemptions} />
        </section>
      )}

      {tab === 'records' && (
        <section className="child-page">
          <div className="growth-board">
            <div><span><ClipboardCheck /></span><strong>{approvedCheckins.length}</strong><small>已通过打卡</small></div>
            <div><span><Sparkles /></span><strong>{earnedTotal}</strong><small>累计获得</small></div>
            <div><span><Gift /></span><strong>{fulfilledRedemptions.length}</strong><small>完成兑换</small></div>
            <div><span><Coins /></span><strong>{spentTotal}</strong><small>已使用积分</small></div>
          </div>
          <SectionTitle icon={<History />} title="积分流水" />
          <TransactionList items={safeTransactions} />
        </section>
      )}
    </main>
  );
}
