// 共享小组件：密码显隐 / 法律条款 / 亦 OpenID 登录
import { api, esc, icon } from './api.js';

let siteLegal = null;

export function togglePwd(inputId, btn) {
  const inp = document.getElementById(inputId);
  if (!inp || !btn) return;
  const show = inp.type === 'password';
  inp.type = show ? 'text' : 'password';
  btn.innerHTML = icon('eye', 15);
  btn.style.color = show ? 'var(--brand-500)' : '';
  btn.setAttribute('aria-label', show ? '隐藏密码' : '显示密码');
}

export function ensureSiteLegal() {
  if (siteLegal) return Promise.resolve(siteLegal);
  return api.get('/site-config').then((res) => {
    siteLegal = (res && res.data) || {};
    return siteLegal;
  }).catch(() => { siteLegal = {}; return siteLegal; });
}

export function openLegal(kind) {
  ensureSiteLegal().then((cfg) => {
    const isPrivacy = kind === 'privacy';
    let txt = isPrivacy ? (cfg.legal_privacy || '') : (cfg.legal_terms || '');
    if (!txt) txt = '（' + (isPrivacy ? '隐私政策' : '服务条款') + '内容暂未发布，请稍后再试）';
    txt = String(txt).replace(/\\n/g, '\n');
    window.MD.openModal({
      title: isPrivacy ? '隐私政策' : '服务条款',
      wide: '680px',
      body: '<div class="legal-doc">' + esc(txt) + '</div>',
      buttons: [{ label: '我知道了', cls: 'btn-primary', onClick: window.MD.closeModal }]
    });
  });
}

export function initOpenIDButtons() {
  fetch('/api/v1/auth/openid/config').then((r) => r.json()).then((res) => {
    const ok = res && res.code === 0 && res.data && res.data.enabled;
    document.querySelectorAll('.oauth-btn').forEach((b) => { b.style.display = ok ? '' : 'none'; });
  }).catch(() => {});
}

export function openIDLogin() {
  location.href = '/api/v1/auth/openid/authorize?intent=login';
}

// 暴露给内联 onclick 使用
window.togglePwd = togglePwd;
window.openLegal = openLegal;
window.openIDLogin = openIDLogin;
