const API = '/api/v1';

const CHILD_KEY = 'scorecard-child-token';
const PARENT_KEY = 'scorecard-parent-token';
const LEGACY_KEY = 'scorecard-token';

export function getChildToken() {
  return sessionStorage.getItem(CHILD_KEY) || '';
}

export function getParentToken() {
  return sessionStorage.getItem(PARENT_KEY) || sessionStorage.getItem(LEGACY_KEY) || '';
}

/** Prefer parent token on admin pages, child token on child pages. */
export function authToken(prefer = 'auto') {
  if (prefer === 'parent') return getParentToken() || getChildToken();
  if (prefer === 'child') return getChildToken() || getParentToken();
  if (typeof location !== 'undefined' && location.pathname.startsWith('/admin')) {
    return getParentToken() || getChildToken();
  }
  return getChildToken() || getParentToken();
}

export function setParentToken(token) {
  sessionStorage.setItem(PARENT_KEY, token);
  sessionStorage.setItem(LEGACY_KEY, token);
}

export function setChildToken(token) {
  sessionStorage.setItem(CHILD_KEY, token);
  localStorage.removeItem('scorecard-child-ok');
}

export function clearParentSession() {
  sessionStorage.removeItem(PARENT_KEY);
  sessionStorage.removeItem(LEGACY_KEY);
}

export function clearChildSession() {
  sessionStorage.removeItem(CHILD_KEY);
  localStorage.removeItem('scorecard-child-ok');
}

export function hasChildSession() {
  return Boolean(getChildToken());
}

export function hasParentSession() {
  return Boolean(getParentToken());
}

export class ApiError extends Error {
  constructor(message, status) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export async function request(path, options = {}) {
  const prefer = options.prefer;
  const token = authToken(prefer);
  const headers = {
    'Content-Type': 'application/json',
    ...(token ? { 'X-Auth-Token': token } : {}),
    ...(options.headers || {}),
  };
  const res = await fetch(API + path, {
    method: options.method || 'GET',
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (res.status === 401) {
      if (prefer === 'parent' || path.startsWith('/admin') || location.pathname.startsWith('/admin')) {
        clearParentSession();
      } else {
        clearChildSession();
      }
    }
    throw new ApiError(data.error || '请求失败', res.status);
  }
  return data;
}

export const api = {
  childLogin: (pin) => request('/auth/child/login', { method: 'POST', body: { pin } }),
  parentLogin: (pin) => request('/auth/parent/login', { method: 'POST', body: { pin } }),
  logout: () => request('/auth/logout', { method: 'POST' }),
  summary: () => request('/children/1/summary'),
  activities: () => request('/activities'),
  checkins: () => request('/checkins'),
  createCheckin: (body) => request('/checkins', { method: 'POST', body }),
  cancelCheckin: (id) => request(`/checkins/${id}/cancel`, { method: 'POST' }),
  transactions: () => request('/transactions'),
  rewards: () => request('/rewards'),
  redemptions: () => request('/redemptions'),
  createRedemption: (body) => request('/redemptions', { method: 'POST', body }),
  adminCheckins: () => request('/admin/checkins', { prefer: 'parent' }),
  adminActivities: () => request('/admin/activities', { prefer: 'parent' }),
  adminRewards: () => request('/admin/rewards', { prefer: 'parent' }),
  adminRedemptions: () => request('/admin/redemptions', { prefer: 'parent' }),
  approveCheckin: (id, body) => request(`/admin/checkins/${id}/approve`, { method: 'POST', body, prefer: 'parent' }),
  rejectCheckin: (id, body) => request(`/admin/checkins/${id}/reject`, { method: 'POST', body, prefer: 'parent' }),
  reverseCheckin: (id, body) => request(`/admin/checkins/${id}/reverse`, { method: 'POST', body, prefer: 'parent' }),
  saveActivity: (body) => request(body.id ? `/admin/activities/${body.id}` : '/admin/activities', {
    method: body.id ? 'PUT' : 'POST',
    body,
    prefer: 'parent',
  }),
  deleteActivity: (id) => request(`/admin/activities/${id}`, { method: 'DELETE', prefer: 'parent' }),
  restoreActivity: (id) => request(`/admin/activities/${id}/restore`, { method: 'POST', prefer: 'parent' }),
  saveReward: (body) => request(body.id ? `/admin/rewards/${body.id}` : '/admin/rewards', {
    method: body.id ? 'PUT' : 'POST',
    body,
    prefer: 'parent',
  }),
  deleteReward: (id) => request(`/admin/rewards/${id}`, { method: 'DELETE', prefer: 'parent' }),
  restoreReward: (id) => request(`/admin/rewards/${id}/restore`, { method: 'POST', prefer: 'parent' }),
  adjust: (body) => request('/admin/adjustments', { method: 'POST', body, prefer: 'parent' }),
  approveRedemption: (id, body = {}) => request(`/admin/redemptions/${id}/approve`, { method: 'POST', body, prefer: 'parent' }),
  rejectRedemption: (id, body = {}) => request(`/admin/redemptions/${id}/reject`, { method: 'POST', body, prefer: 'parent' }),
};
