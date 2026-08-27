// 对话测试
import { api, toast, fmtMoney } from '../api.js';
import { safe } from '../ui.js';
import { refreshBalance } from '../layout.js';
import { updateTokenCreditUI } from './credit.js';
const MD = window.MD;

let chatMsgs = [];
let chatStreaming = false;
let chatAbort = null;
let MODELS = [];

export function initChat() {
  refreshChatModels();
  const input = document.getElementById('chat-input');
  if (input) {
    input.addEventListener('input', autoGrowChatInput);
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendChat(); }
    });
  }
}

function refreshChatModels() {
  const sel = document.getElementById('chat-model');
  if (sel.options.length) return;
  if (MODELS.length) {
    sel.innerHTML = MODELS.map((m) => '<option value="' + esc(m) + '">' + esc(m) + '</option>').join('');
    return;
  }
  setChatStatus('模型加载中…', false);
  api.get('/models').then((res) => {
    const list = (res && res.data) || [];
    MODELS = list.map((m) => m.id);
    refreshChatModels();
    setChatStatus('就绪', true);
  }).catch(() => {
    sel.innerHTML = '<option value="">模型加载失败，请刷新</option>';
    sel.disabled = true;
    setChatStatus('模型加载失败', false);
  });
}

function setChatStatus(text, ok) {
  const head = document.getElementById('chat-head'), t = document.getElementById('chat-status-text');
  if (!head || !t) return;
  t.textContent = text;
  head.classList.remove('is-busy');
  const dot = head.querySelector('.chat-head__dot');
  if (dot) dot.style.background = '';
  if (text === '生成中…') head.classList.add('is-busy');
  if (!ok && dot) dot.style.background = 'var(--c-danger)';
}

function esc(s) { return MD.escapeHtml(s == null ? '' : String(s)); }

function nowHm() {
  const d = new Date();
  return (d.getHours() < 10 ? '0' : '') + d.getHours() + ':' + (d.getMinutes() < 10 ? '0' : '') + d.getMinutes();
}

function chatBubble(role, content) {
  const box = document.createElement('div');
  box.className = 'chat-msg chat-msg--' + role;
  const ic = role === 'user' ? 'user' : 'bolt';
  const col = document.createElement('div');
  col.className = 'chat-msg__col';
  if (role === 'assistant') {
    const think = document.createElement('div');
    think.className = 'chat-msg__think';
    think.style.display = 'none';
    think.innerHTML = '<div class="chat-msg__think-head"><span>思考过程</span><span class="chat-msg__think-arrow">▾</span></div><div class="chat-msg__think-body"></div>';
    think.querySelector('.chat-msg__think-head').addEventListener('click', () => { think.classList.toggle('is-open'); });
    col.appendChild(think);
  }
  const contentEl = document.createElement('div');
  contentEl.className = 'chat-msg__content';
  if (content) contentEl.textContent = content;
  col.appendChild(contentEl);
  const meta = document.createElement('div');
  meta.className = 'chat-msg__meta';
  const time = document.createElement('span');
  time.textContent = nowHm();
  meta.appendChild(time);
  const copyBtn = document.createElement('button');
  copyBtn.className = 'chat-msg__copy';
  copyBtn.innerHTML = MD.icon('copy', 12) + '<span>复制</span>';
  copyBtn.addEventListener('click', () => {
    const txt = contentEl.textContent;
    if (!txt) return;
    const done = () => toast('已复制到剪贴板', 'success');
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(txt).then(done, () => { fallbackCopy(txt); done(); });
    } else { fallbackCopy(txt); done(); }
  });
  meta.appendChild(copyBtn);
  col.appendChild(meta);
  const avatar = document.createElement('div');
  avatar.className = 'chat-msg__avatar';
  avatar.innerHTML = MD.icon(ic, 17);
  box.appendChild(col);
  box.appendChild(avatar);
  return box;
}

function fallbackCopy(txt) {
  const ta = document.createElement('textarea');
  ta.value = txt;
  ta.style.position = 'fixed';
  ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.select();
  try { document.execCommand('copy'); } catch (e) {}
  ta.remove();
}

export function clearChat() {
  if (chatStreaming) return;
  chatMsgs = [];
  const log = document.getElementById('chat-log');
  log.innerHTML = '';
  log.appendChild(document.getElementById('chat-empty'));
  document.getElementById('chat-empty').style.display = '';
  const ta = document.getElementById('chat-input');
  ta.value = '';
  ta.style.height = '';
  ta.focus();
}

export function useSuggestion(btn) {
  const ta = document.getElementById('chat-input');
  ta.value = btn.querySelector('span:nth-child(2)').textContent;
  autoGrowChatInput();
  ta.focus();
}

function autoGrowChatInput() {
  const ta = document.getElementById('chat-input');
  ta.style.height = 'auto';
  ta.style.height = Math.min(ta.scrollHeight, 160) + 'px';
}

function setChatBusy(busy) {
  chatStreaming = busy;
  const send = document.getElementById('chat-send');
  const stop = document.getElementById('chat-stop');
  send.disabled = busy;
  send.querySelector('span:nth-child(2)').textContent = busy ? '生成中…' : '发送';
  stop.style.display = busy ? '' : 'none';
  setChatStatus(busy ? '生成中…' : '就绪', true);
}

export function stopChat() {
  if (chatAbort) chatAbort.abort();
  const head = document.getElementById('chat-head');
  if (head) head.classList.remove('is-busy');
}

export function sendChat() {
  if (chatStreaming) return;
  const model = document.getElementById('chat-model').value;
  const text = document.getElementById('chat-input').value.trim();
  if (!model) { toast('请先选择模型', 'info'); return; }
  if (!text) { toast('请输入消息', 'info'); return; }

  chatMsgs.push({ role: 'user', content: text });
  document.getElementById('chat-input').value = '';
  autoGrowChatInput();

  const log = document.getElementById('chat-log');
  const empty = document.getElementById('chat-empty');
  if (empty.style.display !== 'none') empty.style.display = 'none';
  log.appendChild(chatBubble('user', text));
  const aiBox = chatBubble('assistant', '');
  log.appendChild(aiBox);
  log.scrollTop = log.scrollHeight;

  setChatBusy(true);
  aiBox.classList.add('chat-msg--streaming');
  const apiBase = (window.MASS_API_BASE || location.origin) + '/api/v1';
  const body = JSON.stringify({ model: model, messages: chatMsgs, stream: true });
  const headers = { 'Content-Type': 'application/json' };
  const tk = MD.getToken();
  if (tk) headers['Authorization'] = 'Bearer ' + tk;
  chatAbort = new AbortController();
  const sig = chatAbort.signal;

  fetch(apiBase + '/user/chat/completions', { method: 'POST', headers: headers, body: body, signal: sig })
    .then((r) => {
      if (!r.ok) return r.json().then((d) => { const e = new Error((d && d.message) || 'HTTP ' + r.status); e.status = r.status; e.data = d && d.data; throw e; });
      const reader = r.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      let doneDone = false;
      function pump() {
        return reader.read().then((t) => {
          if (t.done) { finishChat(doneDone); return; }
          buf += decoder.decode(t.value, { stream: true });
          const lines = buf.split('\n');
          buf = lines.pop();
          lines.forEach((line) => {
            line = line.trim();
            if (!line || line.indexOf('data:') !== 0) return;
            const payload = line.slice(5).trim();
            if (payload === '[DONE]') { doneDone = true; return; }
            let chunk;
            try { chunk = JSON.parse(payload); } catch (e2) { return; }
            const delta = (chunk.choices && chunk.choices[0] && chunk.choices[0].delta) || {};
            if (delta && typeof delta.content === 'string' && delta.content) {
              const contentEl = aiBox.querySelector('.chat-msg__content');
              contentEl.textContent += delta.content;
              aiBox.scrollIntoView({ block: 'end' });
            }
            const think = delta && (delta.reasoning_content || delta.thinking);
            if (think && typeof think === 'string' && think) {
              const thinkWrap = aiBox.querySelector('.chat-msg__think');
              const thinkBody = aiBox.querySelector('.chat-msg__think-body');
              thinkWrap.style.display = '';
              thinkBody.textContent += think;
              aiBox.scrollIntoView({ block: 'end' });
            }
          });
          return pump();
        });
      }
      return pump();
    })
    .catch((e) => {
      if (e && e.name === 'AbortError') { finishChat('aborted'); return; }
      finishChat(false, e);
    });

  function finishChat(ok, err) {
    chatAbort = null;
    aiBox.classList.remove('chat-msg--streaming');
    const contentEl = aiBox.querySelector('.chat-msg__content');
    if (ok === false) {
      setChatBusy(false);
      if (err) {
        if (err.status === 402 && err.data && err.data.need) {
          toast('余额不足或授信额度已用完，请充值后再试', 'error');
          window.goView('recharge');
        } else {
          toast(err.message || '请求失败', 'error');
        }
      }
      contentEl.textContent = '（本次请求失败，未计费）';
      const meta = aiBox.querySelector('.chat-msg__meta');
      if (meta) { const t = meta.querySelector('span:first-child'); if (t) t.textContent = nowHm(); }
      chatMsgs.pop();
      refreshBalance();
      return;
    }
    const txt = contentEl.textContent;
    if (!txt) contentEl.textContent = '（本次请求未返回内容）';
    const last = chatMsgs[chatMsgs.length - 1];
    if (last && last.role === 'assistant') chatMsgs.pop();
    chatMsgs.push({ role: 'assistant', content: contentEl.textContent });
    setChatBusy(false);
    loadChatCost(aiBox);
  }
}

function loadChatCost(aiBox) {
  safe(api.get('/user/billing-records?page=1&size=1')).then((res) => {
    const item = res && res.data && res.data.items && res.data.items[0];
    if (!item) return;
    const parts = [];
    if (item.cost) {
      const c = parseFloat(item.cost);
      parts.push('消耗 ¥' + (c > 0 && c < 0.01 ? c.toFixed(4) : fmtMoney(c)));
    }
    parts.push('输入 ' + (item.tokens_in || 0) + ' · 输出 ' + (item.tokens_out || 0));
    if (item.cached_tokens) parts.push('缓存命中 ' + item.cached_tokens);
    const meta = aiBox.querySelector('.chat-msg__meta');
    if (!meta) return;
    const cost = document.createElement('span');
    cost.className = 'chat-msg__cost';
    cost.textContent = parts.join(' · ');
    meta.appendChild(cost);
    if (item.billing_type === 'subscription') {
      const sub = document.createElement('span');
      sub.className = 'tag tag--gray';
      sub.textContent = '订阅抵扣';
      meta.appendChild(sub);
    }
    refreshBalance();
    updateTokenCreditUI();
  });
}

import { onUserReady } from '../main.js';
onUserReady(initChat);
