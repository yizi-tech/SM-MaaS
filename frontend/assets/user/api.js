// 对 mass-design.js 提供的 MD.* 做薄封装，便于页面模块统一调用
const MD = window.MD;

export const api = MD.api;
export function toast(msg, type) { MD.toast(msg, type); }
export function esc(s) { return MD.escapeHtml(s == null ? '' : String(s)); }
export function fmtMoney(n) { return MD.fmtMoney(n); }
export function fmtNum(n) { return MD.fmtNum ? MD.fmtNum(n) : String(n); }
export function siteName() { return MD.siteName ? MD.siteName() : 'MASS'; }
export function todayText() { return MD.todayText ? MD.todayText() : ''; }
export function openModal(opts) { return MD.openModal(opts); }
export function closeModal() { return MD.closeModal(); }
export function confirm(msg, ok, opts) { return MD.confirm(msg, ok, opts); }
export function icon(n, sz) { return MD.icon(n, sz); }
