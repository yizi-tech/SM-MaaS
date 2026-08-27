/* =====================================================================
   MAAS Design System v1.0 —— 共享脚本库
   图标库（1.6 描边线性 SVG）/ Toast / Modal / API 封装 / 工具函数
   ===================================================================== */
(function (global) {
  'use strict';

  /* ---------- 图标库（24x24, stroke 1.6） ---------- */
  var ICONS = {
    grid: '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
    menu: '<path d="M3 6h18M3 12h18M3 18h18"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="M21 21l-4-4"/>',
    bell: '<path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.7 21a2 2 0 0 1-3.4 0"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    key: '<circle cx="8" cy="8" r="4"/><path d="M11 11l9 9"/><path d="M17 17l2-2"/><path d="M14 14l2-2"/>',
    cube: '<path d="M12 3l8 4.5v9L12 21l-8-4.5v-9z"/><path d="M12 3v18"/><path d="M4 7.5l8 4.5 8-4.5"/>',
    chart: '<path d="M3 3v18h18"/><path d="M7 14l4-4 3 3 5-6"/>',
    flask: '<path d="M9 3h6M10 3v6l-5 9a2 2 0 0 0 1.8 3h10.4A2 2 0 0 0 19 18l-5-9V3"/>',
    card: '<rect x="3" y="6" width="18" height="12" rx="2"/><path d="M3 10h18"/><path d="M7 15h4"/>',
    wallet: '<path d="M21 12V7a2 2 0 0 0-2-2H5a2 2 0 0 0 0 4h14v3"/><path d="M3 5v14a2 2 0 0 0 2 2h16v-5"/><path d="M18 12a2 2 0 0 0 0 4h4v-4z"/>',
    user: '<circle cx="12" cy="8" r="4"/><path d="M4 21v-1a6 6 0 0 1 6-6h4a6 6 0 0 1 6 6v1"/>',
    users: '<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>',
    shield: '<path d="M12 3l7 3v5c0 4.5-3 8.5-7 10-4-1.5-7-5.5-7-10V6z"/><path d="M9 12l2 2 4-4"/>',
    idcard: '<rect x="2" y="5" width="20" height="14" rx="2"/><circle cx="8" cy="11" r="2"/><path d="M5 16c.8-1.3 1.8-2 3-2s2.2.7 3 2"/><path d="M14 9h5M14 12h5M14 15h3"/>',
    gear: '<circle cx="12" cy="12" r="3"/><path d="M19 12a7 7 0 0 0-.1-1.2l2-1.5-2-3.4-2.3.9a7 7 0 0 0-2-1.2L14 3h-4l-.6 2.6a7 7 0 0 0-2 1.2l-2.3-.9-2 3.4 2 1.5A7 7 0 0 0 5 12c0 .4 0 .8.1 1.2l-2 1.5 2 3.4 2.3-.9a7 7 0 0 0 2 1.2L10 21h4l.6-2.6a7 7 0 0 0 2-1.2l2.3.9 2-3.4-2-1.5c.1-.4.1-.8.1-1.2z"/>',
    list: '<path d="M3 7h18M3 12h18M3 17h18"/>',
    check: '<path d="M20 6L9 17l-5-5"/>',
    checkCircle: '<circle cx="12" cy="12" r="9"/><path d="M8.5 12.5l2.5 2.5 4.5-5"/>',
    x: '<path d="M6 6l12 12M18 6L6 18"/>',
    alert: '<circle cx="12" cy="12" r="9"/><path d="M12 8v5"/><path d="M12 16.5v.01"/>',
    info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 7.5v.01"/>',
    doc: '<path d="M4 4h11l5 5v11a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z"/><path d="M14 4v5h5"/>',
    clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
    download: '<path d="M12 3v12"/><path d="M7 10l5 5 5-5"/><path d="M4 19h16"/>',
    upload: '<path d="M12 15V3"/><path d="M7 8l5-5 5 5"/><path d="M4 19h16"/>',
    refresh: '<path d="M21 12a9 9 0 1 1-2.6-6.4"/><path d="M21 3v6h-6"/>',
    logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="M16 17l5-5-5-5"/><path d="M21 12H9"/>',
    home: '<path d="M3 10l9-7 9 7v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M9 21v-7h6v7"/>',
    coin: '<circle cx="12" cy="12" r="9"/><path d="M12 7v10M9.5 9.5h4a1.5 1.5 0 0 1 0 3h-3a1.5 1.5 0 0 0 0 3h4"/>',
    copy: '<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/>',
    eye: '<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>',
    trash: '<path d="M3 6h18"/><path d="M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2"/><path d="M6 6l1 14a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-14"/><path d="M10 11v6M14 11v6"/>',
    edit: '<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/>',
    file: '<path d="M4 4h11l5 5v11a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z"/><path d="M14 4v5h5"/><path d="M8 13h8M8 17h5"/>',
    bolt: '<path d="M13 2L4 14h6l-1 8 9-12h-6z"/>',
    server: '<rect x="3" y="4" width="18" height="7" rx="1.5"/><rect x="3" y="13" width="18" height="7" rx="1.5"/><path d="M7 7.5h.01M7 16.5h.01"/>',
    database: '<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/>',
    cpu: '<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9" y="9" width="6" height="6" rx="1"/><path d="M12 2v4M12 18v4M2 12h4M18 12h4M5 5l2 2M17 17l2 2M19 5l-2 2M7 17l-2 2"/>',
    arrowRight: '<path d="M5 12h14"/><path d="M13 6l6 6-6 6"/>',
    arrowLeft: '<path d="M19 12H5"/><path d="M11 18l-6-6 6-6"/>',
    chevronDown: '<path d="M6 9l6 6 6-6"/>',
    filter: '<path d="M3 5h18l-7 8v6l-4 2v-8z"/>',
    star: '<path d="M12 3l2.7 5.6 6.1.8-4.5 4.3 1.1 6-5.4-2.9-5.4 2.9 1.1-6L3.2 9.4l6.1-.8z"/>',
    globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3a15 15 0 0 1 0 18 15 15 0 0 1 0-18"/>',
    zap: '<path d="M13 2L4 14h6l-1 8 9-12h-6z"/>',
    book: '<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V4a2 2 0 0 0-2-2H6.5A2.5 2.5 0 0 0 4 4.5z"/><path d="M4 19.5A2.5 2.5 0 0 0 6.5 22H20v-5"/>',
    help: '<circle cx="12" cy="12" r="9"/><path d="M9.5 9a2.5 2.5 0 0 1 4.9.8c0 1.6-2.4 2.2-2.4 3.7"/><path d="M12 17v.01"/>',
    send: '<path d="M22 2L11 13"/><path d="M22 2l-7 20-4-9-9-4z"/>',
    receipt: '<path d="M4 3h16v18l-2.5-1.5L15 21l-3-1.5L9 21l-2.5-1.5L4 21z"/><path d="M8 8h8M8 12h8M8 16h5"/>',
    calendar: '<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18"/><path d="M8 3v4M16 3v4"/>'
  };

  function icon(name, size) {
    size = size || 16;
    var inner = ICONS[name] || '';
    return '<svg class="ic" width="' + size + '" height="' + size + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + inner + '</svg>';
  }

  /* ---------- Toast ---------- */
  var toastWrap = null;
  function ensureToastWrap() {
    if (!toastWrap) {
      toastWrap = document.createElement('div');
      toastWrap.className = 'toast-wrap';
      document.body.appendChild(toastWrap);
    }
    return toastWrap;
  }
  var TOAST_ICON = { success: 'checkCircle', error: 'alert', warn: 'alert', info: 'info' };
  function toast(msg, type) {
    type = type || 'info';
    var wrap = ensureToastWrap();
    var el = document.createElement('div');
    el.className = 'toast toast--' + type;
    // msg is treated as plain text; escape it so server/user-supplied strings
    // can never inject markup into the toast (stored/reflected XSS sink).
    el.innerHTML = icon(TOAST_ICON[type] || 'info', 16) + '<span>' + escapeHtml(String(msg)) + '</span>';
    wrap.appendChild(el);
    setTimeout(function () {
      el.classList.add('out');
      setTimeout(function () { el.remove(); }, 280);
    }, 2600);
  }

  /* ---------- Modal ---------- */
  function openModal(opts) {
    var mask = document.getElementById('md-modal-mask');
    if (!mask) {
      mask = document.createElement('div');
      mask.id = 'md-modal-mask';
      mask.className = 'modal-mask';
      mask.innerHTML = '<div class="modal">' +
        '<div class="modal__head"><div class="modal__title" id="md-modal-title"></div>' +
        '<button class="modal__close" id="md-modal-close">' + icon('x', 16) + '</button></div>' +
        '<div class="modal__body" id="md-modal-body"></div>' +
        '<div class="modal__foot" id="md-modal-foot"></div></div>';
      document.body.appendChild(mask);
      mask.addEventListener('click', function (e) { if (e.target === mask) closeModal(); });
      document.getElementById('md-modal-close').addEventListener('click', closeModal);
    }
    document.getElementById('md-modal-title').textContent = opts.title || '';
    document.getElementById('md-modal-body').innerHTML = opts.body || '';
    var foot = document.getElementById('md-modal-foot');
    foot.innerHTML = '';
    (opts.buttons || [{ label: '关闭', cls: 'btn-ghost', onClick: closeModal }]).forEach(function (b) {
      var btn = document.createElement('button');
      btn.className = b.cls || 'btn-ghost';
      btn.textContent = b.label; // textContent: button labels are plain text
      btn.addEventListener('click', function () { if (b.onClick) b.onClick(); });
      foot.appendChild(btn);
    });
    if (opts.wide) mask.querySelector('.modal').style.maxWidth = opts.wide;
    else mask.querySelector('.modal').style.maxWidth = '';
    mask.classList.add('show');
    document.body.style.overflow = 'hidden';
  }
  function closeModal() {
    var mask = document.getElementById('md-modal-mask');
    if (mask) mask.classList.remove('show');
    document.body.style.overflow = '';
  }
  function confirmDialog(msg, onOk, opts) {
    opts = opts || {};
    // Default: msg is plain text and gets escaped (safe against stored/reflected XSS).
    // Callers that intentionally pass trusted markup (their own fragments + escaped
    // user data) may opt in with { html: true } — they are responsible for escaping
    // every interpolated user-controlled value themselves.
    var body = opts.html ? String(msg) : escapeHtml(String(msg));
    openModal({
      title: opts.title || '确认操作',
      body: '<p style="font-size:14px;color:var(--text-2);line-height:1.7;">' + body + '</p>',
      buttons: [
        { label: '取消', cls: 'btn-ghost', onClick: closeModal },
        { label: opts.okLabel || '确定', cls: opts.danger ? 'btn-danger btn-danger--solid' : 'btn-primary', onClick: function () { closeModal(); if (onOk) onOk(); } }
      ]
    });
  }

  /* ---------- 侧栏抽屉 ---------- */
  function openSide() {
    var s = document.getElementById('side'), m = document.getElementById('scrim');
    if (s) s.classList.add('open');
    if (m) m.classList.add('show');
  }
  function closeSide() {
    var s = document.getElementById('side'), m = document.getElementById('scrim');
    if (s) s.classList.remove('open');
    if (m) m.classList.remove('show');
  }

  /* ---------- 认证 ---------- */
  var TOKEN_KEY = 'mass_token';
  function getToken() { return localStorage.getItem(TOKEN_KEY) || ''; }
  function setToken(t) { localStorage.setItem(TOKEN_KEY, t); }
  function clearToken() { localStorage.removeItem(TOKEN_KEY); }

  /* ---------- API 封装 ---------- */
  var API_BASE = (global.MASS_API_BASE || 'http://localhost:8080') + '/api/v1';
  function api(method, path, body, opts) {
    opts = opts || {};
    if (opts.query) {
      var qs = Object.keys(opts.query).filter(function (k) { return opts.query[k] !== undefined && opts.query[k] !== ''; })
        .map(function (k) { return encodeURIComponent(k) + '=' + encodeURIComponent(opts.query[k]); }).join('&');
      if (qs) path += (path.indexOf('?') >= 0 ? '&' : '?') + qs;
    }
    var headers = {};
    var tk = getToken();
    if (tk) headers['Authorization'] = 'Bearer ' + tk;
    var isForm = (typeof FormData !== 'undefined') && (body instanceof FormData);
    if (!isForm) headers['Content-Type'] = 'application/json';
    return fetch(API_BASE + path, {
      method: method,
      headers: headers,
      body: isForm ? body : (body ? JSON.stringify(body) : undefined)
    }).then(function (r) {
      if (opts.raw) {
        return r.text().then(function (text) {
          if (!r.ok) { var e = new Error('HTTP ' + r.status); e.status = r.status; throw e; }
          return text;
        });
      }
      return r.json().catch(function () { return { code: r.status, message: r.statusText }; })
        .then(function (data) {
          if (!r.ok || (data && data.code !== 0 && data.code !== undefined)) {
            var err = new Error((data && data.message) || ('HTTP ' + r.status));
            err.status = r.status;
            throw err;
          }
          return data;
        });
    });
  }
  api.get = function (p, query, opts) { return api('GET', p, null, opts ? Object.assign({ query: query }, opts) : { query: query }); };
  api.post = function (p, b) { return api('POST', p, b); };
  api.put = function (p, b) { return api('PUT', p, b); };
  api.del = function (p) { return api('DELETE', p); };
  api.upload = function (p, fd) { return api('POST', p, fd); };

  /* ---------- 工具 ---------- */
  function fmtMoney(v) {
    var n = parseFloat(v);
    if (isNaN(n)) return '0.00';
    return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }
  function fmtNum(v) {
    var n = parseFloat(v);
    if (isNaN(n)) return '0';
    return n.toLocaleString('zh-CN');
  }
  function fmtDate(s) {
    if (!s) return '-';
    var d = new Date(s);
    if (isNaN(d.getTime())) return s;
    var p = function (n) { return (n < 10 ? '0' : '') + n; };
    return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
  }
  function escapeHtml(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  function todayText() {
    var d = new Date();
    var w = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'][d.getDay()];
    return d.getFullYear() + '年' + (d.getMonth() + 1) + '月' + d.getDate() + '日 · ' + w;
  }
  function renderPager(container, page, total, size, onChange) {
    var pages = Math.max(1, Math.ceil(total / size));
    var html = '<button ' + (page <= 1 ? 'disabled' : '') + ' data-p="' + (page - 1) + '">' + icon('arrowLeft', 14) + '</button>';
    var start = Math.max(1, page - 2), end = Math.min(pages, start + 4);
    start = Math.max(1, end - 4);
    if (start > 1) html += '<button data-p="1">1</button>' + (start > 2 ? '<button disabled>…</button>' : '');
    for (var i = start; i <= end; i++) html += '<button data-p="' + i + '" class="' + (i === page ? 'is-active' : '') + '">' + i + '</button>';
    if (end < pages) html += (end < pages - 1 ? '<button disabled>…</button>' : '') + '<button data-p="' + pages + '">' + pages + '</button>';
    html += '<button ' + (page >= pages ? 'disabled' : '') + ' data-p="' + (page + 1) + '">' + icon('arrowRight', 14) + '</button>';
    container.innerHTML = html;
    container.querySelectorAll('button[data-p]').forEach(function (b) {
      b.addEventListener('click', function () { onChange(parseInt(b.dataset.p, 10)); });
    });
  }
  function emptyState(ic, title, desc) {
    return '<div class="empty"><div class="empty__ic">' + icon(ic || 'doc', 26) + '</div>' +
      '<div class="empty__title">' + title + '</div>' +
      (desc ? '<div class="empty__desc">' + desc + '</div>' : '') + '</div>';
  }
  function loadingRow(cols) {
    return '<tr class="loading-row"><td colspan="' + (cols || 6) + '"><span class="spinner"></span>加载中…</td></tr>';
  }
  function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function () { toast('已复制到剪贴板', 'success'); }, function () { fallbackCopy(text); });
    } else fallbackCopy(text);
  }
  function fallbackCopy(text) {
    var ta = document.createElement('textarea');
    ta.value = text; document.body.appendChild(ta); ta.select();
    try { document.execCommand('copy'); toast('已复制到剪贴板', 'success'); }
    catch (e) { toast('复制失败', 'error'); }
    ta.remove();
  }

  /* ---------- 站点配置（品牌信息，自动从后端获取） ---------- */
  var siteConfig = {};
  var _siteConfigPromise = null;
  function getSiteConfig() { return siteConfig; }
  function loadSiteConfig() {
    if (!_siteConfigPromise) {
      _siteConfigPromise = api.get('/site-config').then(function (res) {
        siteConfig = (res && res.data) || {};
        applySiteConfigWhenReady();
        return siteConfig;
      }).catch(function () { return siteConfig; });
    }
    return _siteConfigPromise;
  }
  function siteName() { return (siteConfig.site_name || 'MAAS').trim(); }
  function applySiteConfigWhenReady() {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', applySiteConfig);
    } else {
      applySiteConfig();
    }
  }
  function applySiteConfig() {
    var name = siteName();
    // 品牌名：替换 .brand__name 主文本，保留 <small> 副标题
    document.querySelectorAll('.brand__name').forEach(function (el) {
      var small = el.querySelector('small');
      el.textContent = '';
      el.appendChild(document.createTextNode(name));
      if (small) el.appendChild(small);
    });
    // Logo 文字：取站点名前两个字符（保持 MA 风格的品牌字标）
    var logoText = (name.replace(/\s+/g, '') || 'MAAS').slice(0, 2).toUpperCase();
    document.querySelectorAll('.logo').forEach(function (el) { el.textContent = logoText; });
    // 正文中的 <span data-site-name> 占位符
    document.querySelectorAll('[data-site-name]').forEach(function (el) { el.textContent = name; });
    // 页面标题与描述
    if (siteConfig.site_name) {
      var suffix = location.pathname.indexOf('/admin') !== -1 ? ' 管理控制台 · ADMIN CONSOLE' : ' 控制台 · LLM API Gateway';
      document.title = name + suffix;
    }
    if (siteConfig.site_description) {
      var md = document.querySelector('meta[name="description"]');
      if (md) md.content = siteConfig.site_description;
    }
  }
  // 页面加载即自动拉取一次站点配置
  loadSiteConfig();

  /* ---------- 导出 ---------- */
  global.MD = {
    icon: icon, toast: toast, openModal: openModal, closeModal: closeModal, confirm: confirmDialog,
    openSide: openSide, closeSide: closeSide,
    api: api, getToken: getToken, setToken: setToken, clearToken: clearToken,
    fmtMoney: fmtMoney, fmtNum: fmtNum, fmtDate: fmtDate, escapeHtml: escapeHtml, todayText: todayText,
    renderPager: renderPager, emptyState: emptyState, loadingRow: loadingRow, copyText: copyText,
    loadSiteConfig: loadSiteConfig, getSiteConfig: getSiteConfig, siteName: siteName
  };
})(window);
