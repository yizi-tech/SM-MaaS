/* MASS · StarMoon — 本地交互脚本（无 CDN，替代原 mass-design.js）
   仅提供原型所需交互：图标注入 / Toast / 密码显隐 / 分段控件 / 简单弹窗 */
(function () {
  'use strict';

  /* ---------- 内联 SVG 图标库（线框，等效 SF Symbols） ---------- */
  var ICONS = {
    key: '<path d="M14 7a4 4 0 1 0-3.9 5L7 16v2H4v-2h2v-2h2v-2h2v-2l3.1-3.1A4 4 0 0 0 14 7Z" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/><circle cx="14" cy="11" r="1.4" fill="currentColor"/>',
    chart: '<path d="M4 19V5M4 19h16M8 15l3-4 3 3 4-6" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>',
    shield: '<path d="M12 3 5 6v5c0 4 3 7 7 9 4-2 7-5 7-9V6l-7-3Z" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/><path d="m9 12 2 2 4-4" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>',
    card: '<rect x="3" y="6" width="18" height="12" rx="2.5" fill="none" stroke="currentColor" stroke-width="1.7"/><path d="M3 10h18" stroke="currentColor" stroke-width="1.7"/><path d="M7 15h3" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>',
    coin: '<circle cx="12" cy="12" r="8.5" fill="none" stroke="currentColor" stroke-width="1.7"/><path d="M12 8v8M9.5 9.8c0-1 1-1.6 2.5-1.6s2.5.6 2.5 1.6S13.5 12 12 12s-2.5.6-2.5 1.6S10.5 15 12 15s2.5-.6 2.5-1.6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>',
    doc: '<path d="M7 3h7l4 4v14H7z" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/><path d="M14 3v4h4M9 12h6M9 15h6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>',
    zap: '<path d="M13 3 5 13h5l-1 8 8-10h-5l1-8Z" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/>',
    plus: '<path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"/>',
    eye: '<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" fill="none" stroke="currentColor" stroke-width="1.6"/><circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="1.6"/>',
    eyeOff: '<path d="M4 4l16 16" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/><path d="M9.5 9.7A3 3 0 0 0 12 15a3 3 0 0 0 3.2-1.4M6.5 6.6C3.7 8.3 2 12 2 12s3.5 7 10 7c1.9 0 3.6-.5 5-1.3" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>',
    check: '<path d="m5 12 4.5 4.5L19 7" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"/>',
    chevron: '<path d="m9 6 6 6-6 6" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>',
    trash: '<path d="M5 7h14M9 7V5h6v2M7 7l1 13h8l1-13" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>',
    copy: '<rect x="9" y="9" width="11" height="11" rx="2" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M5 15V5a2 2 0 0 1 2-2h8" fill="none" stroke="currentColor" stroke-width="1.6"/>',
    star: '<path d="m12 4 2.3 4.7 5.2.8-3.8 3.7.9 5.1L12 16.9 7.4 18.9l.9-5.1L4.5 9.5l5.2-.8L12 4Z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>',
    bolt: '<path d="M13 3 5 13h5l-1 8 8-10h-5l1-8Z" fill="currentColor"/>'
  };

  function injectIcons(root) {
    root = root || document;
    var els = root.querySelectorAll('[data-i]');
    els.forEach(function (el) {
      var n = el.getAttribute('data-i');
      var p = ICONS[n]; if (!p) return;
      var sz = parseInt(el.getAttribute('data-sz') || '18', 10);
      el.innerHTML =
        '<svg class="ic" viewBox="0 0 24 24" width="' + sz + '" height="' + sz + '" aria-hidden="true">' + p + '</svg>';
    });
  }

  function toast(msg, type) {
    var wrap = document.querySelector('.toast-wrap');
    if (!wrap) { wrap = document.createElement('div'); wrap.className = 'toast-wrap'; document.body.appendChild(wrap); }
    var t = document.createElement('div');
    t.className = 'toast ' + (type || '');
    t.textContent = msg;
    wrap.appendChild(t);
    setTimeout(function () { t.style.opacity = '0'; t.style.transition = 'opacity .3s'; setTimeout(function () { t.remove(); }, 300); }, 2200);
  }

  function togglePwd(id, btn) {
    var el = document.getElementById(id); if (!el) return;
    if (el.type === 'password') { el.type = 'text'; btn.setAttribute('data-on', '1'); }
    else { el.type = 'password'; btn.removeAttribute('data-on'); }
    injectIcons(btn);
  }

  window.MD = { injectIcons: injectIcons, toast: toast, togglePwd: togglePwd };
  document.addEventListener('DOMContentLoaded', function () { injectIcons(document); });
})();
