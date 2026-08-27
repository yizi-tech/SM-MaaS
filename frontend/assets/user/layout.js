// 共享布局：侧边栏 + 顶栏注入、路由跳转、余额/通知
import { VIEW_TITLES, currentUser, balance, setUser, markReady, DP_KEY } from './state.js';
import { api, toast, esc, fmtMoney, todayText, icon } from './api.js';
import { $, safe, injectIcons } from './ui.js';
import { logout, clearToken } from './auth.js';

const NAV = `
<div class="brand">
  <div style="text-align:center;display:grid;place-items:center;"><img src="/assets/yz-1.png" width="48"></div>
  <div class="brand__name">MAAS<small>LLM API GATEWAY</small></div>
</div>
<nav class="nav">
  <div class="nav__group">
    <div class="nav__group-title">${icon('grid', 14)}<span>工作台</span></div>
    <a class="nav__item" data-view="dashboard">${icon('grid', 17)}<span>概览</span></a>
  </div>
  <div class="nav__group">
    <div class="nav__group-title">${icon('cpu', 14)}<span>平台能力</span></div>
    <a class="nav__item nav__item--sub" data-view="models"><span>模型广场</span></a>
    <a class="nav__item nav__item--sub" data-view="chat"><span>对话测试</span></a>
    <a class="nav__item nav__item--sub" data-view="apidocs"><span>API 对接</span></a>
  </div>
  <div class="nav__group">
    <div class="nav__group-title">${icon('cube', 14)}<span>资产管理</span></div>
    <a class="nav__item nav__item--sub" data-view="keys"><span>API Keys</span></a>
    <a class="nav__item nav__item--sub" data-view="billing"><span>用量账单</span></a>
    <a class="nav__item nav__item--sub" data-view="transactions"><span>交易记录</span></a>
  </div>
  <div class="nav__group">
    <div class="nav__group-title">${icon('coin', 14)}<span>财务</span></div>
    <a class="nav__item nav__item--sub" data-view="plans"><span>订阅套餐</span></a>
    <a class="nav__item nav__item--sub" data-view="coupons"><span>重置券</span></a>
    <a class="nav__item nav__item--sub" data-view="invoices"><span>发票</span></a>
    <a class="nav__item nav__item--sub" data-view="recharge"><span>充值</span></a>
  </div>
  <div class="nav__group">
    <div class="nav__group-title">${icon('user', 14)}<span>账号</span></div>
    <a class="nav__item nav__item--sub" data-view="profile"><span>个人设置</span></a>
    <a class="nav__item nav__item--sub" data-view="verify"><span>实名认证</span></a>
    <a class="nav__item nav__item--sub" data-action="logout" style="margin-top:6px;border-top:1px dashed var(--border-2);border-radius:0 0 10px 10px;">${icon('logout', 14)}<span>退出登录</span></a>
  </div>
</nav>
<div class="side__balance">
  <div class="k">账户余额</div>
  <div class="v" id="side-balance">¥ 0.00</div>
  <button class="btn-primary btn-sm" style="width:100%;margin-top:10px;" data-view="recharge">${icon('plus', 13)}<span>充值</span></button>
</div>`;

const TOPBAR = `
<button class="hamburger" data-action="openSide" aria-label="打开菜单">${icon('menu', 20)}</button>
<div class="breadcrumb" id="breadcrumb">控制台 / <b>概览</b></div>
<div class="search">${icon('search', 16)}<input type="text" id="top-search" placeholder="搜索功能，如：密钥 / 账单 / 充值…"></div>
<div class="header__right">
  <div class="notif-wrap" id="notif-wrap">
    <button class="icon-btn" id="hd-bell" data-action="toggleNotif" aria-label="通知">${icon('bell', 18)}<span class="dot" id="hd-bell-dot" style="display:none"></span></button>
    <div class="notif-panel" id="notif-panel">
      <div class="notif-panel__head"><b>通知</b><button class="link-btn" data-action="markAllNotif">全部已读</button></div>
      <div class="notif-panel__list" id="notif-panel-list"></div>
      <div class="notif-panel__foot" data-view="notifications">查看全部通知</div>
    </div>
  </div>
  <button class="btn-primary" data-view="keys" data-after="new">${icon('plus', 15)}<span>新建 Key</span></button>
  <div class="avatar" id="header-avatar" title="个人设置" style="cursor:pointer" data-view="profile">M</div>
</div>`;

export function goView(v, after) {
  if (!VIEW_TITLES[v]) return;
  let url = '/user/' + v + '.html';
  if (after) url += '?after=' + encodeURIComponent(after);
  location.href = url;
}
window.goView = goView;

export function setActive() {
  const v = document.body.dataset.page || 'dashboard';
  document.querySelectorAll('.nav__item[data-view]').forEach((a) => a.classList.toggle('is-active', a.dataset.view === v));
  const bc = document.getElementById('breadcrumb');
  if (bc) bc.innerHTML = '控制台 / <b>' + (VIEW_TITLES[v] || '') + '</b>';
}

export function updateBalanceUI() {
  const b = fmtMoney(balance);
  const sb = document.getElementById('side-balance'); if (sb) sb.textContent = '¥ ' + b;
  const hb = document.getElementById('hero-balance'); if (hb) hb.textContent = b;
  const rb = document.getElementById('recharge-balance'); if (rb) rb.textContent = b;
}

export function renderUserChrome() {
  if (!currentUser) return;
  const name = currentUser.nickname || currentUser.email || 'M';
  const av = document.getElementById('header-avatar');
  if (av) {
    if (currentUser.avatar) av.innerHTML = '<img src="' + esc(currentUser.avatar) + '" style="width:100%;height:100%;border-radius:50%;object-fit:cover;">';
    else { av.innerHTML = ''; av.textContent = name.slice(0, 1).toUpperCase(); }
  }
  const wt = document.getElementById('welcome-title'); if (wt) wt.textContent = '欢迎回来，' + name + ' 👋';
  const ts = document.getElementById('today-sub'); if (ts) ts.textContent = todayText() + ' · 系统运行正常';
  updateBalanceUI();
}

function quickSearch(q) {
  q = (q || '').trim().toLowerCase();
  const map = {
    '密钥': 'keys', 'key': 'keys', 'keys': 'keys', '账单': 'billing', '用量': 'billing',
    '交易': 'transactions', '记录': 'transactions', '订阅': 'plans', '套餐': 'plans',
    '充值': 'recharge', '余额': 'recharge', '发票': 'invoices', '设置': 'profile', '个人': 'profile',
    '实名': 'verify', '模型': 'models', '模型广场': 'models', '对接': 'apidocs', 'api': 'apidocs',
    '对话': 'chat', '通知': 'notifications', '优惠券': 'coupons', '重置券': 'coupons', '反馈': 'feedback'
  };
  let target = null;
  for (const k in map) { if (q.includes(k)) { target = map[k]; break; } }
  if (target) goView(target);
  else toast('未找到匹配功能', 'info');
}

function toggleNotif() {
  const p = document.getElementById('notif-panel');
  if (!p) return;
  if (p.classList.toggle('is-open')) loadNotifPreview();
}
async function loadNotifPreview() {
  try {
    const res = await api.get('/user/notifications?page=1&size=6');
    const list = document.getElementById('notif-panel-list');
    if (!list) return;
    const items = (res.data && res.data.items) || [];
    list.innerHTML = items.length
      ? items.map((n) => '<div class="notif-item' + (n.read ? '' : ' is-unread') + '"><div class="notif-item__t">' + esc(n.title) + '</div><div class="notif-item__c">' + esc(n.content || '') + '</div></div>').join('')
      : '<div class="notif-empty">暂无通知</div>';
  } catch (e) { /* ignore */ }
}
function markAllNotif() {
  safe(api.post('/user/notifications/read-all')).then(() => {
    const dot = document.getElementById('hd-bell-dot'); if (dot) dot.style.display = 'none';
    loadNotifPreview();
  });
}
async function loadNotifBadge() {
  try {
    const res = await api.get('/user/notifications?page=1&size=1');
    const d = res.data || {};
    const unread = d.unread || 0;
    const dot = document.getElementById('hd-bell-dot');
    if (dot) dot.style.display = unread > 0 ? 'block' : 'none';
  } catch (e) { /* ignore */ }
}

function wireChrome() {
  document.body.addEventListener('click', (e) => {
    const act = e.target.closest('[data-action]');
    if (act) {
      const a = act.dataset.action;
      if (a === 'logout') logout();
      else if (a === 'openSide') document.body.classList.add('side-open');
      else if (a === 'toggleNotif') toggleNotif();
      else if (a === 'markAllNotif') markAllNotif();
      return;
    }
    const nav = e.target.closest('[data-view]');
    if (nav) { e.preventDefault(); goView(nav.dataset.view, nav.dataset.after); }
  });
  const search = document.getElementById('top-search');
  if (search) search.addEventListener('keydown', (e) => { if (e.key === 'Enter') quickSearch(search.value); });
}

export function renderChrome() {
  const chrome = document.getElementById('chrome');
  if (!chrome) return;
  const shell = document.createElement('div'); shell.className = 'app';
  const side = document.createElement('aside'); side.className = 'side'; side.id = 'side'; side.innerHTML = NAV;
  const main = document.createElement('div'); main.className = 'main';
  const header = document.createElement('header'); header.className = 'header'; header.innerHTML = TOPBAR;
  const view = document.getElementById('view');
  main.appendChild(header);
  if (view) main.appendChild(view);
  shell.appendChild(side);
  shell.appendChild(main);
  chrome.replaceWith(shell);
  const scrim = document.createElement('div'); scrim.className = 'scrim'; scrim.id = 'scrim';
  scrim.addEventListener('click', () => document.body.classList.remove('side-open'));
  document.body.appendChild(scrim);
  setActive();
  wireChrome();
}

export function refreshBalance() {
  return safe(api.get('/user/profile')).then((res) => {
    if (res && res.data) {
      setUser(res.data);
      updateBalanceUI();
      if (window.updateTokenCreditUI) window.updateTokenCreditUI();
    }
  });
}

export async function loadUserAndChrome() {
  try {
    const res = await api.get('/user/profile');
    setUser(res.data);
  } catch (e) {
    if (e && e.status === 401) { clearToken(); location.href = '/user/login.html'; return; }
  }
  renderChrome();
  renderUserChrome();
  injectIcons();
  await loadNotifBadge();
  markReady();
}
