// 发票
import { api, toast, esc } from '../api.js';
import { safe, loadingRow } from '../ui.js';

let invPage = 1;

export function initInvoices() {
  loadInvoices();
}

function loadInvoiceQuota() {
  safe(api.get('/user/invoice-quota')).then((res) => {
    if (!res) return;
    const d = res.data || {};
    document.getElementById('inv-recharged').textContent = '¥ ' + fmt(res.data.recharged);
    document.getElementById('inv-occupied').textContent = '¥ ' + fmt(res.data.occupied);
    document.getElementById('inv-quota').textContent = '¥ ' + fmt(res.data.quota);
  });
}

function fmt(n) { return window.MD.fmtMoney(parseFloat(n || 0)); }

export function loadInvoices() {
  loadInvoiceQuota();
  const tbody = document.getElementById('invoices-tbody');
  tbody.innerHTML = loadingRow(6);
  safe(api.get('/user/invoices?page=' + invPage + '&size=15')).then((res) => {
    if (!res) { tbody.innerHTML = ''; return; }
    const d = res.data || {}, items = d.items || [];
    window.MD.renderPager(document.getElementById('invoices-pager'), invPage, d.total || 0, 15, (p) => { invPage = p; loadInvoices(); });
    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="6" style="padding:0;border:0;">' +
        emptyStateCard('暂无发票记录', '点击右上角「申请开票」，使用充值金额申请发票') + '</td></tr>';
      return;
    }
    tbody.innerHTML = items.map((inv) => {
      const stMap = { pending: ['审核中', 'tag--warn'], issued: ['已开具', 'tag--success'], rejected: ['已驳回', 'tag--danger'] };
      const st = stMap[inv.status] || ['未知', 'tag--gray'];
      const type = (inv.title_type === 'company' ? '企业' : '个人') + ' · ' + (inv.invoice_type === 'vat' ? '专票' : '普票');
      const reason = inv.status === 'rejected' ? '<div class="td-sub">' + esc(inv.reject_reason || '') + '</div>' : '';
      return '<tr>' +
        '<td><b style="color:var(--text-1)">' + esc(inv.title) + '</b>' + reason + '</td>' +
        '<td><span class="td-sub">' + type + '</span></td>' +
        '<td class="num">¥' + window.MD.fmtMoney(inv.amount) + '</td>' +
        '<td><span class="tag ' + st[1] + '">' + st[0] + '</span></td>' +
        '<td><span class="mono">' + esc(inv.invoice_no || '-') + '</span></td>' +
        '<td>' + window.MD.fmtDate(inv.created_at) + '</td></tr>';
    }).join('');
  });
}

export function openInvoiceForm() {
  safe(api.get('/user/invoice-quota')).then((res) => {
    const quota = res && res.data ? parseFloat(res.data.quota || 0) : 0;
    window.MD.openModal({
      title: '申请开票',
      wide: '640px',
      body:
        '<div class="field"><label>开票金额（元）<span class="req">*</span></label>' +
        '<input class="input" id="inv-amount" type="number" min="0.01" step="0.01" placeholder="可开额度 ¥' + quota.toFixed(2) + '">' +
        '<div class="hint">可开票额度 ¥' + quota.toFixed(2) + '（已充值金额扣除已占用）</div></div>' +
        '<div class="field"><label>抬头类型</label><div class="model-checks">' +
        '<label class="model-check is-on"><input type="radio" name="inv-title-type" value="company" checked>企业</label>' +
        '<label class="model-check"><input type="radio" name="inv-title-type" value="personal">个人</label>' +
        '</div></div>' +
        '<div class="field"><label>发票类型</label><div class="model-checks">' +
        '<label class="model-check is-on"><input type="radio" name="inv-invoice-type" value="normal" checked>增值税普通发票</label>' +
        '<label class="model-check"><input type="radio" name="inv-invoice-type" value="vat">增值税专用发票</label>' +
        '</div></div>' +
        '<div class="field"><label>发票抬头<span class="req">*</span></label><input class="input" id="inv-title" maxlength="200" placeholder="公司全称或个人姓名"></div>' +
        '<div class="field" id="inv-taxno-field"><label>税号<span class="req">*</span></label><input class="input" id="inv-taxno" maxlength="50" placeholder="统一社会信用代码"></div>' +
        '<div class="field" id="inv-bank-field" style="display:none;"><div class="form-grid" style="grid-template-columns:1fr 1fr;">' +
        '<div><label>开户行<span class="req">*</span></label><input class="input" id="inv-bank" maxlength="100" placeholder="如 招商银行北京分行"></div>' +
        '<div><label>银行账号<span class="req">*</span></label><input class="input" id="inv-account" maxlength="50" placeholder="对公账号"></div>' +
        '</div></div>' +
        '<div class="field"><label>接收邮箱</label><input class="input" id="inv-email" maxlength="100" placeholder="电子发票接收邮箱（选填）"></div>' +
        '<div class="field"><label>备注</label><textarea class="input" id="inv-remark" rows="2" maxlength="500" placeholder="其他说明（选填）"></textarea></div>',
      buttons: [
        { label: '取消', cls: 'btn-ghost', onClick: window.MD.closeModal },
        { label: '提交申请', cls: 'btn-primary', onClick: submitInvoice }
      ]
    });
    const onType = () => {
      const t = document.querySelector('input[name="inv-title-type"]:checked');
      document.getElementById('inv-taxno-field').style.display = t && t.value === 'company' ? '' : 'none';
    };
    document.querySelectorAll('input[name="inv-title-type"]').forEach((r) => { r.addEventListener('change', onType); });
    document.querySelectorAll('input[name="inv-invoice-type"]').forEach((r) => {
      r.addEventListener('change', () => { document.getElementById('inv-bank-field').style.display = r.value === 'vat' ? '' : 'none'; });
    });
  });
}

export function submitInvoice() {
  const amount = parseFloat((document.getElementById('inv-amount').value || '').trim());
  if (!amount || amount <= 0) { toast('请输入有效的开票金额', 'warn'); return; }
  const title = (document.getElementById('inv-title').value || '').trim();
  if (!title) { toast('请填写发票抬头', 'warn'); return; }
  const tt = document.querySelector('input[name="inv-title-type"]:checked');
  const it = document.querySelector('input[name="inv-invoice-type"]:checked');
  const titleType = tt ? tt.value : 'company';
  const invoiceType = it ? it.value : 'normal';
  const taxNo = (document.getElementById('inv-taxno').value || '').trim();
  if (titleType === 'company' && !taxNo) { toast('企业抬头必须填写税号', 'warn'); return; }
  const body = {
    amount: amount.toFixed(2),
    title_type: titleType,
    invoice_type: invoiceType,
    title: title,
    tax_no: taxNo,
    bank_name: (document.getElementById('inv-bank').value || '').trim(),
    bank_account: (document.getElementById('inv-account').value || '').trim(),
    email: (document.getElementById('inv-email').value || '').trim(),
    remark: (document.getElementById('inv-remark').value || '').trim()
  };
  if (invoiceType === 'vat' && (!body.bank_name || !body.bank_account)) { toast('专票必须填写开户行与银行账号', 'warn'); return; }
  api.post('/user/invoices', body).then(() => {
    toast('发票申请已提交，请等待审核', 'success');
    window.MD.closeModal();
    loadInvoices();
  }).catch((e) => toast(e.message || '提交失败', 'error'));
}

function emptyStateCard(title, sub) {
  return '<div style="text-align:center;padding:40px 20px;color:var(--text-3);">' +
    '<div style="font-size:34px;margin-bottom:8px;">🧾</div><div style="font-weight:600;color:var(--text-2);">' + esc(title) + '</div>' +
    '<div style="font-size:13px;margin-top:6px;">' + esc(sub || '') + '</div></div>';
}

import { onUserReady } from '../main.js';
onUserReady(initInvoices);
