// 已登录页面的引导：鉴权 → 拉取用户 → 注入布局 → 标记就绪
import { requireAuth } from './auth.js';
import { loadUserAndChrome } from './layout.js';

if (requireAuth()) {
  loadUserAndChrome()
    .then(() => {
      window.__userReady = true;
      document.dispatchEvent(new Event('user-ready'));
    })
    .catch((err) => { console.error('[mass] 初始化失败', err); });
}

// 供页面模块使用：chrome 就绪后执行回调（兼容模块先于事件加载的情况）
export function onUserReady(cb) {
  if (window.__userReady) cb();
  else document.addEventListener('user-ready', cb, { once: true });
}
