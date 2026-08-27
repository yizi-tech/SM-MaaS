// 认证：登录 / 注册 / 桌面客户端回传 / 退出
import { setUser, DP_KEY } from './state.js';
import { api, toast, esc, openModal, closeModal, confirm } from './api.js';
import { goView } from './layout.js';
import { validEmail, pwdScore, updatePwdMeter, updateMatchTip, fieldOk } from './ui.js';

export function getToken() { return window.MD.getToken(); }
export function setToken(t) { window.MD.setToken(t); }
export function clearToken() { window.MD.clearToken(); }

export function requireAuth() {
  if (!getToken()) { location.href = '/user/login.html'; return false; }
  return true;
}

export function onAuthSuccess(res, msg) {
  setToken(res.data.token);
  setUser(res.data.user);
  toast(msg, 'success');
  maybeDesktopCallback(res.data.token);
  location.href = '/user/dashboard.html';
}

export function doLogin(email, password) {
  const btn = document.getElementById('login-btn');
  if (btn) { btn.disabled = true; btn.innerHTML = '<span class="spinner"></span>登录中…'; }
  api.post('/auth/login', { email: email, password: password })
    .then((res) => onAuthSuccess(res, '登录成功'))
    .catch((e) => toast(e.message, 'error'))
    .finally(() => { if (btn) { btn.disabled = false; btn.textContent = '登 录'; } });
}

export function doRegister(nickname, email, method, phone, code, password) {
  const btn = document.getElementById('reg-btn');
  if (btn) { btn.disabled = true; btn.innerHTML = '<span class="spinner"></span>正在创建账号…'; }
  const payload = { email: email, password: password, nickname: nickname, verify_code: code, verify_method: method };
  if (method === 'sms') payload.phone = phone;
  api.post('/auth/register', payload)
    .then((res) => {
      setToken(res.data.token);
      setUser(res.data.user);
      maybeDesktopCallback(res.data.token);
      showRegistrationSuccess();
    })
    .catch((e) => toast(e.message, 'error'))
    .finally(() => { if (btn) { btn.disabled = false; btn.textContent = '免费注册'; } });
}

export function showRegistrationSuccess() {
  const card = document.getElementById('view-register');
  if (card) card.classList.add('is-success');
  const success = document.getElementById('reg-success');
  if (success) success.classList.add('is-show');
  document.querySelectorAll('[data-ic]').forEach((el) => {
    if (el.__iconDone) return;
    el.innerHTML = window.MD.icon(el.getAttribute('data-ic'), parseInt(el.getAttribute('data-sz') || '16', 10));
    el.__iconDone = true;
  });
}
export function completeRegistration() {
  toast('欢迎使用 MASS，先创建一个 API Key 吧', 'success');
  goView('keys', 'new');
}

/* ---------- 桌面客户端登录回传（SMCode://） ---------- */
export function maybeDesktopCallback(jwt) {
  if (sessionStorage.getItem(DP_KEY) !== '1' || !jwt) return;
  showDesktopConfirm(jwt);
}
export function showDesktopConfirm(jwt) {
  openModal({
    title: '登录桌面客户端',
    body: '<p style="font-size:14px;color:var(--text-2);line-height:1.7;">检测到桌面客户端登录请求，确认使用当前账号登录到桌面客户端？</p>' +
      '<p style="font-size:13px;color:var(--text-3);margin-top:8px;">确认后将在本机唤起亦桌面客户端，不影响当前网页会话。</p>',
    buttons: [
      { label: '继续使用网页版', cls: 'btn-ghost', onClick: cancelDesktopLogin },
      { label: '确认并打开桌面客户端', cls: 'btn-primary', onClick: () => confirmDesktopLogin(jwt) }
    ]
  });
}
export function confirmDesktopLogin(jwt) {
  sessionStorage.removeItem(DP_KEY);
  closeModal();
  toast('正在唤起桌面客户端…', 'info');
  setTimeout(() => { location.href = 'SMCode://auth?token=' + encodeURIComponent(jwt); }, 600);
}
export function cancelDesktopLogin() {
  sessionStorage.removeItem(DP_KEY);
  closeModal();
  toast('已取消桌面客户端登录，可继续使用网页版', 'info');
}

export function logout() {
  confirm('确定要退出登录吗？', () => {
    clearToken();
    setUser(null);
    location.href = '/user/login.html';
  }, { title: '退出登录', okLabel: '退出', danger: true });
}

/* ---------- 登录页初始化 ---------- */
export function initAuthPage() {
  const params = new URLSearchParams(location.search);
  if (params.get('desktop') === '1') sessionStorage.setItem(DP_KEY, '1');

  // 切换 登录 / 注册
  const tabs = document.querySelectorAll('[data-auth-tab]');
  tabs.forEach((t) => t.addEventListener('click', () => {
    const which = t.dataset.authTab;
    tabs.forEach((x) => x.classList.toggle('is-active', x === t));
    document.getElementById('view-login').classList.toggle('is-active', which === 'login');
    document.getElementById('view-register').classList.toggle('is-active', which === 'register');
  }));

  // 注册方式 email / sms
  let regMethod = 'email';
  const methodBtns = document.querySelectorAll('[data-reg-method]');
  methodBtns.forEach((b) => b.addEventListener('click', () => {
    regMethod = b.dataset.regMethod;
    methodBtns.forEach((x) => x.classList.toggle('is-active', x === b));
    const phoneRow = document.getElementById('reg-phone-row');
    if (phoneRow) phoneRow.style.display = regMethod === 'sms' ? '' : 'none';
  }));

  const sendBtn = document.getElementById('reg-send-code');
  if (sendBtn) sendBtn.addEventListener('click', () => {
    const email = document.getElementById('reg-email').value.trim();
    const phone = document.getElementById('reg-phone') ? document.getElementById('reg-phone').value.trim() : '';
    if (regMethod === 'email' && !validEmail(email)) { toast('请输入有效邮箱', 'error'); return; }
    if (regMethod === 'sms' && !phone) { toast('请输入手机号', 'error'); return; }
    sendBtn.disabled = true;
    api.post('/auth/send-code', { method: regMethod, email: email, phone: phone })
      .then(() => toast('验证码已发送', 'success'))
      .catch((e) => toast(e.message, 'error'))
      .finally(() => { sendBtn.disabled = false; });
  });

  const pw = document.getElementById('reg-password');
  if (pw) pw.addEventListener('input', updatePwdMeter);
  const pw2 = document.getElementById('reg-password2');
  if (pw2) pw2.addEventListener('input', updateMatchTip);

  const loginForm = document.getElementById('form-login');
  if (loginForm) loginForm.addEventListener('submit', (e) => {
    e.preventDefault();
    const email = document.getElementById('login-email').value.trim();
    const password = document.getElementById('login-password').value;
    if (!validEmail(email)) { toast('请输入有效邮箱', 'error'); return; }
    doLogin(email, password);
  });

  const regForm = document.getElementById('form-register');
  if (regForm) regForm.addEventListener('submit', (e) => {
    e.preventDefault();
    const nickname = document.getElementById('reg-nickname').value.trim();
    const email = document.getElementById('reg-email').value.trim();
    const phone = document.getElementById('reg-phone') ? document.getElementById('reg-phone').value.trim() : '';
    const code = document.getElementById('reg-verify-code').value.trim();
    const password = document.getElementById('reg-password').value;
    const password2 = document.getElementById('reg-password2').value;
    const agree = document.getElementById('reg-agree');
    if (!nickname) { toast('请输入昵称', 'error'); return; }
    if (!validEmail(email)) { toast('请输入有效邮箱', 'error'); return; }
    if (regMethod === 'sms' && !phone) { toast('请输入手机号', 'error'); return; }
    if (!code) { toast('请输入验证码', 'error'); return; }
    if (pwdScore(password) < 2) { toast('密码强度过低', 'error'); return; }
    if (password !== password2) { toast('两次密码不一致', 'error'); return; }
    if (agree && !agree.checked) { toast('请先同意用户协议', 'error'); return; }
    doRegister(nickname, email, regMethod, phone, code, password);
  });

  const successBtn = document.getElementById('reg-enter');
  if (successBtn) successBtn.addEventListener('click', completeRegistration);
}
