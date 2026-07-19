import { useState } from 'react';
import {
  BookOpen,
  Check,
  ChevronRight,
  ClipboardCheck,
  Coins,
  Gift,
  ListChecks,
  Lock,
  PenLine,
  Plus,
  RotateCcw,
  X,
} from 'lucide-react';
import { asArray, emptyActivity, emptyReward, formatTime, scoreText, signed, sourceLabel, status } from '../lib/format';
import { iconNode } from '../lib/icons';

export function Modal({ title, children, onClose }) {
  return (
    <div className="modal-backdrop" onMouseDown={onClose}>
      <div className="modal" onMouseDown={(e) => e.stopPropagation()}>
        <div className="modal-head">
          <h2>{title}</h2>
          <button type="button" className="ghost-icon dark" onClick={onClose}><X size={18} /></button>
        </div>
        {children}
      </div>
    </div>
  );
}

export function Login({ title, icon, pin, setPin, err, onSubmit }) {
  return (
    <main className="login-page">
      <form className="login-card" onSubmit={onSubmit}>
        <div className="login-brand"><span className="login-icon">{icon}</span><span>Scorecard</span></div>
        <h1>{title}</h1>
        <p>输入 PIN 后继续使用积分打卡。</p>
        <div className="pin-shell">
          <Lock size={17} />
          <input autoFocus type="password" inputMode="numeric" autoComplete="one-time-code" value={pin} onChange={(e) => setPin(e.target.value)} placeholder="PIN" />
        </div>
        <div className="error-line">{err}</div>
        <button type="submit">进入</button>
      </form>
    </main>
  );
}

export function SectionTitle({ icon, title, aside }) {
  return (
    <div className="section-title">
      <span>{icon}</span>
      <h2>{title}</h2>
      {aside && <em>{aside}</em>}
    </div>
  );
}

export function MiniStat({ icon, label, value }) {
  return (
    <div className="mini-stat">
      <span>{icon}</span>
      <div><b>{value}</b><small>{label}</small></div>
    </div>
  );
}

export function Stat({ icon, label, value, tone = 'blue' }) {
  return (
    <div className="stat-card" data-tone={tone}>
      <span>{icon}</span>
      <div><b>{value}</b><small>{label}</small></div>
    </div>
  );
}

export function Segmented({ value, onChange, options }) {
  return (
    <div className="segmented">
      {options.map((opt) => {
        const val = Array.isArray(opt) ? opt[0] : opt;
        const label = Array.isArray(opt) ? opt[1] : opt;
        return (
          <button key={val} className={value === val ? 'active' : ''} onClick={() => onChange(val)} type="button">
            {label}
          </button>
        );
      })}
    </div>
  );
}

export function Toast({ children }) {
  return <div className="toast">{children}</div>;
}

export function EmptyState({ icon, title, text }) {
  return (
    <div className="empty-state">
      <span>{icon || <ListChecks />}</span>
      <strong>{title || text}</strong>
      {text && title && <small>{text}</small>}
    </div>
  );
}

export function StatusBadge({ value }) {
  return <span className={`badge ${value}`}>{status(value)}</span>;
}

export function EnabledBadge({ enabled }) {
  return <span className={`badge ${enabled ? 'approved' : 'reversed'}`}>{enabled ? '启用' : '停用'}</span>;
}

function RecordRow({ icon, title, meta, value, tone, badge }) {
  return (
    <div className="record-row">
      <span className="record-icon">{icon}</span>
      <div className="record-main"><strong>{title}</strong><small>{meta}</small></div>
      {badge && <span className="badge soft">{badge}</span>}
      <b className={tone || ''}>{value}</b>
    </div>
  );
}

export function CheckinList({ items }) {
  const list = asArray(items);
  if (!list.length) return <EmptyState icon={<ClipboardCheck />} title="还没有打卡记录" text="完成一个任务后，这里会留下成长轨迹" />;
  return (
    <div className="record-list timeline-list">
      {list.map((c) => (
        <RecordRow
          key={c.id}
          icon={iconNode(c.activity_icon)}
          title={c.activity_label}
          meta={`${c.activity_date} · ${status(c.status)} · ${c.source === 'makeup' ? '补签' : '正常'}`}
          value={c.status === 'approved' ? signed((c.awarded_points || 0) + (c.streak_bonus || 0)) : status(c.status)}
          tone={c.status === 'approved' ? 'positive' : c.status}
          badge={c.source === 'makeup' ? '补签' : null}
        />
      ))}
    </div>
  );
}

export function RedemptionList({ items }) {
  const list = asArray(items);
  if (!list.length) return <EmptyState icon={<Gift />} title="还没有兑换记录" text="攒够积分后，可以在这里看到兑换进展" />;
  return (
    <div className="record-list timeline-list">
      {list.map((r) => (
        <RecordRow
          key={r.id}
          icon={<Gift />}
          title={r.reward_name}
          meta={status(r.status)}
          value={`-${r.cost_at_time}`}
          tone={r.status === 'fulfilled' ? 'negative' : r.status}
        />
      ))}
    </div>
  );
}

export function TransactionList({ items }) {
  const list = asArray(items);
  if (!list.length) return <EmptyState icon={<Coins />} title="还没有积分流水" text="通过审核、兑换奖励和手工调分都会记录在这里" />;
  return (
    <div className="record-list timeline-list">
      {list.map((t) => (
        <RecordRow
          key={t.id}
          icon={t.change > 0 ? <Plus /> : <ChevronRight />}
          title={t.reason}
          meta={`${formatTime(t.created_at)} · ${sourceLabel(t.source_type)}`}
          value={signed(t.change)}
          tone={t.change > 0 ? 'positive' : 'negative'}
        />
      ))}
    </div>
  );
}

export function AdminCheckins({ items, empty, onApprove, onReject, onReverse }) {
  const list = asArray(items);
  if (!list.length) return <EmptyState icon={<ClipboardCheck />} title={empty} text="新的打卡提交会出现在这里" />;
  return (
    <div className="admin-list">
      {list.map((c) => (
        <div key={c.id} className="admin-item">
          <span className="record-icon">{iconNode(c.activity_icon)}</span>
          <div className="admin-item-main">
            <div className="item-title-line"><strong>{c.activity_label}</strong><StatusBadge value={c.status} /></div>
            <small>{c.user_name} · {c.activity_date} · {c.source === 'makeup' ? '补签' : '正常'} · {scoreText({ score_mode: c.score_mode, base_points: c.base_points })}</small>
          </div>
          <div className="row-actions">
            {c.status === 'pending' && (
              <>
                <button type="button" onClick={() => onApprove(c)}><Check size={16} />通过</button>
                <button type="button" className="danger" onClick={() => onReject(c)}><X size={16} />驳回</button>
              </>
            )}
            {c.status === 'approved' && (
              <button type="button" className="danger" onClick={() => onReverse(c)}><RotateCcw size={16} />撤回</button>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

export function AdminRedemptions({ items, onApprove, onReject }) {
  const list = asArray(items);
  if (!list.length) return <EmptyState icon={<Gift />} title="暂无待审批兑换" text="孩子提交兑换后会出现在这里" />;
  return (
    <div className="admin-list">
      {list.map((r) => (
        <div key={r.id} className="admin-item">
          <span className="record-icon"><Gift /></span>
          <div className="admin-item-main">
            <div className="item-title-line"><strong>{r.reward_name}</strong><StatusBadge value={r.status} /></div>
            <small>{r.user_name} · {r.cost_at_time} 分</small>
          </div>
          <div className="row-actions">
            <button type="button" onClick={() => onApprove(r)}><Check size={16} />通过</button>
            <button type="button" className="danger" onClick={() => onReject(r)}><X size={16} />驳回</button>
          </div>
        </div>
      ))}
    </div>
  );
}

export function ReviewModal({ checkin, onClose, onSubmit }) {
  const [level, setLevel] = useState('pass');
  const [minutes, setMinutes] = useState(30);
  const [counts, setCounts] = useState(true);
  const [note, setNote] = useState('');
  return (
    <Modal title="通过打卡" onClose={onClose}>
      <div className="review-target">
        <span>{iconNode(checkin.activity_icon)}</span>
        <div>
          <strong>{checkin.activity_label}</strong>
          <small>{checkin.activity_date} · {checkin.source === 'makeup' ? '补签' : '正常打卡'} · {scoreText({ score_mode: checkin.score_mode, base_points: checkin.base_points })}</small>
        </div>
      </div>
      {checkin.score_mode === 'quality' && (
        <div className="field">
          <label>质量档位</label>
          <Segmented value={level} onChange={setLevel} options={[['pass', '及格'], ['good', '良好'], ['excellent', '优秀']]} />
        </div>
      )}
      {checkin.score_mode === 'duration' && (
        <div className="field">
          <label>时长分钟</label>
          <input type="number" min="10" step="10" value={minutes} onChange={(e) => setMinutes(Number(e.target.value))} />
        </div>
      )}
      <label className="checkbox-line">
        <input type="checkbox" checked={counts} onChange={(e) => setCounts(e.target.checked)} />
        计入连续打卡
      </label>
      <div className="field">
        <label>备注</label>
        <input value={note} onChange={(e) => setNote(e.target.value)} placeholder="可选" />
      </div>
      <div className="modal-actions">
        <button type="button" className="secondary" onClick={onClose}>取消</button>
        <button type="button" onClick={() => onSubmit({ review_level: level, review_minutes: minutes, counts_for_streak: counts, note })}>
          <Check size={17} />通过
        </button>
      </div>
    </Modal>
  );
}

export function ReasonModal({ title, action, defaultReason, onClose, onSubmit }) {
  const [reason, setReason] = useState(defaultReason);
  return (
    <Modal title={title} onClose={onClose}>
      <div className="field">
        <label>原因</label>
        <input value={reason} onChange={(e) => setReason(e.target.value)} />
      </div>
      <div className="modal-actions">
        <button type="button" className="secondary" onClick={onClose}>取消</button>
        <button type="button" className="danger" onClick={() => onSubmit(reason)}>{action}</button>
      </div>
    </Modal>
  );
}

function IconPicker({ value, onChange }) {
  const icons = ['book', 'read', 'pen', 'math', 'note', 'voice', 'letters', 'run', 'bag', 'home', 'music', 'star'];
  return (
    <div className="icon-picker">
      {icons.map((ic) => (
        <button key={ic} type="button" className={value === ic ? 'active' : ''} onClick={() => onChange(ic)} title={ic}>
          {iconNode(ic)}
        </button>
      ))}
    </div>
  );
}

export function ActivityForm({ draft, setDraft, onSubmit }) {
  return (
    <form className="config-panel" onSubmit={onSubmit}>
      <div className="panel-head">
        <div>
          <strong>{draft.id ? '编辑打卡项' : '新增打卡项'}</strong>
          <small>设置积分规则、分类和展示图标</small>
        </div>
        {draft.id && <EnabledBadge enabled={draft.enabled} />}
      </div>
      <div className="field"><label>名称</label><input value={draft.label} onChange={(e) => setDraft({ ...draft, label: e.target.value })} required /></div>
      <div className="form-pair">
        <div className="field"><label>基础分</label><input type="number" value={draft.base_points} onChange={(e) => setDraft({ ...draft, base_points: e.target.value })} /></div>
        <div className="field">
          <label>类型</label>
          <select value={draft.score_mode} onChange={(e) => setDraft({ ...draft, score_mode: e.target.value })}>
            <option value="default">默认</option>
            <option value="quality">质量</option>
            <option value="duration">时长</option>
          </select>
        </div>
      </div>
      <div className="form-pair">
        <div className="field"><label>分类</label><input value={draft.category} onChange={(e) => setDraft({ ...draft, category: e.target.value })} /></div>
        <div className="field"><label>排序</label><input type="number" value={draft.sort_order} onChange={(e) => setDraft({ ...draft, sort_order: e.target.value })} /></div>
      </div>
      <div className="field"><label>图标</label><IconPicker value={draft.icon} onChange={(icon) => setDraft({ ...draft, icon })} /></div>
      {draft.id && (
        <label className="checkbox-line">
          <input type="checkbox" checked={draft.enabled} onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })} />
          启用这个打卡项
        </label>
      )}
      <div className="form-actions">
        {draft.id && <button type="button" className="secondary" onClick={() => setDraft(emptyActivity())}>取消编辑</button>}
        <button type="submit"><Check size={17} />{draft.id ? '保存修改' : '新增打卡项'}</button>
      </div>
    </form>
  );
}

export function ActivityTable({ items, onEdit }) {
  const list = asArray(items);
  if (!list.length) return <EmptyState icon={<BookOpen />} title="暂无打卡项" text="先添加几个每天要完成的小任务" />;
  return (
    <div className="admin-list compact-list">
      {list.map((a) => (
        <div key={a.id} className="admin-item">
          <span className="record-icon">{iconNode(a.icon)}</span>
          <div className="admin-item-main">
            <div className="item-title-line">
              <strong>{a.label}</strong>
              <span className={`badge ${a.enabled ? 'approved' : 'reversed'}`}>{a.enabled ? '启用' : '停用'}</span>
            </div>
            <small>{a.category} · {scoreText(a)} · 排序 {a.sort_order}</small>
          </div>
          <button type="button" className="secondary" onClick={() => onEdit(a)}><PenLine size={16} />编辑</button>
        </div>
      ))}
    </div>
  );
}

export function RewardForm({ draft, setDraft, onSubmit }) {
  return (
    <form className="config-panel" onSubmit={onSubmit}>
      <div className="panel-head">
        <div>
          <strong>{draft.id ? '编辑奖励' : '新增奖励'}</strong>
          <small>配置兑换成本、库存和审批方式</small>
        </div>
        {draft.id && <EnabledBadge enabled={draft.enabled} />}
      </div>
      <div className="field"><label>奖励名称</label><input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} required /></div>
      <div className="form-pair">
        <div className="field"><label>积分</label><input type="number" value={draft.cost} onChange={(e) => setDraft({ ...draft, cost: e.target.value })} /></div>
        <div className="field"><label>库存</label><input type="number" value={draft.stock} onChange={(e) => setDraft({ ...draft, stock: e.target.value })} /></div>
      </div>
      <div className="field"><label>描述</label><input value={draft.description} onChange={(e) => setDraft({ ...draft, description: e.target.value })} /></div>
      <label className="checkbox-line">
        <input type="checkbox" checked={draft.auto_approve} onChange={(e) => setDraft({ ...draft, auto_approve: e.target.checked })} />
        自动通过
      </label>
      {draft.id && (
        <label className="checkbox-line">
          <input type="checkbox" checked={draft.enabled} onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })} />
          启用这个奖励
        </label>
      )}
      <div className="form-actions">
        {draft.id && <button type="button" className="secondary" onClick={() => setDraft(emptyReward())}>取消编辑</button>}
        <button type="submit"><Check size={17} />{draft.id ? '保存修改' : '新增奖励'}</button>
      </div>
    </form>
  );
}

export function RewardTable({ items, onEdit }) {
  const list = asArray(items);
  if (!list.length) return <EmptyState icon={<Gift />} title="暂无奖励" text="添加奖励后，孩子端会出现兑换货架" />;
  return (
    <div className="admin-list compact-list">
      {list.map((r) => (
        <div key={r.id} className="admin-item">
          <span className="record-icon"><Gift /></span>
          <div className="admin-item-main">
            <div className="item-title-line">
              <strong>{r.name}</strong>
              <span className={`badge ${r.enabled ? 'approved' : 'reversed'}`}>{r.enabled ? '启用' : '停用'}</span>
            </div>
            <small>{r.cost} 分 · {r.auto_approve ? '自动通过' : '需审批'} · 库存 {r.stock < 0 ? '无限' : r.stock}</small>
          </div>
          <button type="button" className="secondary" onClick={() => onEdit(r)}><PenLine size={16} />编辑</button>
        </div>
      ))}
    </div>
  );
}
