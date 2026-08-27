// 个人设置页（资料 + 修改密码 + 亦 OpenID 绑定）
import { currentUser } from '../state.js';
import { api, toast, esc, icon } from '../api.js';
import { safe } from '../ui.js';
import { renderUserChrome } from '../layout.js';

let pwdCodeTimer = null;

export function initProfile() {
  const infoForm = document.getElementById('panel-info');
  const pwdForm = document.getElementById('panel-pwd');
  if (infoForm) infoForm.addEventListener('submit', (e) => { e.preventDefault(); saveProfile(); });
  if (pwdForm) pwdForm.addEventListener('submit', (e) => { e.preventDefault(); savePassword(); });
  fillProfileForm();
}

export function switchTab(t) {
  document.getElementById('tab-info').classList.toggle('is-active', t === 'info');
  document.getElementById('tab-pwd').classList.toggle('is-active', t === 'pwd');
  document.getElementById('panel-info').style.display = t === 'info' ? 'block' : 'none';
  document.getElementById('panel-pwd').style.display = t === 'pwd' ? 'block' : 'none';
}

function fillProfileForm() {
  if (!currentUser) return;
  document.getElementById('pf-nickname').value = currentUser.nickname || '';
  document.getElementById('pf-phone').value = currentUser.phone || '';
  document.getElementById('pf-qq').value = currentUser.qq || '';
  document.getElementById('pf-email').value = currentUser.email || '';
  initPwdVerifyUI();
  previewAvatar();
  loadOpenIDBind();
}

/* ---------- 修改密码（验证码） ---------- */
function initPwdVerifyUI() {
  if (!currentUser) return;
  document.getElementById('pwd-vm-email').textContent = currentUser.email ? '(' + maskTarget(currentUser.email) + ')' : '';
  const smsRadio = document.getElementById('pwd-vm-sms');
  if (currentUser.phone) {
    smsRadio.disabled = false;
    document.getElementById('pwd-vm-sms-t').textContent = '(' + maskTarget(currentUser.phone) + ')';
  } else {
    smsRadio.disabled = true;
    document.getElementById('pwd-vm-sms-t').textContent = '(未绑定手机号)';
    if (document.getElementById('pwd-vm-sms').checked) document.getElementById('pwd-vm-email').checked = true;
  }
  resetPwdCode();
}

function maskTarget(t) {
  if (!t) return '';
  const s = String(t);
  if (s.indexOf('@') > 0) {
    const p = s.split('@');
    return p[0].slice(0, 1) + '***@' + p[1];
  }
  return s.length >= 8 ? s.slice(0, 3) + '****' + s.slice(-4) : '***';
}

function pwdVerifyMethod() {
  const el = document.querySelector('input[name="pwd-vm"]:checked');
  return el ? el.value : 'email';
}

function resetPwdCode() {
  document.getElementById('pwd-code').value = '';
  if (pwdCodeTimer) { clearInterval(pwdCodeTimer); pwdCodeTimer = null; }
  const btn = document.getElementById('pwd-send-code');
  btn.disabled = false;
  btn.textContent = '获取验证码';
  document.getElementById('pwd-code-hint').textContent = '获取验证码后将发送至你的邮箱或手机，5 分钟内有效';
}

export function sendPwdCode() {
  const btn = document.getElementById('pwd-send-code');
  if (btn.disabled) return;
  api.post('/user/password/send-code', { method: pwdVerifyMethod() })
    .then((res) => {
      const t = res && res.data && res.data.target;
      document.getElementById('pwd-code-hint').textContent = '验证码已发送至 ' + (t || '') + '，5 分钟内有效';
      toast('验证码已发送', 'success');
      let s = 60;
      btn.disabled = true;
      pwdCodeTimer = setInterval(() => {
        s--;
        btn.textContent = s + 's 后重新获取';
        if (s <= 0) { clearInterval(pwdCodeTimer); pwdCodeTimer = null; btn.disabled = false; btn.textContent = '重新获取'; }
      }, 1000);
    })
    .catch((e) => toast(e.message, 'error'));
}

export function savePassword() {
  const old = document.getElementById('pwd-old').value, n1 = document.getElementById('pwd-new').value, n2 = document.getElementById('pwd-new2').value;
  const code = document.getElementById('pwd-code').value.trim();
  if (!old || !n1) { toast('请填写完整密码信息', 'warn'); return; }
  if (n1.length < 6) { toast('新密码至少 6 位', 'warn'); return; }
  if (n1 !== n2) { toast('两次输入的新密码不一致', 'warn'); return; }
  if (!code) { toast('请先获取并填写验证码', 'warn'); return; }
  api.put('/user/password', { old_password: old, new_password: n1, verify_method: pwdVerifyMethod(), verify_code: code })
    .then(() => {
      toast('密码修改成功', 'success');
      document.getElementById('pwd-old').value = document.getElementById('pwd-new').value = document.getElementById('pwd-new2').value = '';
      resetPwdCode();
    })
    .catch((e) => toast(e.message, 'error'));
}

function loadOpenIDBind() {
  const box = document.getElementById('openid-bind-box');
  if (!box) return;
  box.innerHTML = '<div class="bind-card"><span class="spinner"></span><span class="bind-card__main">加载绑定状态…</span></div>';
  api.get('/user/openid/status').then((res) => {
    const bound = res.data && res.data.bound;
    const name = (res.data && res.data.username) || '';
    if (bound) {
      box.innerHTML =
        '<div class="bind-card">' +
        '<img src="/assets/yz-1.png" width="42" height="42" style="border-radius:12px;flex:none;">' +
        '<div class="bind-card__main"><div class="bind-card__name">已绑定亦 OpenID</div>' +
        '<div class="bind-card__meta">' + esc(name || '亦 OpenID 账号') + '</div></div>' +
        '<button class="btn-ghost btn-sm" onclick="openIDUnbind()">解绑</button>' +
        '</div>';
    } else {
      box.innerHTML =
        '<div class="bind-card">' +
        '<img src="/assets/yz-1.png" width="42" height="42" style="border-radius:12px;flex:none;">' +
        '<div class="bind-card__main"><div class="bind-card__name">绑定亦 OpenID</div>' +
        '<div class="bind-card__meta">绑定后可一键登录本账号</div></div>' +
        '<button class="btn-primary btn-sm" onclick="openIDBind()">立即绑定</button>' +
        '</div>';
    }
  }).catch((e) => {
    box.innerHTML = '<div class="hint">' + esc(e.message || '绑定状态加载失败') + '</div>';
  });
}

export function openIDBind() {
  toast('正在跳转亦 OpenID 授权…', 'info');
  api.post('/user/openid/bind').then((res) => {
    const url = res.data && res.data.authorize_url;
    if (url) location.href = url;
  }).catch((e) => toast(e.message || '绑定失败', 'error'));
}

export function openIDUnbind() {
  const done = () => {
    api.post('/user/openid/unbind').then(() => {
      toast('已解绑', 'success');
      loadOpenIDBind();
    }).catch((e) => toast(e.message || '解绑失败', 'error'));
  };
  if (window.MD.confirm) window.MD.confirm('确定解除亦 OpenID 绑定？', done);
  else done();
}

function qqAvatarURL(qq) {
  const q = (qq || '').trim();
  return q && /^\d{5,11}$/.test(q) ? 'https://q1.qlogo.cn/g?b=qq&nk=' + q + '&s=640' : '';
}

export function previewQQAvatar(input) {
  const url = qqAvatarURL(input && input.value || '');
  const box = document.getElementById('pf-avatar-preview');
  if (url) {
    box.style.backgroundImage = "url('" + url.replace(/\\/g, '\\\\').replace(/'/g, "\\'") + "')";
    box.style.backgroundSize = 'cover';
    box.style.backgroundPosition = 'center';
    box.style.backgroundRepeat = 'no-repeat';
    box.textContent = '';
  } else {
    previewAvatar();
  }
}

function previewAvatar() {
  const box = document.getElementById('pf-avatar-preview');
  if (currentUser && currentUser.avatar) {
    box.style.backgroundImage = "url('" + currentUser.avatar.replace(/\\/g, '\\\\').replace(/'/g, "\\'") + "')";
    box.style.backgroundSize = 'cover';
    box.style.backgroundPosition = 'center';
    box.style.backgroundRepeat = 'no-repeat';
    box.textContent = '';
  } else {
    box.style.backgroundImage = '';
    box.textContent = (currentUser && (currentUser.nickname || currentUser.email) || 'M').slice(0, 1).toUpperCase();
  }
}

export function saveProfile() {
  const nickname = document.getElementById('pf-nickname').value.trim();
  if (!nickname) { toast('昵称不能为空', 'warn'); return; }
  const qq = document.getElementById('pf-qq').value.trim();
  if (qq && !/^\d{5,11}$/.test(qq)) { toast('请填写正确的 QQ 号（5-11 位数字）', 'warn'); return; }
  api.put('/user/profile', {
    nickname: nickname,
    phone: document.getElementById('pf-phone').value.trim(),
    qq: qq
  }).then((res) => {
    currentUser.nickname = res.data.nickname;
    currentUser.avatar = res.data.avatar;
    renderUserChrome();
    previewAvatar();
    toast('资料已更新', 'success');
  }).catch((e) => toast(e.message, 'error'));
}

window.switchTab = switchTab;
window.sendPwdCode = sendPwdCode;
window.openIDBind = openIDBind;
window.openIDUnbind = openIDUnbind;
window.previewQQAvatar = previewQQAvatar;

import { onUserReady } from '../main.js';
onUserReady(initProfile);
