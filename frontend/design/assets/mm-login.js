/* MiniMax 风格登录页 —— 本地交互脚本（无 CDN，无依赖）
   Tab 切换 / 验证码倒计时 / 协议校验 / 表单校验 */
(function () {
  'use strict';

  var $ = function (id) { return document.getElementById(id); };

  /* ---------- Tab 切换（手机号登录 / 密码登录） ---------- */
  var tabs = document.querySelectorAll('.mm-tabs button');
  tabs.forEach(function (btn) {
    btn.addEventListener('click', function () {
      tabs.forEach(function (b) { b.classList.remove('is-on'); });
      btn.classList.add('is-on');
      var mode = btn.getAttribute('data-tab');
      $('form-code').hidden = mode !== 'code';
      $('form-password').hidden = mode !== 'password';
      clearErrors();
    });
  });

  /* ---------- 验证码倒计时 ---------- */
  var counting = false;
  var codeBtn = $('code-send');
  if (codeBtn) {
    codeBtn.addEventListener('click', function () {
      if (counting) return;
      var phone = $('phone').value.trim();
      if (!/^1\d{10}$/.test(phone)) { showError('code', '请输入正确的手机号'); return; }
      clearErrors();
      var s = 60;
      counting = true;
      codeBtn.setAttribute('disabled', '');
      codeBtn.textContent = s + 's 后重新获取';
      var timer = setInterval(function () {
        s -= 1;
        if (s <= 0) {
          clearInterval(timer);
          counting = false;
          codeBtn.removeAttribute('disabled');
          codeBtn.textContent = '获取验证码';
        } else {
          codeBtn.textContent = s + 's 后重新获取';
        }
      }, 1000);
    });
  }

  /* ---------- 校验 ---------- */
  function showError(key, msg) {
    var el = $('err-' + key);
    if (el) { el.textContent = msg; el.classList.add('show'); }
  }
  function clearErrors() {
    document.querySelectorAll('.mm-error').forEach(function (e) { e.classList.remove('show'); });
  }

  function checkAgree(checkboxId, errKey) {
    if (!$(checkboxId).checked) { showError(errKey, '请先阅读并同意相关协议'); return false; }
    return true;
  }

  /* 验证码登录提交 */
  var formCode = $('form-code');
  if (formCode) {
    formCode.addEventListener('submit', function (e) {
      e.preventDefault();
      clearErrors();
      var phone = $('phone').value.trim();
      var code = $('code').value.trim();
      if (!/^1\d{10}$/.test(phone)) { showError('code', '请输入正确的手机号'); return; }
      if (!code) { showError('code', '请输入验证码'); return; }
      if (!checkAgree('agree', 'agree-code')) return;
      toast('登录成功（原型）');
    });
  }

  /* 密码登录提交 */
  var formPwd = $('form-password');
  if (formPwd) {
    formPwd.addEventListener('submit', function (e) {
      e.preventDefault();
      clearErrors();
      var acct = $('account').value.trim();
      var pwd = $('password').value.trim();
      if (!acct) { showError('pwd', '请输入账号'); return; }
      if (!pwd) { showError('pwd', '请输入密码'); return; }
      if (!checkAgree('agree2', 'agree-pwd')) return;
      toast('登录成功（原型）');
    });
  }

  /* ---------- 极简 Toast（黑白灰体系） ---------- */
  function toast(msg) {
    var t = document.createElement('div');
    t.style.cssText = 'position:fixed;left:50%;top:32px;transform:translateX(-50%);' +
      'background:#181818;color:#fff;font-size:13px;padding:10px 18px;border-radius:10px;' +
      'box-shadow:0 4px 20px rgba(0,0,0,.08);z-index:99;font-family:inherit;';
    t.textContent = msg;
    document.body.appendChild(t);
    setTimeout(function () { t.style.opacity = '0'; t.style.transition = 'opacity .3s'; setTimeout(function () { t.remove(); }, 300); }, 2000);
  }

  window.MM = { toast: toast };
})();
