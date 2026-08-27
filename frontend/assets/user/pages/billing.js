// 用量账单页
import { whenReady } from '../state.js';
import { api, esc } from '../api.js';
import { $, safe, injectIcons } from '../ui.js';

const MD = window.MD;

function fmtRates(n) {
  const v = parseFloat(n);
  if (isNaN(v)) return '—';
  v = Math.round(v * 100000) / 100000;
  return String(v).replace(/(\.\d*?)0+$/, '$1').replace(/\.$/, '');
}
function fmtCost6(n) {
  const v = parseFloat(n);
  if (isNaN(v)) return '¥0.000000';
  return '¥' + v.toFixed(6).replace(/(\.\d*?)0+$/, '$1');
}
function fmtShortDate(iso) {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return esc(iso || '');
  const p = (x) => (x < 10 ? '0' : '') + x;
  return p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
}

function loadBilling(page) {
  const tbody = $('billing-tbody');
  tbody.innerHTML = MD.loadingRow(6);
  safe(api.get('/user/billing-records?page=' + page + '&size=10')).then((res) => {
    if (!res) { tbody.innerHTML = ''; return; }
    const d = res.data || {}, items = d.items || [];
    $('billing-total').textContent = '共 ' + (d.total || 0) + ' 条';
    $('b-m-calls').innerHTML = MD.fmtNum(d.total || 0) + '<small>次</small>';
    MD.renderPager($('billing-pager'), page, d.total || 0, 10, loadBilling);

    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="8" style="padding:0;border:0;">' +
        MD.emptyState('chart', '暂无账单记录', '发起 API 调用后，消费明细将展示在这里') + '</td></tr>';
      return;
    }
    tbody.innerHTML = items.map((r) => {
      const typeTag = r.billing_type === 'subscription'
        ? '<span class="tag tag--success">套餐抵扣</span>'
        : '<span class="tag tag--info">按量计费</span>';
      const cacheInfo = [];
      if (r.tokens_cached > 0) cacheInfo.push('命中 ' + MD.fmtNum(r.tokens_cached));
      if (r.tokens_cache_write > 0) cacheInfo.push('写入 ' + MD.fmtNum(r.tokens_cache_write));
      const cacheCell = cacheInfo.length
        ? '<span class="td-sub" title="缓存读按输入价10%、缓存写按输入价125%计费">' + cacheInfo.join(' · ') + '</span>'
        : '<span class="td-sub">—</span>';
      return '<tr>' +
        '<td><span class="tag-inline">' + esc(r.model) + '</span></td>' +
        '<td class="num">' + MD.fmtNum(r.tokens_in) + '</td>' +
        '<td class="num">' + MD.fmtNum(r.tokens_out) + '</td>' +
        '<td class="num">' + cacheCell + '</td>' +
        '<td class="num">¥' + MD.fmtMoney(r.cost) + '</td>' +
        '<td>' + typeTag + '</td>' +
        '<td>' + MD.fmtDate(r.created_at) + '</td>' +
        '<td><button class="btn-ghost btn-sm" onclick="openUsageDetail(' + r.id + ')">明细</button></td></tr>';
    }).join('');
  });

  safe(api.get('/user/usage')).then((res) => {
    if (!res) return;
    $('b-m-tokens').textContent = MD.fmtNum(res.data.total_tokens);
    $('b-m-cost').textContent = MD.fmtMoney(res.data.total_cost);
  });
}

export function openUsageDetail(id) {
  MD.openModal({
    title: '用量明细',
    wide: '560px',
    body: '<div style="text-align:center;padding:30px;color:var(--text-3)"><span class="spinner"></span>加载中…</div>',
    buttons: [{ label: '关闭', cls: 'btn-ghost', onClick: MD.closeModal }]
  });
  safe(api.get('/user/billing-records/' + id)).then((res) => {
    const r = res.data || {};
    let detail = null;
    try { detail = r.detail ? JSON.parse(r.detail) : null; } catch (e) { detail = null; }
    const inTotal = r.tokens_in || 0, outTotal = r.tokens_out || 0;
    const cached = r.cached_tokens || 0, cw = r.tokens_cache_write || 0;
    const uncachedIn = Math.max(0, inTotal - cached - cw);
    const lines = (detail && detail.lines) || [];
    const lineHtml = lines.map((l) => {
      const label = l.key === 'input' ? '输入计费' : l.key === 'cache_read' ? '缓存计费'
        : l.key === 'cache_write' ? '缓存写入计费' : '输出计费';
      return '<div class="fmt-line"><span>' + label + '(' + MD.fmtNum(l.tokens) + ' tokens × ' + fmtRates(l.rate * 1000000) + '/M Tokens)</span>' +
        '<span class="mono">' + fmtCost6(l.amount) + '</span></div>';
    }).join('');
    const mult = detail ? parseFloat(detail.multiplier) : 1;
    let discountHtml = '';
    if (detail && mult !== 1 && !isNaN(mult) && parseFloat(detail.discount) > 0) {
      discountHtml = '<div class="fmt-line"><span>折扣(' + (mult * 100).toFixed(1) + '%)</span>' +
        '<span class="mono" style="color:var(--c-success);">-' + fmtCost6(detail.discount).replace('¥', '¥') + '</span></div>';
    }
    const formulaHtml = detail && lines.length
      ? '<div class="fmt-card"><div class="fmt-title">计费公式</div>' +
        lineHtml +
        '<div class="fmt-line fmt-line--sub"><span>小计</span><span class="mono">' + fmtCost6(detail.subtotal) + '</span></div>' +
        discountHtml +
        '<div class="fmt-line fmt-total"><span>计费金额</span><span class="mono">' + fmtCost6(detail.final || r.cost) + '</span></div></div>'
      : '<div class="fmt-card"><div class="fmt-title">计费</div>' +
        '<div class="fmt-line fmt-total"><span>计费金额</span><span class="mono">' + fmtCost6(r.cost) + '</span></div></div>';
    const tokenCells = (label, count, key) => {
      const amt = detail && detail.lines ? (detail.lines.filter((l) => l.key === key)[0] || {}).amount : null;
      return '<div class="fmt-line"><span>' + label + '</span>' +
        '<span class="mono">' + MD.fmtNum(count) + (amt != null ? ' · ' + fmtCost6(amt) : '') + '</span></div>';
    };
    const cacheWriteCell = cw > 0 ? tokenCells('缓存写入', cw, 'cache_write') : '';
    const tokenCard =
      '<div class="fmt-card"><div class="fmt-title">Token 用量</div>' +
      tokenCells('输入', uncachedIn, 'input') +
      tokenCells('缓存命中', cached, 'cache_read') +
      cacheWriteCell +
      tokenCells('输出', outTotal, 'output') +
      '<div class="fmt-line fmt-total"><span>合计</span><span class="mono">' + MD.fmtNum(inTotal + outTotal) + ' · ' + fmtCost6(r.cost) + '</span></div>' +
      '</div>';
    const head =
      '<div class="fmt-head"><div class="fmt-date">' + fmtShortDate(r.created_at) + '</div>' +
      '<span class="tag tag--success">成功</span></div>' +
      '<div class="fmt-stat">' +
      '<div><span class="k">请求 ID</span><span class="v mono">' + esc(r.request_id || '—') +
      (r.request_id ? '<button class="btn-link" style="margin-left:6px;font-size:12px;" onclick="MD.copyText(\'' + esc(r.request_id) + '\')">复制</button>' : '') +
      '</span></div>' +
      '<div><span class="k">模型</span><span class="v"><span class="tag-inline">' + esc(r.model) + '</span></span></div>' +
      '</div>' +
      '<div class="fmt-metrics">' +
      '<div class="m"><span class="k">TTFT</span><span class="v">' + (r.ttft_ms ? (r.ttft_ms / 1000).toFixed(1) + 's' : '—') + '</span></div>' +
      '<div class="m"><span class="k">耗时</span><span class="v">' + (r.duration_ms ? (r.duration_ms / 1000).toFixed(1) + 's' : '—') + '</span></div>' +
      '<div class="m"><span class="k">总费用</span><span class="v">' + fmtCost6(r.cost) + '</span></div>' +
      '</div>' +
      (cached > 0 ? '<div class="fmt-cache">带缓存 · 缓存命中按缓存价计费</div>' : '') +
      tokenCard + formulaHtml;
    MD.updateModal({ body: '<div class="usage-detail">' + head + '</div>' });
  }).catch((err) => {
    MD.updateModal({ body: '<div style="padding:20px;color:var(--c-danger)">' + esc(err.message || '加载失败') + '</div>' });
  });
}

whenReady().then(() => { loadBilling(1); injectIcons(); });

window.openUsageDetail = openUsageDetail;
