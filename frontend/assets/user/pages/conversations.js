// 对话记录
import { api, toast, esc, fmtNum, fmtMoney } from '../api.js';
import { safe } from '../ui.js';

const convState = { page: 1, size: 12, total: 0 };

export function initConversations() {
  loadConversations(1);
}

export function loadConversations(page) {
  const model = document.getElementById('conv-model').value;
  convState.page = page || 1;
  api.get('/user/conversations', { model: model, page: convState.page, size: convState.size })
    .then((res) => {
      const d = res.data || {};
      const items = d.items || [];
      convState.total = d.total || 0;
      const sel = document.getElementById('conv-model');
      const known = sel.innerHTML;
      const opts = (d.models || []).map((m) => '<option value="' + esc(m) + '">' + esc(m) + '</option>').join('');
      if (opts && known.indexOf(opts) === -1) sel.innerHTML = '<option value="">全部模型</option>' + opts;
      if (!items.length) {
        document.getElementById('conv-list').innerHTML = '<div class="card"><p style="color:var(--text-3);font-size:13px;">暂无对话记录，调用一次 API 后即可在此查看</p></div>';
        return;
      }
      const rows = items.map((c) => {
        let msgs = [];
        try { msgs = JSON.parse(c.messages || '[]'); } catch (e) {}
        let first = '';
        for (let i = 0; i < msgs.length; i++) {
          if (msgs[i].role === 'user' && msgs[i].content) { first = msgs[i].content; break; }
        }
        if (first.length > 90) first = first.slice(0, 90) + '…';
        return '<tr><td>' + esc(c.model) + (c.stream ? ' <span class="tag tag--info">流式</span>' : '') + '</td>' +
          '<td style="max-width:340px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--text-2);">' + esc(first || '（无文本请求）') + '</td>' +
          '<td>' + fmtNum(c.tokens_in + c.tokens_out) + ' Tokens</td>' +
          '<td>¥' + fmtMoney(c.cost) + '</td>' +
          '<td style="white-space:nowrap;">' + window.MD.fmtDate(c.created_at) + '</td>' +
          '<td><button class="btn-ghost btn-sm" onclick="viewConversation(' + c.id + ')">查看</button></td></tr>';
      }).join('');
      document.getElementById('conv-list').innerHTML =
        '<div class="card" style="padding:0;overflow:hidden;"><table class="table"><thead><tr>' +
        '<th>模型</th><th>请求内容</th><th>Tokens</th><th>费用</th><th>时间</th><th></th></tr></thead>' +
        '<tbody>' + rows + '</tbody></table></div>' +
        '<div style="margin-top:12px;display:flex;align-items:center;gap:12px;justify-content:center;">' +
        '<button class="btn-ghost btn-sm" ' + (convState.page <= 1 ? 'disabled' : '') + ' onclick="loadConversations(' + (convState.page - 1) + ')">上一页</button>' +
        '<span class="hint">第 ' + convState.page + ' / ' + Math.max(1, Math.ceil(convState.total / convState.size)) + ' 页，共 ' + fmtNum(convState.total) + ' 条</span>' +
        '<button class="btn-ghost btn-sm" ' + (convState.page * convState.size >= convState.total ? 'disabled' : '') + ' onclick="loadConversations(' + (convState.page + 1) + ')">下一页</button>' +
        '</div>';
    })
    .catch((e) => toast(e.message || '加载失败', 'error'));
}

export function viewConversation(id) {
  api.get('/user/conversations/' + id).then((res) => {
    const c = res.data;
    let msgs = [];
    try { msgs = JSON.parse(c.messages || '[]'); } catch (e) {}
    let resp = {};
    try { resp = JSON.parse(c.response || '{}'); } catch (e) {}
    let body = '';
    msgs.forEach((m) => {
      body += '<div style="margin:0 0 10px;"><span class="tag ' + (m.role === 'user' ? 'tag--info' : 'tag--warn') + '" style="margin-right:6px;">' + esc(m.role) + '</span>' +
        '<div style="margin-top:6px;padding:10px 12px;background:var(--bg-2);border-radius:8px;font-size:13px;white-space:pre-wrap;word-break:break-all;max-height:220px;overflow-y:auto;">' + esc(m.content || '') + '</div></div>';
    });
    if (resp.content) {
      body += '<div style="margin:0 0 10px;"><span class="tag tag--success" style="margin-right:6px;">assistant</span>' +
        '<div style="margin-top:6px;padding:10px 12px;background:var(--bg-2);border-radius:8px;font-size:13px;white-space:pre-wrap;word-break:break-all;max-height:320px;overflow-y:auto;">' + esc(resp.content) + '</div></div>';
    }
    window.MD.openModal({
      title: '对话详情 · ' + c.model,
      body: body + '<div class="hint">' + c.tokens_in + ' in / ' + c.tokens_out + ' out · ¥' + fmtMoney(c.cost) + ' · ' + window.MD.fmtDate(c.created_at) + '</div>',
      buttons: [{ label: '关闭', cls: 'btn-ghost', onClick: window.MD.closeModal }]
    });
  }).catch((e) => toast(e.message || '加载失败', 'error'));
}

export function exportJsonl() {
  api.get('/user/conversations/export.jsonl', {}, { raw: true })
    .then((text) => {
      const blob = new Blob([text], { type: 'application/x-ndjson' });
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = 'conversations.jsonl';
      document.body.appendChild(a);
      a.click();
      URL.revokeObjectURL(a.href);
      a.remove();
      toast('导出成功', 'success');
    })
    .catch((e) => toast(e.message || '导出失败', 'error'));
}

window.viewConversation = viewConversation;
window.exportJsonl = exportJsonl;

import { onUserReady } from '../main.js';
onUserReady(initConversations);
