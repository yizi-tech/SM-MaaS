// 概览页
import { whenReady, currentUser } from '../state.js';
import { api, esc, fmtMoney, fmtNum, toast } from '../api.js';
import { $, safe, renderAxis, dailySeries, renderCostChart, renderTokenBars, injectIcons } from '../ui.js';
import { checkCreditPopup, renderCreditBanner, renderCreditCard } from './credit.js';

function renderDashSubs(subs) {
  const box = $('dash-subs');
  if (!box) return;
  let html = '';
  const credits = currentUser ? (currentUser.token_credits || 0) : 0;
  const creditUsed = currentUser ? (currentUser.credit_used || 0) : 0;
  if (credits > 0 || creditUsed > 0) {
    const left = Math.max(0, credits - creditUsed);
    const leftPct = credits > 0 ? Math.max(0, (credits - creditUsed) / credits * 100) : 0;
    const pctText = leftPct >= 100 ? '100.0' : leftPct.toFixed(2);
    const fillCls = leftPct < 20 ? ' bar__fill--warn' : '';
    html += '<div class="plan">' +
      '<div class="plan__row"><span class="plan__name">Token 加油包额度</span>' +
      '<span class="plan__rate">剩 ' + pctText + '%</span></div>' +
      '<div class="bar"><div class="bar__fill' + fillCls + '" style="width:' + leftPct + '%"></div></div>' +
      '<div class="plan__row" style="margin-top:6px;"><span class="plan__meta">' + fmtNum(credits) + ' Tokens</span>' +
      '<span class="plan__meta">已用 ' + fmtNum(creditUsed) + '</span></div></div>';
  }
  if (subs.length) {
    html += subs.map((s) => {
      const inc = Number(s.included_tokens) || 0, used = Number(s.used_tokens) || 0;
      const leftPct = inc > 0 ? Math.max(0, (inc - used) / inc * 100) : 0;
      const pctText = leftPct >= 100 ? '100.0' : leftPct.toFixed(2);
      const fillCls = leftPct < 20 ? ' bar__fill--warn' : '';
      return '<div class="plan">' +
        '<div class="plan__row"><span class="plan__name">' + esc(s.plan_name) + ' 套餐</span>' +
        '<span class="plan__rate">剩 ' + pctText + '%</span></div>' +
        '<div class="bar"><div class="bar__fill' + fillCls + '" style="width:' + leftPct + '%"></div></div>' +
        '<div class="plan__row" style="margin-top:6px;"><span class="plan__meta">' + fmtNum(inc) + ' Tokens</span>' +
        '<span class="plan__meta">已用 ' + fmtNum(used) + '</span></div></div>';
    }).join('');
  }
  if (!html) {
    html = '<div class="plan"><div class="plan__row">' +
      '<span class="plan__name">暂无订阅套餐</span>' +
      '<a class="btn-link" onclick="goView(\'plans\')">去订阅 →</a></div></div>';
  }
  box.innerHTML = html;
}

function loadDashboard() {
  renderAxis($('spark-axis'));
  renderAxis($('bars-axis'));
  checkCreditPopup();
  renderCreditBanner();
  renderCreditCard();

  const d7 = new Date(); d7.setHours(0, 0, 0, 0); d7.setDate(d7.getDate() - 6);
  safe(api.get('/user/usage?start=' + encodeURIComponent(d7.toISOString()))).then((res) => {
    if (!res) return;
    $('spark-total').textContent = fmtMoney(res.data.total_cost);
    $('token-total').textContent = fmtNum(res.data.total_tokens);
    renderCostChart(dailySeries(res.data.daily, 'cost'));
    renderTokenBars(dailySeries(res.data.daily, 'tokens'));
  });

  safe(api.get('/user/subscriptions')).then((res) => {
    if (!res) return;
    renderDashSubs(res.data || []);
  });

}

whenReady().then(() => { loadDashboard(); injectIcons(); });
