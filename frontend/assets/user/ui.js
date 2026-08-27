// 通用 UI 助手 + 图表渲染（移植自原单体 SPA）
import { esc, toast, fmtNum } from './api.js';

export const $ = (id) => document.getElementById(id);

export function safe(p) {
  return Promise.resolve(p).catch((e) => {
    toast(e && e.message ? e.message : String(e), 'error');
    return null;
  });
}

// 最近 7 天标签（今天/MM-DD）
export function last7Labels() {
  const out = [], p = (n) => (n < 10 ? '0' : '') + n;
  for (let i = 6; i >= 0; i--) {
    const d = new Date(Date.now() - i * 864e5);
    out.push(i === 0 ? '今天' : p(d.getMonth() + 1) + '-' + p(d.getDate()));
  }
  return out;
}

export function renderAxis(el) {
  if (!el) return;
  el.innerHTML = last7Labels().map((t) => (t === '今天' ? '<b>今天</b>' : '<span>' + t + '</span>')).join('');
}

// 把 [data-ic] 占位符替换为 MD.icon SVG
export function injectIcons() {
  document.querySelectorAll('[data-ic]').forEach((el) => {
    if (el.__iconDone) return;
    el.innerHTML = window.MD.icon(el.getAttribute('data-ic'), parseInt(el.getAttribute('data-sz') || '16', 10));
    el.__iconDone = true;
  });
}

export function loadingRow(cols) {
  return window.MD.loadingRow(cols);
}

/* ---------- 概览图表 ---------- */
// 后端仅返回有记录的稀疏日数据，这里补齐为最近 7 天连续序列（无记录补 0）
export function dailySeries(daily, valKey) {
  const map = {};
  (daily || []).forEach((it) => { map[String(it.date).slice(0, 10)] = parseFloat(it[valKey]) || 0; });
  const out = [], p = (n) => (n < 10 ? '0' : '') + n;
  for (let i = 6; i >= 0; i--) {
    const d = new Date(Date.now() - i * 864e5);
    const key = d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate());
    out.push({ x: (6 - i) * 100, value: map[key] || 0 });
  }
  return out;
}

// 中点平滑曲线
function smoothPath(pts) {
  if (pts.length < 2) return '';
  let d = 'M' + pts[0][0] + ',' + pts[0][1];
  for (let i = 1; i < pts.length; i++) {
    const xc = (pts[i - 1][0] + pts[i][0]) / 2, yc = (pts[i - 1][1] + pts[i][1]) / 2;
    d += ' C' + xc + ',' + pts[i - 1][1] + ' ' + xc + ',' + pts[i][1] + ' ' + pts[i][0] + ',' + pts[i][1];
  }
  return d;
}

// 近期消费：渐变面积 + 曲线 + 峰值高亮点
export function renderCostChart(series) {
  let max = 0;
  series.forEach((it) => { if (it.value > max) max = it.value; });
  const pts = series.map((it) => [it.x, max > 0 ? 110 - (it.value / max) * 95 : 110]);
  const line = smoothPath(pts);
  if (!line) return;
  const area = $('spark-area'), lineEl = $('spark-line'), dot = $('spark-dot');
  if (area) area.setAttribute('d', line + ' L600,120 L0,120 Z');
  if (lineEl) lineEl.setAttribute('d', line);
  if (dot) {
    if (max > 0) {
      const peak = series.reduce((a, b) => (b.value > a.value ? b : a));
      const cy = 110 - (peak.value / max) * 95;
      dot.innerHTML = '<circle cx="' + peak.x + '" cy="' + cy + '" r="4" fill="#5b6cff"/>' +
        '<circle cx="' + peak.x + '" cy="' + cy + '" r="8" fill="#5b6cff" opacity=".2"/>';
    } else dot.innerHTML = '';
  }
}

// 调用量：迷你柱
export function renderTokenBars(series) {
  let max = 0;
  series.forEach((it) => { if (it.value > max) max = it.value; });
  const box = $('mini-bars');
  if (!box) return;
  box.innerHTML = series.map((it) => {
    const h = max > 0 ? Math.max(4, Math.round(it.value / max * 100)) : 4;
    return '<div class="mini-bar" style="height:' + h + '%" title="' + fmtNum(it.value) + ' Tokens"></div>';
  }).join('');
}

/* ---------- 表单助手（注册 / 设置 / 实名） ---------- */
export function fieldOk(inputEl, ok) {
  if (!inputEl) return ok;
  const f = inputEl.closest('.field');
  if (f) f.classList.toggle('is-error', !ok);
  return ok;
}
export function validEmail(v) { return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test((v || '').trim()); }
export function pwdScore(v) {
  let s = 0; if (!v) return 0;
  if (v.length >= 8) s++;
  if (/[A-Z]/.test(v)) s++;
  if (/[0-9]/.test(v)) s++;
  if (/[^A-Za-z0-9]/.test(v)) s++;
  return s;
}
export function updatePwdMeter() {
  const el = document.getElementById('reg-pwd-meter'), lv = document.getElementById('reg-pwd-lv'), inp = document.getElementById('reg-password');
  if (!inp) return;
  const s = pwdScore(inp.value);
  if (el) el.setAttribute('data-lv', String(s));
  if (lv) { lv.setAttribute('data-lv', String(s)); lv.textContent = ['', '弱', '中', '强', '很强'][s] || ''; }
}
export function updateMatchTip() {
  const a = document.getElementById('reg-password'), b = document.getElementById('reg-password2'), tip = document.getElementById('reg-match-tip');
  if (!a || !b || !tip) return;
  if (!b.value) { tip.className = 'match-tip'; tip.textContent = ''; return; }
  const ok = a.value === b.value;
  tip.className = 'match-tip' + (ok ? ' is-ok' : ' is-err');
  tip.textContent = ok ? '两次输入一致' : '两次密码不一致';
}
