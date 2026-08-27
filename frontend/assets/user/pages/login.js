// 登录页引导
import { initAuthPage, maybeDesktopCallback, setToken } from '../auth.js';
import { initOpenIDButtons } from '../widgets.js';
import { injectIcons } from '../ui.js';

document.addEventListener('DOMContentLoaded', () => {
  injectIcons();
  initAuthPage();
  initOpenIDButtons();

  const q = new URLSearchParams(location.search);
  if ((q.get('desktop_client') || '').toLowerCase() === 'smcode') {
    sessionStorage.setItem('dp_pending', '1');
    history.replaceState({}, '', location.pathname);
    const token = q.get('oauth_token');
    if (token) { setToken(token); location.href = '/user/dashboard.html'; }
    else if (window.MD.getToken()) maybeDesktopCallback(window.MD.getToken());
  }
});
