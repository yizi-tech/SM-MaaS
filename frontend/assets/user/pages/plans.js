// 订阅套餐页
import { currentUser } from '../state.js';
import { api, toast, fmtMoney, fmtNum, icon, esc, confirm } from '../api.js';
import { safe, injectIcons } from '../ui.js';
import { refreshBalance, goView } from '../layout.js';
import { startRechargeFor } from './recharge.js';

let plansCache = [];
let subsCache = [];
let MODELS = [];

export function initPlans() {
  loadModelsList();
  loadPlans();
  const resume = sessionStorage.getItem('mass_resume');
  if (resume) {
    sessionStorage.removeItem('mass_resume');
    try {
      const a = JSON.parse(resume);
      if (a && a.action === 'subscribe') subscribePlan(a.planId);
    } catch (e) {}
  }
}

function loadModelsList() {
  if (MODELS.length) return;
  safe(api.get('/models')).then((res) => {
    if (res && res.data) MODELS = res.data.map((m) => m.id);
  });
}

export function loadPlans() {
  const box = document.getElementById('current-subs');
  box.innerHTML = '<div class="card card--flat" style="text-align:center;padding:30px;color:var(--text-3);font-size:13px;"><span class="spinner"></span>正在加载订阅信息…</div>';
  Promise.all([
    safe(api.get('/plans')),
    safe(api.get('/user/subscriptions'))
  ]).then((rs) => {
    const plans = (rs[0] && rs[0].data) || [];
    const subs = (rs[1] && rs[1].data) || [];
    plansCache = plans;
    subsCache = subs;
    renderCurrentSubs(subs);
    renderPlanGrid(plans, subs);
  });
}

function renderCurrentSubs(subs) {
  const box = document.getElementById('current-subs');
  if (!subs.length) {
    box.innerHTML = '<div class="card card--flat"><div class="card__title">当前订阅</div>' +
      '<p style="font-size:13px;color:var(--text-3);">暂无有效订阅，选择下方套餐即可开通 Token Plan。</p></div>';
    return;
  }
  box.innerHTML = subs.map((s) => {
    const inc = Number(s.included_tokens) || 0, used = Number(s.used_tokens) || 0;
    const usedPct = inc > 0 ? Math.min(100, used / inc * 100) : 0;
    return '<div class="card card--flat" style="margin-bottom:12px;"><div class="sub-item">' +
      '<div class="sub-item__main">' +
      '<div class="sub-item__name">' + esc(s.plan_name) +
      '<span class="tag tag--success">' + icon('check', 12) + '生效中</span>' +
      (s.auto_renew ? '<span class="tag tag--info">自动续费</span>' : '') + '</div>' +
      '<div class="sub-item__meta">有效期 ' + window.MD.fmtDate(s.start_at) + ' ~ ' + window.MD.fmtDate(s.end_at) +
      ' · 已用 ' + fmtNum(used) + ' / ' + fmtNum(inc) + ' Tokens（' + usedPct.toFixed(1) + '%）</div>' +
      '<div class="sub-item__bar"><div class="bar"><div class="bar__fill" style="width:' + usedPct + '%"></div></div></div>' +
      '</div>' +
      '<button class="btn-danger btn-sm" onclick="cancelSub(' + s.id + ')">' + icon('x', 13) + '取消订阅</button>' +
      '</div></div>';
  }).join('');
}

function renderPlanGrid(plans, subs) {
  const grid = document.getElementById('plan-grid');
  if (!plans.length) {
    grid.innerHTML = '<div class="card" style="grid-column:1/-1;">' + emptyState('暂无可订阅的套餐', '请稍后再来看看') + '</div>';
    return;
  }
  let activeSub = null;
  subs.forEach((s) => { if (s.status === 'active' && !activeSub) activeSub = s; });
  const activeName = activeSub ? activeSub.plan_name : '';
  const activeTokens = activeSub ? activeSub.included_tokens : 0;

  grid.innerHTML = plans.map((p, i) => {
    const isCurrent = activeName === p.name;
    const isDowngrade = !!activeSub && p.included_tokens < activeTokens;
    const canUpgrade = !!activeSub && p.included_tokens > activeTokens;
    const cls = 'plan-card' + (i === 1 ? ' is-hot' : '') + (isCurrent ? ' is-current' : '');
    const flag = i === 1 ? '<div class="plan-card__flag">最受欢迎</div>' : '';
    const access = p.model_access || [];
    const modelDesc = (!access.length || access.length >= MODELS.length) ? '全部模型' : access.length + ' 个模型';
    let btn;
    if (isCurrent) {
      btn = '<button class="btn-ghost" style="width:100%;" disabled>' + icon('check', 14) + '<span>当前订阅</span></button>';
    } else if (isDowngrade) {
      btn = '<button class="btn-ghost" style="width:100%;" disabled title="仅支持升级到更高额度的套餐"><span>仅支持升级</span></button>';
    } else if (canUpgrade) {
      btn = '<button class="btn-primary" style="width:100%;" onclick="subscribePlan(' + p.id + ')">' + icon('upload', 14) + '<span>升级订阅</span></button>';
    } else {
      btn = '<button class="btn-primary" style="width:100%;" onclick="subscribePlan(' + p.id + ')">' + icon('card', 14) + '<span>立即订阅</span></button>';
    }
    return '<article class="' + cls + '">' + flag +
      '<div class="plan-card__name">' + esc(p.name) + '</div>' +
      '<div class="plan-card__desc">' + esc(p.description || '') + '</div>' +
      '<div class="plan-card__price">¥' + fmtMoney(p.price) + '<small> / ' + p.duration_days + ' 天</small></div>' +
      '<ul class="plan-card__features">' +
      '<li>' + icon('check', 15) + '包含 ' + fmtNum(p.included_tokens) + ' Tokens</li>' +
      '<li>' + icon('check', 15) + 'RPM ' + fmtNum(p.rpm) + ' 请求 / 分钟</li>' +
      '<li>' + icon('check', 15) + 'TPM ' + fmtNum(p.tpm) + ' Token / 分钟</li>' +
      '<li>' + icon('check', 15) + '并发上限 ' + fmtNum(p.concurrent_limit) + '</li>' +
      '<li>' + icon('check', 15) + '可访问 ' + modelDesc + '</li>' +
      '</ul>' + btn + '</article>';
  }).join('');
  injectIcons();
}

export function subscribePlan(id) {
  const p = plansCache.filter((x) => x.id === id)[0] || {};
  const subs = (subsCache || []).filter((s) => s.status === 'active');
  const s = subs[0];
  if (s && s.plan_name === p.name) { toast('你已订阅该套餐', 'warn'); return; }
  if (s && p.included_tokens < s.included_tokens) { toast('仅支持升级到 token 额度更高的套餐', 'warn'); return; }
  const total = parseFloat(p.price) || 0;
  let pay = total;
  let tip = '';
  if (s && s.included_tokens > 0) {
    const remaining = Math.max(0, s.included_tokens - (s.used_tokens || 0));
    const unit = (parseFloat(s.price) || 0) / s.included_tokens;
    const credit = Math.min(remaining * unit, total);
    pay = Math.max(0, total - credit);
    tip = '<br>剩余 ' + fmtNum(remaining) + ' Tokens 抵扣 <b style="font-family:var(--font-num);">¥' + fmtMoney(credit.toFixed(2)) + '</b>，' +
      (pay > 0 ? '实付 <b style="font-family:var(--font-num);">¥' + fmtMoney(pay.toFixed(2)) + '</b>。' : '本次无需支付余额。');
  }
  confirm('确认' + (s ? '升级为' : '订阅') + '「' + esc(p.name || '') + '」套餐？<br>将从账户余额中扣除 <b style="font-family:var(--font-num);">¥' + fmtMoney(pay.toFixed(2)) + '</b>。' + tip, function () {
    api.post('/user/subscribe', { plan_id: id })
      .then(() => {
        toast('订阅成功，「' + (p.name || '') + '」已生效', 'success');
        refreshBalance();
        loadPlans();
      })
      .catch((e) => {
        if (e.status === 402 && e.data && e.data.need) {
          const need = Math.max(1, Math.ceil(parseFloat(e.data.need) * 100) / 100);
          toast('余额不足，还需 ¥' + fmtMoney(need) + '，自动发起充值', 'info');
          startRechargeFor(need, { action: 'subscribe', planId: id });
          return;
        }
        toast(e.message, 'error');
      });
  }, { title: '订阅确认', okLabel: s ? '确认升级' : '确认订阅', html: true });
}

export function cancelSub(id) {
  const s = subsCache.filter((x) => x.id === id)[0] || {};
  confirm('确定取消「' + esc(s.plan_name || '') + '」订阅吗？<br>取消后到期前仍可使用，但不再自动续费。', function () {
    api.post('/user/subscriptions/' + id + '/cancel')
      .then(() => { toast('订阅已取消', 'success'); loadPlans(); })
      .catch((e) => toast(e.message, 'error'));
  }, { title: '取消订阅', okLabel: '取消订阅', danger: true, html: true });
}

function emptyState(title, sub) {
  return '<div style="text-align:center;padding:40px 20px;color:var(--text-3);">' +
    '<div style="font-size:34px;margin-bottom:8px;">📦</div><div style="font-weight:600;color:var(--text-2);">' + esc(title) + '</div>' +
    '<div style="font-size:13px;margin-top:6px;">' + esc(sub || '') + '</div></div>';
}

window.subscribePlan = subscribePlan;
window.cancelSub = cancelSub;

import { onUserReady } from '../main.js';
onUserReady(initPlans);
