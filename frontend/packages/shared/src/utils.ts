// 通用工具函数

export function formatNumber(n: number | string | undefined | null): string {
  if (n === undefined || n === null || n === '') return '0';
  const num = typeof n === 'string' ? parseFloat(n) : n;
  if (Number.isNaN(num)) return '0';
  return num.toLocaleString('zh-CN', { maximumFractionDigits: 4 });
}

export function formatMoney(v: string | number | undefined | null): string {
  if (v === undefined || v === null || v === '') return '¥0.00';
  const num = typeof v === 'string' ? parseFloat(v) : v;
  if (Number.isNaN(num)) return '¥0.00';
  return `¥${num.toFixed(2)}`;
}

export function formatPercent(used: number, total: number): string {
  if (!total) return '0%';
  const p = Math.min(100, Math.max(0, (used / total) * 100));
  return `${p.toFixed(1)}%`;
}

// 状态 → Ant Design Badge / Tag 颜色
export const statusColor: Record<string, string> = {
  active: 'success',
  disabled: 'default',
  suspended: 'warning',
  pending: 'processing',
  approved: 'success',
  rejected: 'error',
  verified: 'success',
  unverified: 'default',
  success: 'success',
  failed: 'error',
  refunded: 'warning',
  cancelled: 'default',
  unused: 'processing',
  used: 'default',
  issued: 'success',
  processing: 'processing',
  resolved: 'success',
  closed: 'default',
  recharge: 'success',
  consume: 'processing',
  refund: 'warning',
  subscription: 'geekblue',
  token_package: 'purple',
  adjust: 'orange',
};

export function tagColor(status: string | undefined): string {
  return statusColor[status || ''] || 'default';
}

export const typeLabel: Record<string, string> = {
  recharge: '充值',
  consume: '消费',
  refund: '退款',
  subscription: '订阅',
  token_package: '加油包',
  adjust: '余额调整',
  pending: '待处理',
  processing: '处理中',
  success: '成功',
  failed: '失败',
  refunded: '已退款',
  cancelled: '已取消',
  active: '正常',
  disabled: '禁用',
  suspended: '停用',
  approved: '已通过',
  rejected: '已驳回',
  verified: '已认证',
  unverified: '未认证',
  used: '已使用',
  unused: '未使用',
  issued: '已开具',
  resolved: '已解决',
  closed: '已关闭',
  bug: '问题反馈',
  suggestion: '功能建议',
  other: '其他',
  normal: '普票',
  vat: '专票',
  company: '企业',
  personal: '个人',
  system: '系统',
  order: '订单',
  credit: '授信',
  security: '安全',
  timeout: '超时',
};

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

// copyText copies text to the clipboard. It uses the async Clipboard API when
// available (secure contexts), and falls back to a hidden textarea + execCommand
// so it still works on plain-HTTP deployments where navigator.clipboard is
// undefined. Returns a promise resolving to true on success.
export async function copyText(text: string): Promise<boolean> {
  if (text === undefined || text === null) return false;
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // fall through to the execCommand fallback below
  }
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.top = '-9999px';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}
