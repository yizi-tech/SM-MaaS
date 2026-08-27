// 充值页（含 Token 加油包 + 易支付）
import { api, toast, fmtMoney, icon, esc, confirm } from '../api.js';
import { safe } from '../ui.js';
import { refreshBalance, goView } from '../layout.js';
import { loadTokenPackages, updateTokenCreditUI, fmtTokens, purchaseTokenPkg } from './credit.js';

let payMethodsCache = null;
const rechargeState = { amount: 100, custom: false, method: null, epayTxNo: null };

export function initRecharge() {
  refreshBalance();
  loadTokenPackages();
  loadPayConfig();
  const pend = sessionStorage.getItem('mass_pending_pay');
  if (pend) {
    try {
      const p = JSON.parse(pend);
      rechargeState.custom = true;
      const inp = document.getElementById('custom-amount');
      if (inp) inp.value = fmtMoney(p.amount);
    } catch (e) {}
  }
  renderRechargeUI();
}

function renderRechargeUI() {
  const chips = [50, 100, 200, 500];
  let html = chips.map((a) => {
    const active = !rechargeState.custom && rechargeState.amount === a ? ' is-active' : '';
    return '<button class="amount-chip' + active + '" onclick="pickAmount(' + a + ')">¥' + a + '</button>';
  }).join('');
  html += '<button class="amount-chip' + (rechargeState.custom ? ' is-active' : '') + '" onclick="pickAmount(0)">自定义</button>';
  document.getElementById('amount-chips').innerHTML = html;
  document.getElementById('custom-amount-field').style.display = rechargeState.custom ? 'block' : 'none';

  const methods = getPayMethods();
  document.getElementById('pay-methods').innerHTML = methods.length
    ? methods.map((m) => {
      const active = rechargeState.method === m.k ? ' is-active' : '';
      return '<div class="pay-method' + active + '" onclick="pickPay(\'' + m.k + '\')">' + icon(m.ic, 22) + m.label + '</div>';
    }).join('')
    : '<div class="pay-method" style="cursor:default;opacity:.7;">' + icon('card', 22) + '在线支付暂未启用，请联系管理员</div>';
  updateRechargeSummary();
}

function getPayMethods() {
  const list = [];
  if (payMethodsCache && payMethodsCache.indexOf('epay') >= 0) {
    list.push({ k: 'epay', label: '在线支付（易支付）', ic: 'card' });
  }
  return list;
}

export function loadPayConfig() {
  safe(api.get('/user/payment-config')).then((res) => {
    if (!res) return;
    payMethodsCache = (res.data || {}).methods || null;
    if (!payMethodsCache || payMethodsCache.indexOf('epay') < 0) {
      rechargeState.method = null;
    } else if (rechargeState.method !== 'epay') {
      rechargeState.method = 'epay';
    }
    renderRechargeUI();
  });
}

export function pickAmount(a) {
  if (a === 0) { rechargeState.custom = true; }
  else { rechargeState.custom = false; rechargeState.amount = a; }
  renderRechargeUI();
  if (rechargeState.custom) { const el = document.getElementById('custom-amount'); if (el) el.focus(); }
}

export function pickPay(k) { rechargeState.method = k; renderRechargeUI(); }

export function updateRechargeSummary() {
  const amt = getRechargeAmount();
  document.getElementById('recharge-total').textContent = '¥ ' + (amt > 0 ? fmtMoney(amt) : '0.00');
}

function getRechargeAmount() {
  if (rechargeState.custom) {
    const v = parseFloat((document.getElementById('custom-amount') || {}).value);
    return isNaN(v) ? 0 : v;
  }
  return rechargeState.amount;
}

/* 余额不足自动充值：预设金额跳转充值页，支付到账后自动执行 action（如重新订阅） */
export function startRechargeFor(amount, action) {
  sessionStorage.setItem('mass_pending_pay', JSON.stringify({ amount, action }));
  goView('recharge');
}

export function doRecharge() {
  const amount = getRechargeAmount();
  if (!(amount >= 1)) { toast('请选择或输入有效的充值金额（≥ ¥1.00）', 'warn'); return; }
  if (rechargeState.method !== 'epay') { toast('在线支付尚未启用，请联系管理员', 'warn'); return; }
  const btn = document.getElementById('recharge-btn');
  btn.disabled = true; btn.innerHTML = '<span class="spinner"></span>正在发起支付…';
  api.post('/user/recharge/epay', { amount: amount.toFixed(2), pay_type: 'alipay' })
    .then((res) => {
      const d = res.data || {};
      if (!d.pay_url) throw new Error('未获取到支付链接');
      rechargeState.epayTxNo = d.out_trade_no;
      toast('已生成支付订单，请在打开的页面完成支付', 'info');
      window.open(d.pay_url, '_blank', 'noopener');
      pollEpayStatus(d.out_trade_no, amount);
    })
    .catch((e) => toast(e.message || '发起支付失败', 'error'))
    .finally(() => {
      btn.disabled = false;
      btn.innerHTML = icon('coin', 16) + '<span>确认充值</span>';
    });
}

/* 易支付订单轮询：每 2.5s 查询一次本地状态，最多 40 次（约 100 秒） */
function pollEpayStatus(outTradeNo, amount) {
  let tries = 0;
  const timer = setInterval(() => {
    tries++;
    if (tries > 40) { clearInterval(timer); toast('支付确认超时，可稍后点击「查询支付结果」', 'warn'); return; }
    api.get('/user/recharge/epay/status?out_trade_no=' + encodeURIComponent(outTradeNo)).then((res) => {
      const status = (res.data || {}).status;
      if (status === 'success') {
        clearInterval(timer);
        toast('支付成功，¥' + fmtMoney(amount) + ' 已到账', 'success');
        refreshBalance();
        afterPaid();
        return;
      }
      if (status === 'cancelled') {
        clearInterval(timer);
        toast('订单已超时(30 分钟未支付)，已自动取消', 'warn');
        sessionStorage.removeItem('mass_pending_pay');
        return;
      }
      if (tries === 5) toast('支付完成后，可点击下方「查询支付结果」确认到账', 'info');
    });
  }, 2500);
}

function afterPaid() {
  const pend = sessionStorage.getItem('mass_pending_pay');
  sessionStorage.removeItem('mass_pending_pay');
  let action = null;
  if (pend) { try { action = JSON.parse(pend).action; } catch (e) {} }
  if (action && action.action === 'subscribe') {
    sessionStorage.setItem('mass_resume', JSON.stringify(action));
    goView('plans');
  }
}

/* 手动向易支付网关查询订单结果（防丢单） */
export function queryEpayResult() {
  if (!rechargeState.epayTxNo) { toast('请先发起一笔易支付充值，再查询结果', 'warn'); return; }
  api.post('/user/recharge/epay/query', { out_trade_no: rechargeState.epayTxNo })
    .then((res) => {
      const status = (res.data || {}).status;
      if (status === 'success') {
        toast('支付成功，已到账', 'success');
        refreshBalance();
        afterPaid();
      } else {
        toast('订单尚未支付，请完成支付后重试', 'info');
      }
    })
    .catch((e) => toast(e.message || '查询失败', 'error'));
}

window.pickAmount = pickAmount;
window.pickPay = pickPay;
window.updateRechargeSummary = updateRechargeSummary;
window.doRecharge = doRecharge;
window.queryEpayResult = queryEpayResult;

import { onUserReady } from '../main.js';
onUserReady(initRecharge);
