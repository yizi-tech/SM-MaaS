// 交易记录页
import { whenReady, TX_TYPE, TX_STATUS, PAY_LABEL } from '../state.js';
import { api, esc } from '../api.js';
import { $, safe, injectIcons } from '../ui.js';

const MD = window.MD;

function loadTx(page) {
  const tbody = $('tx-tbody');
  tbody.innerHTML = MD.loadingRow(7);
  safe(api.get('/user/transactions?page=' + page + '&size=10')).then((res) => {
    if (!res) { tbody.innerHTML = ''; return; }
    const d = res.data || {}, items = d.items || [];
    $('tx-total').textContent = '共 ' + (d.total || 0) + ' 条';
    MD.renderPager($('tx-pager'), page, d.total || 0, 10, loadTx);

    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="7" style="padding:0;border:0;">' +
        MD.emptyState('list', '暂无交易记录', '充值或消费后，资金流水将展示在这里') + '</td></tr>';
      return;
    }
    tbody.innerHTML = items.map((t) => {
      const tp = TX_TYPE[t.type] || [t.type, 'tag--gray'];
      const st = TX_STATUS[t.status] || [t.status, 'tag--gray'];
      const positive = (t.balance_after != null && t.balance_before != null)
        ? parseFloat(t.balance_after) > parseFloat(t.balance_before)
        : (t.type === 'recharge' || t.type === 'refund');
      const sign = positive ? '+' : '-';
      const amtColor = positive ? 'var(--c-success)' : 'var(--text-1)';
      return '<tr>' +
        '<td class="mono">' + esc(t.transaction_no) + '</td>' +
        '<td><span class="tag ' + tp[1] + '">' + tp[0] + '</span></td>' +
        '<td class="num" style="color:' + amtColor + ';font-weight:600;">' + sign + '¥' + MD.fmtMoney(t.amount) + '</td>' +
        '<td class="mono" style="color:var(--text-3);font-size:12px;">¥' + MD.fmtMoney(t.balance_before) + ' → ¥' + MD.fmtMoney(t.balance_after) + '</td>' +
        '<td>' + (PAY_LABEL[t.payment_method] || '—') + '</td>' +
        '<td><span class="tag ' + st[1] + '">' + st[0] + '</span></td>' +
        '<td title="' + esc(t.description || '') + '">' + MD.fmtDate(t.created_at) + '</td></tr>';
    }).join('');
  });
}

whenReady().then(() => { loadTx(1); injectIcons(); });
