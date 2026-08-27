// 反馈
import { api, toast, esc, fmtMoney } from '../api.js';

export function initFeedback() {
  loadMyFeedback(1);
}

export function submitFeedback() {
  const type = document.getElementById('fb-type').value;
  const title = (document.getElementById('fb-title').value || '').trim();
  const content = (document.getElementById('fb-content').value || '').trim();
  const contact = (document.getElementById('fb-contact').value || '').trim();
  if (!title) { toast('请输入标题', 'warn'); return; }
  if (!content) { toast('请输入详细描述', 'warn'); return; }
  api.post('/user/feedback', { type: type, title: title, content: content, contact: contact })
    .then(() => {
      toast('反馈已提交，感谢你的支持', 'success');
      document.getElementById('fb-title').value = '';
      document.getElementById('fb-content').value = '';
      document.getElementById('fb-contact').value = '';
      loadMyFeedback(1);
    })
    .catch((e) => toast(e.message || '提交失败', 'error'));
}

export function loadMyFeedback(page) {
  api.get('/user/feedback', { page: page || 1, size: 10 }).then((res) => {
    const d = res.data || {};
    const items = d.items || [];
    if (!items.length) {
      document.getElementById('fb-list').innerHTML = '<div class="card"><p style="color:var(--text-3);font-size:13px;">暂无反馈记录</p></div>';
      return;
    }
    const ST = { pending: ['待处理', 'tag--warn'], processing: ['处理中', 'tag--info'], resolved: ['已解决', 'tag--success'], closed: ['已关闭', 'tag--gray'] };
    const TP = { bug: '程序问题', suggestion: '功能建议', other: '其他' };
    document.getElementById('fb-list').innerHTML = items.map((f) => {
      const st = ST[f.status] || ['未知', 'tag--gray'];
      return '<div class="card" style="margin-bottom:10px;">' +
        '<div style="display:flex;justify-content:space-between;align-items:center;gap:10px;flex-wrap:wrap;">' +
        '<b>' + esc(f.title) + '</b><span class="tag ' + st[1] + '">' + st[0] + '</span></div>' +
        '<div style="margin-top:8px;font-size:13px;color:var(--text-2);white-space:pre-wrap;word-break:break-all;">' + esc(f.content) + '</div>' +
        '<div style="margin-top:8px;display:flex;gap:10px;flex-wrap:wrap;" class="hint">' +
        '<span>类型：' + (TP[f.type] || f.type) + '</span><span>' + window.MD.fmtDate(f.created_at) + '</span>' +
        (f.admin_note ? '<span style="color:var(--success);">处理备注：' + esc(f.admin_note) + '</span>' : '') +
        '</div></div>';
    }).join('');
    }).catch((e) => toast(e.message || '加载失败', 'error'));
}

import { onUserReady } from '../main.js';
onUserReady(initFeedback);
