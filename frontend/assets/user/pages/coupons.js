// 重置券
import { api, toast, esc } from '../api.js';
import { safe, loadingRow } from '../ui.js';

export function initCoupons() {
  loadCoupons();
}

export function loadCoupons() {
  const tbody = document.getElementById('coupons-tbody');
  tbody.innerHTML = loadingRow(5);
  safe(api.get('/user/reset-coupons')).then((res) => {
    if (!res) { tbody.innerHTML = ''; return; }
    const items = res.data || [];
    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="5" style="padding:0;border:0;">' +
        emptyStateCard('暂无重置券', '平台发放的重置券会展示在这里，用于恢复订阅已用额度') + '</td></tr>';
      return;
    }
    tbody.innerHTML = items.map((c) => {
      const used = c.status === 'used';
      const statusTag = used ? '<span class="tag tag--gray">已使用</span>' : '<span class="tag tag--success">未使用</span>';
      const btn = used
        ? '<button class="btn-ghost btn-sm" disabled style="opacity:.5;cursor:not-allowed;">已兑换</button>'
        : '<button class="btn-primary btn-sm" onclick="redeemCoupon(' + c.id + ')">立即兑换</button>';
      return '<tr>' +
        '<td class="mono">' + esc(c.code) + '</td>' +
        '<td>' + statusTag + '</td>' +
        '<td><span class="td-sub">' + esc(c.note || '-') + '</span></td>' +
        '<td>' + window.MD.fmtDate(c.created_at) + '</td>' +
        '<td>' + btn + '</td></tr>';
    }).join('');
  });
}

export function redeemCoupon(id) {
  window.MD.confirm('确定兑换该重置券吗？<br><span class="hint">兑换后你所有有效订阅的已用额度将立即归零</span>', function () {
    safe(api.post('/user/reset-coupons/' + id + '/redeem')).then((res) => {
      const d = res.data || {};
      const n = d.reset_count || 0;
      toast('兑换成功，已重置 ' + n + ' 个订阅套餐的已用额度', 'success');
      loadCoupons();
    });
  }, { title: '兑换重置券', okLabel: '确认兑换', html: true });
}

function emptyStateCard(title, sub) {
  return '<div style="text-align:center;padding:40px 20px;color:var(--text-3);">' +
    '<div style="font-size:34px;margin-bottom:8px;">🎟️</div><div style="font-weight:600;color:var(--text-2);">' + esc(title) + '</div>' +
    '<div style="font-size:13px;margin-top:6px;">' + esc(sub || '') + '</div></div>';
}

window.redeemCoupon = redeemCoupon;

import { onUserReady } from '../main.js';
onUserReady(initCoupons);
