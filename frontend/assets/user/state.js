// 全局状态与常量（ES Module 单例，跨页面共享）
export const VIEW_TITLES = {
  dashboard: '概览', keys: 'API Keys', billing: '用量账单', transactions: '交易记录',
  plans: '订阅套餐', recharge: '充值', profile: '个人设置', verify: '实名认证', models: '模型广场',
  coupons: '重置券', notifications: '通知', invoices: '发票', apidocs: 'API 对接',
  conversations: '对话记录', feedback: '反馈', chat: '对话测试'
};

export const MODELS = [];                 // 动态模型目录（GET /models 加载）
export const TX_TYPE = {
  recharge: ['充值', 'tag--success'], consume: ['消费', 'tag--info'], refund: ['退款', 'tag--warn'],
  subscription: ['订阅扣费', 'tag--gray'], adjust: ['余额调账', 'tag--info'], token_package: ['Token 加油包', 'tag--info']
};
export const TX_STATUS = {
  pending: ['处理中', 'tag--warn'], success: ['成功', 'tag--success'], failed: ['失败', 'tag--danger'], refunded: ['已退款', 'tag--gray']
};
export const PAY_LABEL = { epay: '在线支付（易支付）' };

export let currentUser = null;
export let balance = 0;
export let credit = 0;
export const keysCache = [];
export const plansCache = [];
export const subsCache = [];
export let payMethodsCache = null;
export const convState = { page: 1, size: 10, total: 0 };
export const rechargeState = { amount: 100, method: 'epay', custom: false, epayTxNo: null };
export const DP_KEY = 'dp_pending';

export function setUser(u) {
  currentUser = u || null;
  balance = (u && typeof u.balance !== 'undefined') ? u.balance : 0;
  credit = (u && typeof u.credit !== 'undefined') ? u.credit : 0;
}
export function setPayMethods(m) { payMethodsCache = m; }

let _readyResolve;
const _readyPromise = new Promise((res) => { _readyResolve = res; });
export function whenReady(cb) {
  if (cb) return _readyPromise.then(cb);
  return _readyPromise;
}
export function markReady() { if (_readyResolve) { _readyResolve(); _readyResolve = null; } }
