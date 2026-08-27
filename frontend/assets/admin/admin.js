'use strict';
/* =====================================================================
   MAAS 管理控制台 SPA
   依赖：../assets/mass-design.js（MD.*）与后端 /api/v1
   ===================================================================== */

/* ---------- 小工具 ---------- */
function el(id) { return document.getElementById(id); }
function esc(s) { return MD.escapeHtml(s); }
function dstr(d) { var p = function (n) { return (n < 10 ? '0' : '') + n; }; return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()); }
function daysAgo(n) { var d = new Date(); d.setDate(d.getDate() - n); return dstr(d); }
function maskIdNo(s) { s = String(s || ''); return s.length <= 7 ? s : s.slice(0, 3) + '***********' + s.slice(-4); }

/* ---------- 语义映射 ---------- */
var USER_STATUS = { active: ['正常', 'tag--success'], disabled: ['已禁用', 'tag--danger'], suspended: ['已暂停', 'tag--warn'] };
var REAL_NAME   = { verified: ['已认证', 'tag--success'], unverified: ['未认证', 'tag--gray'], pending: ['审核中', 'tag--warn'], rejected: ['已拒绝', 'tag--danger'] };
var VERIFY_ST   = { pending: ['待审核', 'tag--warn'], approved: ['已通过', 'tag--success'], rejected: ['已拒绝', 'tag--danger'] };
var TX_TYPE     = { recharge: ['充值', 'tag--success'], consume: ['消费', 'tag--danger'], subscription: ['订阅', 'tag--info'], refund: ['退款', 'tag--warn'] };
var TX_STATUS   = { pending: ['待处理', 'tag--warn'], success: ['成功', 'tag--success'], failed: ['失败', 'tag--danger'], refunded: ['已退款', 'tag--gray'] };
var LOG_LEVEL   = { debug: ['DEBUG', 'tag--gray'], info: ['INFO', 'tag--info'], warn: ['WARN', 'tag--warn'], error: ['ERROR', 'tag--danger'] };

function tagOf(map, val) {
  var t = map[val];
  return t ? '<span class="tag ' + t[1] + '">' + t[0] + '</span>'
           : '<span class="tag tag--gray">' + esc(val || '-') + '</span>';
}
function metricCard(icon, label, valueHtml, sub) {
  return '<article class="card metric-card"><div class="m-label">' + MD.icon(icon, 15) + esc(label) + '</div>' +
    '<div class="m-value">' + valueHtml + '</div>' + (sub ? '<div class="m-sub">' + sub + '</div>' : '') + '</article>';
}
function emptyRow(cols, icon, title, desc) {
  return '<tr><td colspan="' + cols + '" style="padding:0;border:0">' + MD.emptyState(icon, title, desc) + '</td></tr>';
}
function countText(total, page, size) {
  var pages = Math.max(1, Math.ceil(total / size));
  return '共 ' + MD.fmtNum(total) + ' 条 · 第 ' + page + ' / ' + pages + ' 页';
}

/* ---------- 迷你柱状图 ---------- */
function renderBars(barEl, axisEl, series) {
  var max = 0;
  series.forEach(function (it) { if (it.value > max) max = it.value; });
  barEl.innerHTML = series.map(function (it) {
    var h = max > 0 ? Math.max(4, Math.round(it.value / max * 100)) : 4;
    return '<div class="mini-bar" style="height:' + h + '%" title="' + it.date + ' · ¥' + MD.fmtMoney(it.value) + '"></div>';
  }).join('');
  if (!axisEl) return;
  var n = series.length;
  if (!n) { axisEl.innerHTML = ''; return; }
  var today = dstr(new Date());
  var label = function (i) {
    return series[i].date === today ? '<b>今天</b>' : '<span>' + series[i].date.slice(5) + '</span>';
  };
  var html = label(0);
  if (n > 2) html += label(Math.floor((n - 1) / 3));
  if (n > 3) html += label(Math.floor((n - 1) * 2 / 3));
  if (n > 1) html += (series[n - 1].date === today) ? '<b>今天</b>' : '<span>' + series[n - 1].date.slice(5) + '</span>';
  axisEl.innerHTML = html;
}
/* 把后端稀疏的按日数据补齐为连续日期序列 */
function buildSeries(startStr, endStr, items) {
  var map = {};
  (items || []).forEach(function (it) { map[String(it.date).slice(0, 10)] = parseFloat(it.amount || 0); });
  var out = [], d = new Date(startStr + 'T00:00:00'), end = new Date(endStr + 'T00:00:00');
  if (isNaN(d.getTime()) || isNaN(end.getTime()) || d > end) return out;
  for (var i = 0; i < 92 && d <= end; i++) {
    var key = dstr(d);
    out.push({ date: key, value: map[key] || 0 });
    d.setDate(d.getDate() + 1);
  }
  return out;
}

/* =====================================================================
   全局状态 & 视图切换
   ===================================================================== */
var App = {
  profile: null,
  view: 'dashboard',
  users:  { page: 1, size: 10, keyword: '', status: '' },
  verify: { page: 1, size: 10, status: '' },
  orders: { page: 1, size: 10, type: '', status: '' },
  logs:   { page: 1, size: 10, level: '', module: '', start: '', end: '' }
};

var VIEWS = {
  dashboard: { title: '数据概览',     load: loadDashboard },
  users:     { title: '用户管理',     load: loadUsers },
  verify:    { title: '实名认证审核', load: loadVerify },
orders:     { title: '订单管理',     load: loadOrders },
  invoices:   { title: '发票审核',     load: loadInvoices },
  pay:        { title: '支付接口',     load: loadPayPage },
  credit:     { title: '授信审核',     load: function () { loadCreditApps(1); loadCollections(1); } },
  plans:     { title: '套餐管理',     load: loadPlans },
  revenue:   { title: '收入分析',     load: loadRevenue },
  channels:  { title: '模型渠道',      load: loadChannels },
  pricing:   { title: '模型定价',      load: loadPricingGroups },
  modelPrices: { title: '模型价格',      load: loadModelPrices },
  resetCoupons: { title: '重置券',       load: loadResetCoupons },
  tokenPackages: { title: '加油包管理', load: loadTokenPackages },
  config:    { title: '系统配置',     load: loadConfig },
  logs:      { title: '系统日志',     load: loadLogs },
  health:    { title: '系统健康',     load: loadHealth },
  notifications: { title: '通知中心', load: loadNotifications },
  feedback: { title: '反馈处理', load: function () { loadFeedback(1); } },
  conversations: { title: '调用详情', load: function () { loadAdminConversations(1); } }
};

/* ================= 多页面框架（登录 / 导航 / 框架注入） ================= */
function showView(name){ location.href = name + '.html'; }

function fieldOk(inputEl, ok){ var f = inputEl.closest('.field'); if(f) f.classList.toggle('is-error', !ok); return ok; }
function validEmail(v){ return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v); }

var ADMIN_NAV = [
  { icon:'grid',  title:'看板',   items:[['dashboard','数据概览']] },
  { icon:'users', title:'运营管理', items:[['users','用户管理'],['verify','实名认证审核'],['orders','订单管理'],['invoices','发票审核'],['credit','授信审核'],['feedback','反馈处理']] },
  { icon:'coin',  title:'商业化',  items:[['plans','套餐管理'],['token-packages','加油包管理'],['pay','支付接口'],['revenue','收入分析']] },
  { icon:'cpu',   title:'AI 平台', items:[['channels','模型渠道'],['pricing','模型定价'],['model-prices','模型价格'],['reset-coupons','重置券'],['conversations','调用详情']] },
  { icon:'gear',  title:'系统',   items:[['notifications','通知中心'],['config','系统配置'],['logs','系统日志'],['health','系统健康']] }
];

function buildSidebar(active){
  var html = '<div class="brand"><div style="text-align:center"><img src="/assets/yz-1.png" width="48"></div>' +
    '<div class="brand__name">MAAS<small>ADMIN CONSOLE</small></div></div><nav class="nav">';
  ADMIN_NAV.forEach(function(g){
    html += '<div class="nav__group"><div class="nav__group-title"><span data-icon="'+g.icon+':14"></span><span>'+g.title+'</span></div>';
    g.items.forEach(function(it){
      var v=it[0], label=it[1];
      html += '<a class="nav__item nav__item--sub'+(v===active?' is-active':'')+'" data-view="'+v+'" href="'+v+'.html"><span>'+label+'</span></a>';
    });
    html += '</div>';
  });
  html += '</nav><div class="side-foot">' +
    '<a class="nav__item" href="../user/index.html"><span data-icon="home:17"></span><span>返回用户端</span></a>' +
    '<a class="nav__item" id="btn-logout" onclick="doLogout()"><span data-icon="logout:17"></span><span>退出登录</span></a></div>';
  return html;
}
function buildTopbar(view){
  var t = (VIEWS[view] && VIEWS[view].title) || '管理控制台';
  return '<button class="hamburger" onclick="MD.openSide()" aria-label="菜单"><span data-icon="menu:20"></span></button>' +
    '<div class="breadcrumb" id="crumb">管理控制台 / <b>'+esc(t)+'</b></div>' +
    '<div class="search"><span data-icon="search:16"></span><input type="text" id="hd-search" placeholder="搜索用户（邮箱 / 昵称），回车跳转"></div>' +
    '<div class="header__right"><button class="icon-btn" id="hd-bell" aria-label="通知"><span data-icon="bell:18"></span><span class="dot"></span></button>' +
    '<div class="avatar" id="hd-avatar" title="管理员">A</div></div>';
}
function paintIcons(){
  document.querySelectorAll('[data-icon]').forEach(function(n){
    var parts = String(n.getAttribute('data-icon')).split(':');
    n.innerHTML = MD.icon(parts[0], parseInt(parts[1] || '16', 10));
  });
}
function injectChrome(view){
  var side=el('side'), top=el('topbar');
  if(side) side.innerHTML = buildSidebar(view);
  if(top) top.innerHTML = buildTopbar(view);
  paintIcons();
  var s=el('hd-search');
  if(s) s.addEventListener('keydown', function(e){ if(e.key==='Enter'){ var kw=this.value.trim(); if(kw) location.href='users.html?q='+encodeURIComponent(kw); } });
}
function refreshNotifBadge(){
  if(!MD.getToken()) return;
  MD.api.get('/user/notifications/unread-count').then(function(res){
    if(!res) return;
    var n=(res.data||{}).unread||0;
    var dot=el('hd-bell') && el('hd-bell').querySelector('.dot');
    if(dot){ dot.style.display=n>0?'':'none'; dot.textContent=n>99?'99+':n; dot.classList.add('is-num'); }
  });
}
function bootstrap(view){
  if(!MD.getToken()){ location.replace('login.html'); return; }
  var app=el('app'); if(app) app.style.display='flex';
  injectChrome(view);
  if(view==='users'){ var q=new URLSearchParams(location.search).get('q'); if(q){ App.users.keyword=q; var s=el('hd-search'); if(s) s.value=q; } }
  MD.api.get('/user/profile').then(function(res){
    var u=res.data||{}; App.profile=u;
    var av=el('hd-avatar'); if(av){ var letter=String(u.nickname||u.email||'A').trim().charAt(0).toUpperCase()||'A'; av.textContent=letter; av.title=(u.nickname||'')+' '+(u.email||''); }
  }).catch(function(){});
  refreshNotifBadge();
  setInterval(refreshNotifBadge, 60000);
  if(view==='revenue'){ var rs=el('rev-start'), re=el('rev-end'); if(rs&&!rs.value) rs.value=daysAgo(13); if(re&&!re.value) re.value=dstr(new Date()); }
  var v=VIEWS[view]; if(v && v.load){ try{ v.load(); }catch(e){ MD.toast('页面加载失败','error'); } }
}
function initLogin(){
  if(MD.getToken()){ location.replace('dashboard.html'); return; }
  paintIcons();
  var form=el('login-form'); if(form) form.addEventListener('submit', doLogin);
  var eye=el('login-eye'); if(eye) eye.addEventListener('click', function(){
    var inp=el('login-password'); var show=inp.type==='password'; inp.type=show?'text':'password';
    this.innerHTML=MD.icon('eye',15); this.style.color=show?'var(--brand-500)':''; this.setAttribute('aria-label', show?'隐藏密码':'显示密码');
  });
  ['login-email','login-password'].forEach(function(id){ var e=el(id); if(e) e.addEventListener('input', function(){ fieldOk(this,true); }); });
}
function doLogin(ev){
  if(ev) ev.preventDefault();
  var emailEl=el('login-email'), pwdEl=el('login-password');
  var email=emailEl.value.trim(), pwd=pwdEl.value;
  if(!fieldOk(emailEl, validEmail(email)) || !fieldOk(pwdEl, !!pwd)) return;
  var btn=el('login-btn'); btn.disabled=true; btn.innerHTML='<span class="spinner"></span>登录中…';
  MD.api.post('/auth/login', { email:email, password:pwd }).then(function(res){
    var data=res.data||{}, user=data.user;
    if(!user || user.role!=='admin'){ MD.clearToken(); MD.toast('该账号无管理员权限，拒绝访问','error'); return; }
    MD.setToken(data.token); App.profile=user; MD.toast('欢迎回来，'+esc(user.nickname||user.email),'success');
    location.replace('dashboard.html');
  }).catch(function(err){ MD.toast(err.message||'登录失败','error'); }).finally(function(){ btn.disabled=false; btn.textContent='登录管理控制台'; });
}
function doLogout(){
  MD.confirm('确定退出管理控制台吗？', function(){ MD.clearToken(); App.profile=null; location.replace('login.html'); }, { title:'退出登录', okLabel:'退出' });
}

/* 暴露给页面内联脚本调用 */
window.Admin = {
  bootstrap: bootstrap,
  initLogin: initLogin,
  doLogin: doLogin,
  doLogout: doLogout,
  showView: showView
};

/* =====================================================================
   视图 1 · 数据看板
   ===================================================================== */
function loadDashboard() {
  el('dash-date').textContent = MD.todayText();

  // 4 个核心指标
  el('dash-stats').innerHTML =
    metricCard('users', '总用户', '-', '累计注册') +
    metricCard('bolt', '活跃用户', '-', '状态正常') +
    metricCard('coin', '总收入', '-', '累计交易流水') +
    metricCard('chart', '今日请求', '-', 'API 调用次数');
  MD.api.get('/admin/analytics/overview').then(function (res) {
    var d = res.data || {};
    el('dash-stats').innerHTML =
      metricCard('users', '总用户', MD.fmtNum(d.total_users), '累计注册') +
      metricCard('bolt', '活跃用户', MD.fmtNum(d.active_users), '状态正常') +
      metricCard('coin', '总收入', '¥ ' + MD.fmtMoney(d.total_revenue), '累计交易流水') +
      metricCard('chart', '今日请求', MD.fmtNum(d.today_requests), 'API 调用次数');
  }).catch(function (err) { MD.toast(err.message || '概览数据加载失败', 'error'); });

  // 健康胶囊
  el('dash-health').innerHTML = '<span class="health-pill health-pill--info">' + MD.icon('clock', 14) + '检测中…</span>';
  MD.api.get('/admin/health').then(function (res) {
    var h = res.data || {}, ok = h.database && h.redis && h.api;
    el('dash-health').innerHTML = ok
      ? '<span class="health-pill">' + MD.icon('check', 14) + '系统正常</span>'
      : '<span class="health-pill health-pill--warn">' + MD.icon('alert', 14) + '服务异常</span>';
  }).catch(function () {
    el('dash-health').innerHTML = '<span class="health-pill health-pill--warn">' + MD.icon('alert', 14) + '检测失败</span>';
  });

  // 收入趋势（近 14 天）
  var start = daysAgo(13), end = dstr(new Date());
  MD.api.get('/admin/analytics/revenue?start_date=' + start + '&end_date=' + end).then(function (res) {
    var series = buildSeries(start, end, res.data || []);
    var total = series.reduce(function (s, it) { return s + it.value; }, 0);
    el('dash-rev-total').innerHTML = '¥ ' + MD.fmtMoney(total) + ' <small>14 天累计收入</small>';
    renderBars(el('dash-bars'), el('dash-axis'), series);
  }).catch(function (err) {
    MD.toast(err.message || '收入数据加载失败', 'error');
    el('dash-bars').innerHTML = ''; el('dash-axis').innerHTML = '';
  });
}

/* =====================================================================
   视图 2 · 用户管理
   ===================================================================== */
function usersQuery() {
  App.users.page = 1;
  App.users.keyword = el('user-kw').value.trim();
  App.users.status = el('user-status').value;
  loadUsers();
}
/* ---------- 反馈处理 ---------- */
var fbState = { page: 1, size: 10, total: 0 };
var FB_STATUS = { pending: ['待处理', 'tag--warn'], processing: ['处理中', 'tag--info'], resolved: ['已解决', 'tag--success'], closed: ['已关闭', 'tag--gray'] };
var FB_TYPE = { bug: '程序问题', suggestion: '功能建议', other: '其他' };

function loadFeedback(page) {
  fbState.page = page || 1;
  var status = el('fb-status-filter') ? el('fb-status-filter').value : '';
  MD.api.get('/admin/feedback', { status: status, page: fbState.page, size: fbState.size })
    .then(function (res) {
      var d = res.data || {};
      var items = d.items || [];
      fbState.total = d.total || 0;
      if (!items.length) {
        el('feedback-list').innerHTML = MD.emptyState('doc', '暂无反馈', '用户提交的反馈会显示在这里');
        return;
      }
      el('feedback-list').innerHTML = items.map(function (f) {
        var st = FB_STATUS[f.status] || ['未知', 'tag--gray'];
        var note = f.admin_note ? '<div class="hint" style="margin-top:8px;color:var(--success);">处理备注：' + MD.escapeHtml(f.admin_note) + '</div>' : '';
        return '<div class="card" style="margin-bottom:12px;">' +
          '<div style="display:flex;justify-content:space-between;align-items:center;gap:10px;flex-wrap:wrap;">' +
          '<b>' + MD.escapeHtml(f.title) + '</b>' +
          '<div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap;">' +
          '<span class="tag ' + st[1] + '">' + st[0] + '</span>' +
          '<button class="btn-ghost btn-sm" onclick="setFbStatus(' + f.id + ',\'processing\')">处理中</button>' +
          '<button class="btn-ghost btn-sm" onclick="setFbStatus(' + f.id + ',\'resolved\')">标记解决</button>' +
          '<button class="btn-ghost btn-sm" onclick="setFbStatus(' + f.id + ',\'closed\')">关闭</button>' +
          '</div></div>' +
          '<div style="margin-top:8px;font-size:13px;color:var(--text-2);white-space:pre-wrap;word-break:break-all;">' + MD.escapeHtml(f.content) + '</div>' +
          '<div class="hint" style="margin-top:8px;">类型：' + (FB_TYPE[f.type] || f.type) + ' · 用户 #' + f.user_id + ' · ' + MD.fmtDate(f.created_at) +
          (f.contact ? ' · 联系方式：' + MD.escapeHtml(f.contact) : '') + '</div>' + note +
          '<div style="margin-top:10px;display:flex;gap:8px;">' +
          '<input class="input" id="fb-note-' + f.id + '" placeholder="处理备注（展示给用户）" maxlength="2000" style="flex:1;" value="' + MD.escapeHtml(f.admin_note || '') + '">' +
          '<button class="btn-primary btn-sm" onclick="saveFbNote(' + f.id + ')">保存备注</button></div>' +
          '</div>';
      }).join('') +
      '<div style="margin-top:12px;display:flex;align-items:center;gap:12px;justify-content:center;">' +
      '<button class="btn-ghost btn-sm" ' + (fbState.page <= 1 ? 'disabled' : '') + ' onclick="loadFeedback(fbState.page - 1)">上一页</button>' +
      '<span class="hint">第 ' + fbState.page + ' / ' + Math.max(1, Math.ceil(fbState.total / fbState.size)) + ' 页，共 ' + MD.fmtNum(fbState.total) + ' 条</span>' +
      '<button class="btn-ghost btn-sm" ' + (fbState.page * fbState.size >= fbState.total ? 'disabled' : '') + ' onclick="loadFeedback(fbState.page + 1)">下一页</button>' +
      '</div>';
    })
    .catch(function (e) { MD.toast(e.message || '加载失败', 'error'); });
}

function setFbStatus(id, status) {
  MD.api.put('/admin/feedback/' + id + '/status', { status: status })
    .then(function () { MD.toast('状态已更新', 'success'); loadFeedback(fbState.page); })
    .catch(function (e) { MD.toast(e.message || '更新失败', 'error'); });
}

function saveFbNote(id) {
  var note = el('fb-note-' + id).value || '';
  MD.api.put('/admin/feedback/' + id + '/status', { status: 'keep', admin_note: note })
    .then(function () { MD.toast('备注已保存', 'success'); loadFeedback(fbState.page); })
    .catch(function (e) { MD.toast(e.message || '保存失败', 'error'); });
}

/* ---------- 调用详情：跨用户对话留存 ---------- */
var convAdminState = { page: 1, size: 10, total: 0 };

function loadAdminConversations(page) {
  convAdminState.page = page || 1;
  var uid = (el('admin-conv-user') && el('admin-conv-user').value || '').trim();
  var model = el('admin-conv-model') ? el('admin-conv-model').value : '';
  var status = el('admin-conv-status') ? el('admin-conv-status').value : '';
  var q = { model: model, status: status, page: convAdminState.page, size: convAdminState.size };
  if (uid) q.user_id = uid;
  MD.api.get('/admin/conversations', q).then(function (res) {
    var d = res.data || {};
    var items = d.items || [];
    convAdminState.total = d.total || 0;
    var sel = el('admin-conv-model');
    var known = sel.innerHTML;
    var opts = (d.models || []).map(function (m) { return '<option value="' + MD.escapeHtml(m) + '">' + MD.escapeHtml(m) + '</option>'; }).join('');
    if (opts && known.indexOf(opts) === -1) sel.innerHTML = '<option value="">全部模型</option>' + opts;
    if (!items.length) {
      el('admin-conv-list').innerHTML = MD.emptyState('doc', '暂无调用记录', '全平台用户的 API / 对话调用会显示在这里');
      return;
    }
    var tableRows = items.map(function (c) {
      var msgs = [];
      try { msgs = JSON.parse(c.messages || '[]'); } catch (e) {}
      var first = '';
      for (var i = 0; i < msgs.length; i++) {
        if (msgs[i].role === 'user' && msgs[i].content) { first = msgs[i].content; break; }
      }
      if (first.length > 80) first = first.slice(0, 80) + '…';
      return '<tr>' +
        '<td><a class="link" onclick="showView(\'users\');">' + (c.user_email ? MD.escapeHtml(c.user_email) : '用户') + '</a><div class="hint">#' + c.user_id + '</div></td>' +
        '<td>' + MD.escapeHtml(c.model) + (c.stream ? ' <span class="tag tag--info">流式</span>' : '') + '</td>' +
        '<td style="max-width:320px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--text-2);">' + MD.escapeHtml(first || '（无文本请求）') + '</td>' +
        '<td>' + MD.fmtNum(c.tokens_in + c.tokens_out) + ' Tokens</td>' +
        '<td>¥' + MD.fmtMoney(c.cost) + '</td>' +
        '<td>' + (c.status === 'error' ? '<span class="tag tag--danger">失败</span>' : '<span class="tag tag--success">成功</span>') + '</td>' +
        '<td style="white-space:nowrap;">' + MD.fmtDate(c.created_at) + '</td>' +
        '<td><button class="btn-ghost btn-sm" onclick="viewAdminConversation(' + c.id + ')">查看</button></td></tr>';
    }).join('');

    var cards = items.map(function (c) {
      var msgs = [];
      try { msgs = JSON.parse(c.messages || '[]'); } catch (e) {}
      var first = '';
      for (var i = 0; i < msgs.length; i++) {
        if (msgs[i].role === 'user' && msgs[i].content) { first = msgs[i].content; break; }
      }
      if (first.length > 120) first = first.slice(0, 120) + '…';
      var row = function (k, v) { return '<div class="conv-card__row"><span class="conv-card__k">' + k + '</span><span class="conv-card__v">' + v + '</span></div>'; };
      return '<div class="conv-card" style="background:var(--bg-card);border:1px solid var(--border-1);border-radius:var(--r-md);padding:12px 14px;margin-bottom:10px;">' +
        row('用户', (c.user_email ? MD.escapeHtml(c.user_email) : '用户') + ' #' + c.user_id) +
        row('模型', MD.escapeHtml(c.model) + (c.stream ? ' · 流式' : '')) +
        row('请求', MD.escapeHtml(first || '（无文本请求）')) +
        row('Tokens', MD.fmtNum(c.tokens_in + c.tokens_out) + '（¥' + MD.fmtMoney(c.cost) + '）') +
        row('状态', c.status === 'error' ? '失败' : '成功') +
        row('时间', MD.fmtDate(c.created_at)) +
        '<div style="margin-top:8px;"><button class="btn-ghost btn-sm" onclick="viewAdminConversation(' + c.id + ')">查看完整调用</button></div>' +
        '</div>';
    }).join('');

    var pager = '<div style="margin-top:12px;display:flex;align-items:center;gap:12px;justify-content:center;">' +
      '<button class="btn-ghost btn-sm" ' + (convAdminState.page <= 1 ? 'disabled' : '') + ' onclick="loadAdminConversations(convAdminState.page - 1)">上一页</button>' +
      '<span class="hint">第 ' + convAdminState.page + ' / ' + Math.max(1, Math.ceil(convAdminState.total / convAdminState.size)) + ' 页，共 ' + MD.fmtNum(convAdminState.total) + ' 条</span>' +
      '<button class="btn-ghost btn-sm" ' + (convAdminState.page * convAdminState.size >= convAdminState.total ? 'disabled' : '') + ' onclick="loadAdminConversations(convAdminState.page + 1)">下一页</button>' +
      '</div>';

    el('admin-conv-list').innerHTML =
      '<div class="table-wrap"><table class="table"><thead><tr>' +
      '<th>用户</th><th>模型</th><th>请求内容</th><th>Tokens</th><th>费用</th><th>状态</th><th>时间</th><th></th></tr></thead>' +
      '<tbody>' + tableRows + '</tbody></table></div>' + cards + pager;
  }).catch(function (e) { MD.toast(e.message || '加载失败', 'error'); });
}

function viewAdminConversation(id) {
  MD.api.get('/admin/conversations/' + id).then(function (res) {
    var c = res.data;
    var msgs = [];
    try { msgs = JSON.parse(c.messages || '[]'); } catch (e) {}
    var resp = {};
    try { resp = JSON.parse(c.response || '{}'); } catch (e) {}
    var roleCls = { user: 'tag--info', assistant: 'tag--warn', system: 'tag--gray' };
    var thread = '';
    msgs.forEach(function (m) {
      var rc = roleCls[m.role] || 'tag--gray';
      thread += '<div style="margin:0 0 10px;"><span class="tag ' + rc + '" style="margin-right:6px;">' + MD.escapeHtml(m.role) + '</span>' +
        '<div style="margin-top:6px;padding:10px 12px;background:var(--bg-2);border-radius:8px;font-size:13px;white-space:pre-wrap;word-break:break-all;">' + MD.escapeHtml(m.content || '') + '</div></div>';
    });
    if (resp.content) {
      thread += '<div style="margin:0 0 10px;"><span class="tag tag--success" style="margin-right:6px;">assistant</span>' +
        '<div style="margin-top:6px;padding:10px 12px;background:var(--bg-2);border-radius:8px;font-size:13px;white-space:pre-wrap;word-break:break-all;">' + MD.escapeHtml(resp.content) + '</div></div>';
    }
    var meta = function (k, v) {
      return '<div style="display:flex;gap:10px;padding:7px 0;border-bottom:1px solid var(--border-2);font-size:13px;">' +
        '<div style="width:96px;flex:none;color:var(--text-3);">' + k + '</div>' +
        '<div style="color:var(--text-1);word-break:break-all;">' + v + '</div></div>';
    };
    var st = c.status === 'error' ? '<span class="tag tag--danger">失败</span>' : '<span class="tag tag--success">成功</span>';
    var metaHtml =
      meta('用户', (c.user_email ? MD.escapeHtml(c.user_email) : '用户') + ' <span class="hint">#' + c.user_id + '</span>') +
      meta('模型', MD.escapeHtml(c.model) + (c.stream ? ' <span class="tag tag--info">流式</span>' : '')) +
      meta('状态', st) +
      meta('请求 ID', MD.escapeHtml(c.request_id || '-')) +
      meta('API Key', c.api_key_id ? ('#' + c.api_key_id) : '-') +
      meta('输入 Token', MD.fmtNum(c.tokens_in)) +
      meta('输出 Token', MD.fmtNum(c.tokens_out)) +
      meta('缓存 Token', MD.fmtNum(c.tokens_cached)) +
      meta('费用', '¥' + MD.fmtMoney(c.cost)) +
      meta('完成原因', MD.escapeHtml(resp.finish_reason || '-')) +
      meta('时间', MD.fmtDate(c.created_at));
    var raw = MD.escapeHtml(JSON.stringify({ messages: msgs, response: resp }, null, 2));
    var body =
      '<div style="margin-bottom:4px;">' + metaHtml + '</div>' +
      '<div style="margin:14px 0 8px;font-weight:600;font-size:14px;">对话内容</div>' +
      (thread || '<div class="hint">（无内容）</div>') +
      '<details style="margin-top:12px;"><summary style="cursor:pointer;color:var(--brand-500);font-size:13px;">查看原始 JSON</summary>' +
      '<pre style="margin-top:8px;padding:12px;background:var(--bg-2);border-radius:8px;font-size:12px;white-space:pre-wrap;word-break:break-all;max-height:260px;overflow:auto;">' + raw + '</pre></details>';
    MD.openModal({
      title: '调用详情 · ' + c.model,
      body: body,
      buttons: [{ label: '关闭', cls: 'btn-ghost', onClick: MD.closeModal }]
    });
  }).catch(function (e) { MD.toast(e.message || '加载失败', 'error'); });
}

function exportAdminConversations() {
  var uid = (el('admin-conv-user') && el('admin-conv-user').value || '').trim();
  var model = el('admin-conv-model') ? el('admin-conv-model').value : '';
  var status = el('admin-conv-status') ? el('admin-conv-status').value : '';
  var q = [];
  if (uid) q.push('user_id=' + encodeURIComponent(uid));
  if (model) q.push('model=' + encodeURIComponent(model));
  if (status) q.push('status=' + encodeURIComponent(status));
  var url = '/admin/conversations/export.jsonl' + (q.length ? ('?' + q.join('&')) : '');
  MD.api.get(url, {}, { raw: true }).then(function (text) {
    var blob = new Blob([text], { type: 'application/x-ndjson' });
    var a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'conversations-all.jsonl';
    document.body.appendChild(a);
    a.click();
    URL.revokeObjectURL(a.href);
    a.remove();
    MD.toast('导出成功', 'success');
  }).catch(function (e) { MD.toast(e.message || '导出失败', 'error'); });
}



function loadUsers() {
  var st = App.users;
  var tbody = el('users-tbody');
  tbody.innerHTML = MD.loadingRow(9);
  var q = '/admin/users?page=' + st.page + '&size=' + st.size + (st.keyword ? '&keyword=' + encodeURIComponent(st.keyword) : '');
  MD.api.get(q).then(function (res) {
    var data = res.data || {}, items = data.items || [];
    if (st.status) items = items.filter(function (u) { return u.status === st.status; });
    if (!items.length) {
      tbody.innerHTML = emptyRow(9, 'users', '暂无用户数据', st.keyword ? '换个关键词试试' : '');
    } else {
      tbody.innerHTML = items.map(function (u) {
        var actions = '<div class="t-actions">' +
          '<button class="btn-link" onclick="viewUser(' + u.id + ')">' + MD.icon('eye', 14) + '详情</button>' +
          '<button class="btn-link" onclick="editUser(' + u.id + ')">' + MD.icon('edit', 14) + '编辑</button>' +
          (u.status === 'active'
            ? '<button class="btn-link btn-link--danger" onclick="changeUserStatus(' + u.id + ',\'disabled\')">' + MD.icon('x', 14) + '禁用</button>' +
              '<button class="btn-link" onclick="changeUserStatus(' + u.id + ',\'suspended\')">暂停</button>'
            : '<button class="btn-link" onclick="changeUserStatus(' + u.id + ',\'active\')">' + MD.icon('check', 14) + '启用</button>') +
          '</div>';
        return '<tr>' +
          '<td class="num">' + u.id + '</td>' +
          '<td class="mono">' + esc(u.email) + '</td>' +
          '<td>' + esc(u.nickname || '-') + '</td>' +
          '<td>' + (u.role === 'admin' ? '<span class="tag tag--info">管理员</span>' : '<span class="tag tag--gray">用户</span>') + '</td>' +
          '<td class="num">¥ ' + MD.fmtMoney(u.balance) + '</td>' +
          '<td>' + tagOf(REAL_NAME, u.real_name_status) + '</td>' +
          '<td>' + tagOf(USER_STATUS, u.status) + '</td>' +
          '<td class="mono">' + MD.fmtDate(u.created_at) + '</td>' +
          '<td>' + actions + '</td></tr>';
      }).join('');
    }
    el('users-count').textContent = countText(data.total || 0, data.page || st.page, data.size || st.size);
    MD.renderPager(el('users-pager'), data.page || st.page, data.total || 0, data.size || st.size, function (p) { st.page = p; loadUsers(); });
  }).catch(function (err) {
    MD.toast(err.message || '用户列表加载失败', 'error');
    tbody.innerHTML = emptyRow(9, 'alert', '加载失败', err.message);
  });
}
function changeUserStatus(id, status) {
  var label = (USER_STATUS[status] || [status])[0];
  var danger = status !== 'active';
  MD.confirm('确定将用户 #' + id + ' 的账号状态调整为「' + label + '」吗？该操作立即生效。', function () {
    MD.api.put('/admin/users/' + id + '/status', { status: status }).then(function () {
      MD.toast('用户状态已更新为「' + label + '」', 'success');
      loadUsers();
    }).catch(function (err) { MD.toast(err.message || '操作失败', 'error'); });
  }, { title: '变更账号状态', okLabel: '确定' + label, danger: danger });
}
function editUser(id) {
  MD.openModal({
    title: '编辑用户 #' + id, wide: '640px',
    body: '<div style="text-align:center;padding:30px;color:var(--text-3)"><span class="spinner"></span>加载中…</div>',
    buttons: []
  });
  MD.api.get('/admin/users/' + id).then(function (res) {
    var u = res.data || {};
    var body =
      '<div class="field"><label>邮箱</label><input class="input" id="eu-email" value="' + esc(u.email || '') + '" disabled></div>' +
      '<div class="field"><label>昵称</label><input class="input" id="eu-nickname" value="' + esc(u.nickname || '') + '" maxlength="50"></div>' +
      '<div class="field"><label>手机号</label><input class="input" id="eu-phone" value="' + esc(u.phone || '') + '"></div>' +
      '<div class="field"><label>积分余额调账（增减额度，可为负，如 +100 / -50）</label>' +
      '<input class="input" id="eu-balance" placeholder="如 +100 或 -50" type="text"></div>';
    MD.updateModal({
      body: body,
      buttons: [
        { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
        { label: '保存', cls: 'btn-primary', onClick: function () { saveEditUser(id); } }
      ]
    });
  }).catch(function (err) {
    MD.updateModal({ body: '<div style="padding:20px;color:var(--c-danger)">' + esc(err.message || '加载失败') + '</div>' });
  });
}

function saveEditUser(id) {
  var payload = {
    nickname: el('eu-nickname').value.trim(),
    phone: el('eu-phone').value.trim()
  };
  var bal = el('eu-balance').value.trim();
  if (bal) payload.balance_adjust = bal;
  MD.api.put('/admin/users/' + id, payload).then(function () {
    MD.toast('用户已更新', 'success');
    MD.closeModal();
    loadUsers();
  }).catch(function (err) { MD.toast(err.message || '保存失败', 'error'); });
}

function viewUser(id) {
  MD.openModal({
    title: '用户详情', wide: '640px',
    body: '<div style="text-align:center;padding:30px;color:var(--text-3)"><span class="spinner"></span>加载中…</div>',
    buttons: [{ label: '关闭', cls: 'btn-ghost', onClick: MD.closeModal }]
  });
  MD.api.get('/admin/users/' + id).then(function (res) {
    var u = res.data || {};
    var item = function (k, v, mono) {
      return '<div class="desc-item"><span class="k">' + k + '</span><span class="v' + (mono ? ' mono' : '') + '">' + (v === '' || v == null ? '-' : v) + '</span></div>';
    };
    var subs = u.subscriptions || [], keys = u.api_keys || [];
    var html = '<div class="desc-grid">' +
      item('用户 ID', u.id, true) +
      item('角色', u.role === 'admin' ? '管理员' : '普通用户') +
      item('邮箱', esc(u.email), true) +
      item('昵称', esc(u.nickname)) +
      item('手机号', esc(u.phone), true) +
      item('余额', '¥ ' + MD.fmtMoney(u.balance), true) +
      item('实名状态', tagOf(REAL_NAME, u.real_name_status)) +
      item('账号状态', tagOf(USER_STATUS, u.status)) +
      item('最近登录', MD.fmtDate(u.last_login_at), true) +
      item('登录 IP', esc(u.last_login_ip), true) +
      item('注册时间', MD.fmtDate(u.created_at), true) +
      '</div><div class="divider"></div>' +
      '<div class="card__title" style="margin-bottom:8px">订阅（' + subs.length + '）</div>' +
      (subs.length ? subs.map(function (s) {
        return '<div class="desc-item"><span class="k">' + esc(s.plan_name || ('订阅#' + s.id)) + '</span>' +
          '<span class="v">' + tagOf({ active: ['生效中', 'tag--success'], cancelled: ['已取消', 'tag--gray'], expired: ['已过期', 'tag--gray'] }, s.status) +
          ' <span style="color:var(--text-3);font-size:12px">至 ' + MD.fmtDate(s.end_at) + '</span></span></div>';
      }).join('') : '<div style="font-size:12px;color:var(--text-3)">暂无有效订阅</div>') +
      '<div class="card__title" style="margin:14px 0 8px">API Keys（' + keys.length + '）</div>' +
      (keys.length ? keys.map(function (k) {
        return '<div class="desc-item"><span class="k mono">' + esc(k.key_prefix) + '…</span>' +
          '<span class="v">' + esc(k.name) + ' <span style="color:var(--text-3);font-size:12px">最近使用 ' + MD.fmtDate(k.last_used_at) + '</span></span></div>';
      }).join('') : '<div style="font-size:12px;color:var(--text-3)">暂无 API Key</div>');
    MD.openModal({
      title: '用户详情 · #' + id, wide: '640px', body: html,
      buttons: [{ label: '关闭', cls: 'btn-ghost', onClick: MD.closeModal }]
    });
  }).catch(function (err) {
    MD.closeModal();
    MD.toast(err.message || '用户详情加载失败', 'error');
  });
}

/* =====================================================================
   视图 3 · 实名认证审核
   ===================================================================== */
function verifyTab(btn) {
  document.querySelectorAll('#verify-tabs .tab').forEach(function (t) { t.classList.remove('is-active'); });
  btn.classList.add('is-active');
  App.verify.status = btn.dataset.status || '';
  App.verify.page = 1;
  loadVerify();
}
var verifyCache = [];
function loadVerify() {
  var st = App.verify;
  var tbody = el('verify-tbody');
  tbody.innerHTML = MD.loadingRow(6);
  var q = '/admin/identity-verifications?page=' + st.page + '&size=' + st.size + (st.status ? '&status=' + st.status : '');
  MD.api.get(q).then(function (res) {
    var data = res.data || {};
    verifyCache = data.items || [];
    if (!verifyCache.length) {
      tbody.innerHTML = emptyRow(6, 'idcard', '暂无认证记录');
    } else {
      tbody.innerHTML = verifyCache.map(function (v) {
        var act = v.status === 'pending'
          ? '<button class="btn-link" onclick="openReview(' + v.id + ')">' + MD.icon('edit', 14) + '审核</button>'
          : '<button class="btn-link" onclick="openReview(' + v.id + ')">' + MD.icon('eye', 14) + '查看</button>';
        return '<tr>' +
          '<td class="num">#' + v.user_id + '<span class="td-sub">记录 ' + v.id + '</span></td>' +
          '<td>' + esc(v.real_name) + '</td>' +
          '<td class="mono">' + esc(maskIdNo(v.id_number)) + '</td>' +
          '<td class="mono">' + MD.fmtDate(v.created_at) + '</td>' +
          '<td>' + tagOf(VERIFY_ST, v.status) + '</td>' +
          '<td><div class="t-actions">' + act + '</div></td></tr>';
      }).join('');
    }
    el('verify-count').textContent = countText(data.total || 0, data.page || st.page, data.size || st.size);
    MD.renderPager(el('verify-pager'), data.page || st.page, data.total || 0, data.size || st.size, function (p) { st.page = p; loadVerify(); });
  }).catch(function (err) {
    MD.toast(err.message || '认证列表加载失败', 'error');
    tbody.innerHTML = emptyRow(6, 'alert', '加载失败', err.message);
  });
}
function openReview(id) {
  var rec = verifyCache.filter(function (v) { return v.id === id; })[0];
  if (!rec) { MD.toast('记录不存在，请刷新列表', 'warn'); return; }
  var pending = rec.status === 'pending';
  var item = function (k, v, mono) {
    return '<div class="desc-item"><span class="k">' + k + '</span><span class="v' + (mono ? ' mono' : '') + '">' + v + '</span></div>';
  };
  var body = '<div class="desc-grid">' +
    item('用户 ID', '#' + rec.user_id, true) +
    item('真实姓名', esc(rec.real_name)) +
    item('证件号码', esc(rec.id_number || '-'), true) +
    item('提交时间', MD.fmtDate(rec.created_at), true) +
    item('当前状态', tagOf(VERIFY_ST, rec.status)) +
    (rec.reviewed_at ? item('审核时间', MD.fmtDate(rec.reviewed_at), true) : '') +
    '</div>' +
    (rec.reject_reason ? '<div class="desc-item"><span class="k">拒绝理由</span><span class="v">' + esc(rec.reject_reason) + '</span></div>' : '') +
    (pending ? '<div class="divider"></div><div class="field" style="margin-bottom:0">' +
      '<label>审核备注 / 拒绝理由<span class="req" title="拒绝时必填">*</span></label>' +
      '<textarea class="textarea" id="review-reason" placeholder="通过可留空；拒绝时必须填写理由，将同步给用户"></textarea>' +
      '<div class="hint">审核结果立即生效，并同步更新用户实名状态。</div></div>' : '');
  var buttons = [{ label: '关闭', cls: 'btn-ghost', onClick: MD.closeModal }];
  if (pending) {
    buttons = [
      { label: '拒绝', cls: 'btn-danger btn-danger--solid', onClick: function () { submitReview(id, 'reject'); } },
      { label: '通过', cls: 'btn-primary', onClick: function () { submitReview(id, 'approve'); } }
    ];
  }
  MD.openModal({ title: '实名审核 · 记录 #' + id, wide: '560px', body: body, buttons: buttons });
}
function submitReview(id, action) {
  var reason = '';
  var ta = el('review-reason');
  if (ta) reason = ta.value.trim();
  if (action === 'reject' && !reason) { MD.toast('拒绝时必须填写理由', 'warn'); return; }
  var payload = { action: action, reason: reason, status: action === 'approve' ? 'approved' : 'rejected' };
  MD.api.post('/admin/identity-verifications/' + id + '/review', payload).then(function () {
    MD.toast(action === 'approve' ? '已通过该认证申请' : '已拒绝该认证申请', 'success');
    MD.closeModal();
    loadVerify();
  }).catch(function (err) { MD.toast(err.message || '审核失败', 'error'); });
}

/* =====================================================================
   视图 4 · 订单管理
   ===================================================================== */
function ordersQuery() {
  App.orders.page = 1;
  App.orders.type = el('order-type').value;
  App.orders.status = el('order-status').value;
  loadOrders();
}
function loadOrders() {
  var st = App.orders;
  var tbody = el('orders-tbody');
  tbody.innerHTML = MD.loadingRow(7);
  var q = '/admin/orders?page=' + st.page + '&size=' + st.size +
    (st.type ? '&type=' + st.type : '') + (st.status ? '&status=' + st.status : '');
  MD.api.get(q).then(function (res) {
    var data = res.data || {}, items = data.items || [];
    if (!items.length) {
      tbody.innerHTML = emptyRow(7, 'receipt', '暂无订单数据');
    } else {
      tbody.innerHTML = items.map(function (o) {
        var inbound = o.type === 'recharge' || o.type === 'subscription';
        var cls = inbound ? 'amt-in' : 'amt-out';
        var sign = inbound ? '+' : '-';
        return '<tr>' +
          '<td class="mono">' + esc(o.transaction_no) + '</td>' +
          '<td class="num">' + (o.user_id ? '#' + o.user_id : '—') + '</td>' +
          '<td>' + tagOf(TX_TYPE, o.type) + '</td>' +
          '<td><span class="num ' + cls + '">' + sign + ' ¥ ' + MD.fmtMoney(o.amount) + '</span></td>' +
          '<td>' + esc(o.payment_method || '-') + '</td>' +
          '<td>' + tagOf(TX_STATUS, o.status) + '</td>' +
          '<td class="mono">' + MD.fmtDate(o.created_at) + '</td></tr>';
      }).join('');
    }
    el('orders-count').textContent = countText(data.total || 0, data.page || st.page, data.size || st.size);
    MD.renderPager(el('orders-pager'), data.page || st.page, data.total || 0, data.size || st.size, function (p) { st.page = p; loadOrders(); });
  }).catch(function (err) {
    MD.toast(err.message || '订单列表加载失败', 'error');
    tbody.innerHTML = emptyRow(7, 'alert', '加载失败', err.message);
  });
}

/* =====================================================================
   视图 5 · 套餐管理
   ===================================================================== */
var plansCache = [];
function loadPlans() {
  var tbody = el('plans-tbody');
  tbody.innerHTML = MD.loadingRow(9);
  MD.api.get('/admin/plans').then(function (res) {
    plansCache = res.data || [];
    if (!plansCache.length) {
      tbody.innerHTML = emptyRow(9, 'card', '暂无套餐', '点击右上角「新建套餐」创建');
      return;
    }
    tbody.innerHTML = plansCache.map(function (p) {
      var status = p.status || 'active';
      return '<tr>' +
        '<td><b style="color:var(--text-1)">' + esc(p.name) + '</b><span class="td-sub">' + esc(p.description || '暂无描述') + '</span></td>' +
        '<td class="num">¥ ' + MD.fmtMoney(p.price) + '<span class="td-sub">' + esc(p.currency || 'CNY') + '</span></td>' +
        '<td class="num">' + MD.fmtNum(p.duration_days) + ' 天</td>' +
        '<td class="num">' + MD.fmtNum(p.rpm) + '</td>' +
        '<td class="num">' + MD.fmtNum(p.tpm) + '</td>' +
        '<td class="num">' + MD.fmtNum(p.included_tokens) + '</td>' +
        '<td>' + (status === 'active' ? '<span class="tag tag--success">上架中</span>' : '<span class="tag tag--gray">已下架</span>') + '</td>' +
        '<td class="num">' + (p.sort_order == null ? '-' : p.sort_order) + '</td>' +
        '<td><div class="t-actions">' +
          '<button class="btn-link" onclick="openPlanForm(' + p.id + ')">' + MD.icon('edit', 14) + '编辑</button>' +
          '<button class="btn-link btn-link--danger" onclick="deletePlan(' + p.id + ')">' + MD.icon('trash', 14) + '删除</button>' +
        '</div></td></tr>';
    }).join('');
  }).catch(function (err) {
    MD.toast(err.message || '套餐列表加载失败', 'error');
    tbody.innerHTML = emptyRow(9, 'alert', '加载失败', err.message);
  });
}
function openPlanForm(id) {
  var p = null;
  if (id) {
    p = plansCache.filter(function (x) { return x.id === id; })[0];
    if (!p) { MD.toast('套餐不存在，请刷新', 'warn'); return; }
  }
  var f = function (name, label, value, req, hint) {
    return '<div class="field"><label>' + label + (req ? '<span class="req">*</span>' : '') + '</label>' +
      '<input class="input" id="pf-' + name + '" value="' + esc(value == null ? '' : value) + '">' +
      (hint ? '<div class="hint">' + hint + '</div>' : '') + '</div>';
  };
  var body = '<div class="form-grid">' +
    f('name', '套餐名称', p && p.name, true) +
    f('price', '价格（元）', p && p.price, true) +
    '<div class="field full"><label>描述</label><input class="input" id="pf-description" value="' + esc(p ? p.description : '') + '" placeholder="一句话说明套餐定位"></div>' +
    f('duration', '时长（天）', p ? p.duration_days : 30, true) +
    f('sort', '排序', p && p.sort_order != null ? p.sort_order : 0, false, '数字越小越靠前') +
    f('rpm', 'RPM（每分钟请求）', p ? p.rpm : 60) +
    f('tpm', 'TPM（每分钟 Token）', p ? p.tpm : 100000) +
    f('tokens', '包含 Token', p ? p.included_tokens : 0) +
    f('concurrent', '并发数', p ? p.concurrent_limit : 10) +
    '<div class="field full"><label>模型权限</label>' +
      '<input class="input" id="pf-models" value="' + esc(p && p.model_access ? p.model_access.join(',') : '') + '" placeholder="多个模型用英文逗号分隔，如 gpt-4o,claude-3-5-sonnet">' +
      '<div class="hint">留空表示继承默认可用模型列表</div></div>' +
    '</div>';
  MD.openModal({
    title: id ? '编辑套餐' : '新建套餐', wide: '600px', body: body,
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '保存', cls: 'btn-primary', onClick: function () { savePlan(id); } }
    ]
  });
}
function planFormVal(name) { var e = el('pf-' + name); return e ? e.value.trim() : ''; }
function savePlan(id) {
  var name = planFormVal('name'), price = planFormVal('price'), duration = parseInt(planFormVal('duration'), 10);
  if (!name) { MD.toast('请填写套餐名称', 'warn'); return; }
  if (price === '' || isNaN(parseFloat(price)) || parseFloat(price) < 0) { MD.toast('请填写有效价格（0 表示免费）', 'warn'); return; }
  if (!duration || duration < 1) { MD.toast('请填写有效时长（天）', 'warn'); return; }
  var models = planFormVal('models');
  var body = {
    name: name,
    description: planFormVal('description'),
    price: String(parseFloat(price)),
    currency: 'CNY',
    duration_days: duration,
    rpm: parseInt(planFormVal('rpm'), 10) || 0,
    tpm: parseInt(planFormVal('tpm'), 10) || 0,
    included_tokens: parseInt(planFormVal('tokens'), 10) || 0,
    concurrent_limit: parseInt(planFormVal('concurrent'), 10) || 0,
    model_access: models ? models.split(',').map(function (s) { return s.trim(); }).filter(Boolean) : [],
    sort_order: parseInt(planFormVal('sort'), 10) || 0
  };
  var req = id ? MD.api.put('/admin/plans/' + id, body) : MD.api.post('/admin/plans', body);
  req.then(function () {
    MD.toast(id ? '套餐已更新' : '套餐已创建', 'success');
    MD.closeModal();
    loadPlans();
  }).catch(function (err) { MD.toast(err.message || '保存失败', 'error'); });
}
function deletePlan(id) {
  var p = plansCache.filter(function (x) { return x.id === id; })[0];
  MD.confirm('确定删除套餐「' + esc(p ? p.name : '#' + id) + '」吗？删除后不可恢复。', function () {
    MD.api.del('/admin/plans/' + id).then(function () {
      MD.toast('套餐已删除', 'success');
      loadPlans();
    }).catch(function (err) { MD.toast(err.message || '删除失败', 'error'); });
  }, { title: '删除套餐', okLabel: '删除', danger: true });
}

/* =====================================================================
   视图 · 模型渠道
   ===================================================================== */
var channelsCache = [];
function loadChannels() {
  var tbody = el('channels-tbody');
  tbody.innerHTML = MD.loadingRow(7);
  MD.api.get('/admin/channels').then(function (res) {
    channelsCache = res.data || [];
    if (!channelsCache.length) {
      tbody.innerHTML = emptyRow(7, 'card', '暂无渠道', '点击右上角「新建渠道」创建');
      return;
    }
    tbody.innerHTML = channelsCache.map(function (ch) {
      var typeTag = ch.type === 'anthropic'
        ? '<span class="tag tag--warning">Anthropic</span>'
        : '<span class="tag tag--info">OpenAI</span>';
      return '<tr>' +
        '<td><b style="color:var(--text-1)">' + esc(ch.name) + '</b>' +
          (ch.remark ? '<span class="td-sub">' + esc(ch.remark) + '</span>' : '') + '</td>' +
        '<td>' + typeTag + '</td>' +
        '<td class="mono">' + esc(ch.base_url || '-') + '</td>' +
        '<td class="num">' + ch.priority + '</td>' +
        '<td>' + (ch.enabled
          ? '<span class="tag tag--success">启用</span>'
          : '<span class="tag tag--gray">禁用</span>') + '</td>' +
        '<td><span class="td-sub">' + esc(ch.remark || '') + '</span></td>' +
        '<td><div class="t-actions">' +
          '<button class="btn-link" onclick="toggleChannel(' + ch.id + ')">' + (ch.enabled ? '停用' : '启用') + '</button>' +
          '<button class="btn-link" onclick="openChannelForm(' + ch.id + ')">' + MD.icon('edit', 14) + '编辑</button>' +
          '<button class="btn-link btn-link--danger" onclick="deleteChannel(' + ch.id + ')">' + MD.icon('trash', 14) + '删除</button>' +
        '</div></td></tr>';
    }).join('');
  }).catch(function (err) {
    MD.toast(err.message || '渠道列表加载失败', 'error');
    tbody.innerHTML = emptyRow(7, 'alert', '加载失败', err.message);
  });
}
function openChannelForm(id) {
  var ch = null;
  if (id) {
    ch = channelsCache.filter(function (x) { return x.id === id; })[0];
    if (!ch) { MD.toast('渠道不存在，请刷新', 'warn'); return; }
  }
  var f = function (name, label, value, req, hint) {
    return '<div class="field"><label>' + label + (req ? '<span class="req">*</span>' : '') + '</label>' +
      '<input class="input" id="cf-' + name + '" value="' + esc(value == null ? '' : value) + '">' +
      (hint ? '<div class="hint">' + hint + '</div>' : '') + '</div>';
  };
  var typeSel = '<div class="field"><label>提供商类型<span class="req">*</span></label>' +
    '<select class="input" id="cf-type">' +
      '<option value="openai"' + (ch && ch.type === 'openai' ? ' selected' : '') + '>OpenAI（含兼容网关）</option>' +
      '<option value="anthropic"' + (ch && ch.type === 'anthropic' ? ' selected' : '') + '>Anthropic</option>' +
    '</select></div>';
  var body = '<div class="form-grid">' +
    f('name', '渠道名称', ch && ch.name, true) +
    typeSel +
    '<div class="field full"><label>Base URL</label><input class="input" id="cf-base_url" value="' + esc(ch ? ch.base_url : '') + '" placeholder="如 https://api.openai.com 或中转网关地址"></div>' +
    '<div class="field full"><label>API Key</label><input class="input" id="cf-api_key" type="password" value="' + esc(ch ? ch.api_key : '') + '" autocomplete="new-password"></div>' +
    '<div class="field full" style="display:flex;align-items:center;gap:10px;flex-wrap:wrap;">' +
      '<button type="button" class="btn-ghost" id="cf-test-btn" onclick="testChannelConn()"><span id="cf-test-label">测试连接并获取模型</span></button>' +
      '<div class="hint" style="margin:0;">调用上游 /models 接口验证 Key 并自动填充模型列表</div>' +
    '</div>' +
    '<div class="field full"><div id="cf-test-status" class="cf-test-status"></div></div>' +
    '<div class="field full"><label>模型列表</label>' +
      '<input class="input" id="cf-models" value="' + esc(ch && ch.models ? ch.models.join(',') : '') + '" placeholder="英文逗号分隔，支持通配符，如 gpt-4o*,claude-3*">' +
      '<div class="hint">留空表示承接所有模型</div></div>' +
    f('priority', '优先级', ch ? ch.priority : 0, false, '数字越大越优先') +
    '<div class="field"><label>状态</label>' +
      '<label class="switch-row"><input type="checkbox" id="cf-enabled"' + (ch && ch.enabled ? ' checked' : '') + '><span class="switch"></span><span style="font-size:13px;">启用该渠道</span></label></div>' +
    '<div class="field full"><label>备注</label><input class="input" id="cf-remark" value="' + esc(ch ? ch.remark : '') + '" placeholder="用途说明"></div>' +
    '</div>';
  MD.openModal({
    title: id ? '编辑渠道' : '新建渠道', wide: '620px', body: body,
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '保存', cls: 'btn-primary', onClick: function () { saveChannel(id); } }
    ]
  });
}
function channelVal(name) { var e = el('cf-' + name); return e ? e.value.trim() : ''; }
function testChannelConn() {
  var type = (el('cf-type') && el('cf-type').value) || 'openai';
  var base = channelVal('base_url'), key = channelVal('api_key');
  if (!base) { MD.toast('请先填写 Base URL', 'warn'); return; }
  if (!key) { MD.toast('请先填写 API Key', 'warn'); return; }
  var btn = el('cf-test-btn'), label = el('cf-test-label'), status = el('cf-test-status');
  btn.disabled = true; label.textContent = '测试中…';
  if (status) status.innerHTML = '<span class="hint" style="color:var(--text-3)">正在请求 ' + esc(base.replace(/\/+$/, '')) + '/models …</span>';
  MD.api.post('/admin/channels/test', { type: type, base_url: base, api_key: key }).then(function (res) {
    var d = res.data || {};
    if (d.ok) {
      var models = d.models || [];
      var box = el('cf-models');
      if (box && models.length) {
        box.value = models.join(',');
        box.title = models.join(', ');
      }
      if (status) status.innerHTML = '<span class="hint" style="color:#2b8a3e">✓ 连接成功（' + d.latency_ms + 'ms），获取 ' + d.count + ' 个模型，已填充到模型列表</span>';
      MD.toast('连接成功，已填充 ' + d.count + ' 个模型', 'success');
    } else {
      if (status) status.innerHTML = '<span class="hint" style="color:#e03131">✗ ' + esc(d.error || '测试失败') + '</span>';
      MD.toast(d.error || '连接失败', 'error');
    }
  }).catch(function (err) {
    if (status) status.innerHTML = '<span class="hint" style="color:#e03131">✗ ' + esc(err.message || '测试失败') + '</span>';
    MD.toast(err.message || '测试失败', 'error');
  }).finally(function () {
    btn.disabled = false; label.textContent = '重新测试';
  });
}
function saveChannel(id) {
  var ch = null;
  if (id) {
    ch = channelsCache.filter(function (x) { return x.id === id; })[0];
    if (!ch) { MD.toast('渠道不存在，请刷新', 'warn'); return; }
  }
  var name = channelVal('name'), type = channelVal('type') || 'openai';
  if (!name) { MD.toast('请填写渠道名称', 'warn'); return; }
  if (!/^(openai|anthropic)$/.test(type)) { MD.toast('请选择有效的提供商类型', 'warn'); return; }
  var modelsInput = channelVal('models');
  var keyInput = channelVal('api_key');
  var key = '';
  if (keyInput && (!ch || keyInput !== ch.api_key)) key = keyInput;
  var body = {
    name: name,
    type: type,
    base_url: channelVal('base_url'),
    api_key: key,
    models: modelsInput ? modelsInput.split(',').map(function (s) { return s.trim(); }).filter(Boolean) : [],
    priority: parseInt(channelVal('priority'), 10) || 0,
    enabled: !!(el('cf-enabled') && el('cf-enabled').checked),
    remark: channelVal('remark')
  };
  var req = id ? MD.api.put('/admin/channels/' + id, body) : MD.api.post('/admin/channels', body);
  req.then(function () {
    MD.toast(id ? '渠道已更新' : '渠道已创建', 'success');
    MD.closeModal();
    loadChannels();
  }).catch(function (err) { MD.toast(err.message || '保存失败', 'error'); });
}
function toggleChannel(id) {
  var ch = channelsCache.filter(function (x) { return x.id === id; })[0];
  if (!ch) { MD.toast('渠道不存在，请刷新', 'warn'); return; }
  var next = !ch.enabled;
  var body = {
    name: ch.name, type: ch.type, base_url: ch.base_url, api_key: '',
    models: ch.models || [], priority: ch.priority, enabled: next, remark: ch.remark
  };
  MD.api.put('/admin/channels/' + id, body).then(function () {
    MD.toast(next ? '渠道已启用，流量将开始路由到此渠道' : '渠道已停用', 'success');
    loadChannels();
  }).catch(function (err) { MD.toast(err.message || '操作失败', 'error'); });
}
function deleteChannel(id) {
  var ch = channelsCache.filter(function (x) { return x.id === id; })[0];
  MD.confirm('确定删除渠道「' + esc(ch ? ch.name : '#' + id) + '」吗？删除后不可恢复。', function () {
    MD.api.del('/admin/channels/' + id).then(function () {
      MD.toast('渠道已删除', 'success');
      loadChannels();
    }).catch(function (err) { MD.toast(err.message || '删除失败', 'error'); });
  }, { title: '删除渠道', okLabel: '删除', danger: true });
}

/* =====================================================================
   视图 · 模型定价（分组倍率）
   ===================================================================== */
var pricingCache = [];
function renderPricingStatCards(list) {
  var box = el('pricing-usage');
  if (!box) return;
  var enabled = list.filter(function (g) { return g.enabled; }).length;
  var totalModels = 0;
  list.forEach(function (g) { totalModels += (g.models || []).length; });
  box.innerHTML =
    '<div class="card stat-card"><div class="stat-card__label">分组总数</div><div class="stat-card__value" style="color:var(--text-1)">' + list.length + '</div></div>' +
    '<div class="card stat-card"><div class="stat-card__label">启用分组</div><div class="stat-card__value" style="color:#2b8a3e">' + enabled + '</div></div>' +
    '<div class="card stat-card"><div class="stat-card__label">默认倍率（未匹配）</div><div class="stat-card__value" style="color:var(--text-2)">1.0</div></div>';
}
function loadPricingGroups() {
  var tbody = el('pricing-tbody');
  tbody.innerHTML = MD.loadingRow(5);
  MD.api.get('/admin/pricing-groups').then(function (res) {
    pricingCache = res.data || [];
    renderPricingStatCards(pricingCache);
    if (!pricingCache.length) {
      tbody.innerHTML = emptyRow(5, 'card', '暂无定价分组', '点击右上角「新建分组」配置倍率；未配置任何分组时所有模型按原价（倍率 1.0）计费');
      return;
    }
    tbody.innerHTML = pricingCache.map(function (g) {
      var key = g.id;
      return '<tr>' +
        '<td><b style="color:var(--text-1)">' + esc(g.name) + '</b></td>' +
        '<td><span class="tag tag--warning" style="font-weight:600;font-size:14px;">× ' + esc(g.multiplier) + '</span></td>' +
        '<td>' + (g.enabled
          ? '<span class="tag tag--success">启用</span>'
          : '<span class="tag tag--gray">停用</span>') + '</td>' +
        '<td><span class="td-sub">' + esc(g.remark || '') + '</span></td>' +
        '<td><div class="t-actions">' +
          '<button class="btn-link" onclick="togglePricingGroup(' + g.id + ')">' + (g.enabled ? '停用' : '启用') + '</button>' +
          '<button class="btn-link" onclick="openPricingGroupForm(' + g.id + ')">' + MD.icon('edit', 14) + '编辑</button>' +
          '<button class="btn-link btn-link--danger" onclick="deletePricingGroup(' + g.id + ')">' + MD.icon('trash', 14) + '删除</button>' +
        '</div></td></tr>';
    }).join('');
  }).catch(function (err) {
    MD.toast(err.message || '定价分组加载失败', 'error');
    tbody.innerHTML = emptyRow(5, 'alert', '加载失败', err.message);
  });
}
function openPricingGroupForm(id) {
  var g = null;
  if (id) {
    g = pricingCache.filter(function (x) { return x.id === id; })[0];
    if (!g) { MD.toast('分组不存在，请刷新', 'warn'); return; }
  }
  var f = function (name, label, value, req, hint) {
    return '<div class="field"><label>' + label + (req ? '<span class="req">*</span>' : '') + '</label>' +
      '<input class="input" id="pg-' + name + '" value="' + esc(value == null ? '' : value) + '">' +
      (hint ? '<div class="hint">' + hint + '</div>' : '') + '</div>';
  };
  var body = '<div class="form-grid">' +
    f('name', '分组名称', g && g.name, true, '如：标准 / 旗舰 / 企业') +
    f('multiplier', '倍率', g ? g.multiplier : '1.0', true, '正数，计费 = 模型原价 × 倍率，如 1.5 / 2.0') +
    '<div class="field full"><label>适用模型</label>' +
      '<input class="input" id="pg-models" value="' + esc(g && g.models ? g.models.join(',') : '') + '" placeholder="英文逗号分隔，支持通配符，如 gpt-4o*,claude-3*">' +
      '<div class="hint">留空表示适用于所有模型</div></div>' +
    '<div class="field"><label>状态</label>' +
      '<label class="switch-row"><input type="checkbox" id="pg-enabled"' + ((!g || g.enabled) ? ' checked' : '') + '><span class="switch"></span><span style="font-size:13px;">启用该分组</span></label></div>' +
    '<div class="field full"><label>备注</label><input class="input" id="pg-remark" value="' + esc(g ? g.remark : '') + '" placeholder="用途说明"></div>' +
    '</div>';
  MD.openModal({
    title: id ? '编辑定价分组' : '新建定价分组', wide: '620px', body: body,
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '保存', cls: 'btn-primary', onClick: function () { savePricingGroup(id); } }
    ]
  });
}
function pgVal(name) { var e = el('pg-' + name); return e ? e.value.trim() : ''; }
function savePricingGroup(id) {
  var name = pgVal('name'), mult = pgVal('multiplier');
  if (!name) { MD.toast('请填写分组名称', 'warn'); return; }
  if (!mult || isNaN(Number(mult)) || Number(mult) <= 0) { MD.toast('请填写有效的正数倍率', 'warn'); return; }
  var modelsInput = pgVal('models');
  var body = {
    name: name,
    multiplier: mult,
    models: modelsInput ? modelsInput.split(',').map(function (s) { return s.trim(); }).filter(Boolean) : [],
    enabled: !!(el('pg-enabled') && el('pg-enabled').checked),
    remark: pgVal('remark')
  };
  var req = id ? MD.api.put('/admin/pricing-groups/' + id, body) : MD.api.post('/admin/pricing-groups', body);
  req.then(function () {
    MD.toast(id ? '分组已更新，新倍率立即生效' : '分组已创建，立即生效', 'success');
    MD.closeModal();
    loadPricingGroups();
  }).catch(function (err) { MD.toast(err.message || '保存失败', 'error'); });
}
function togglePricingGroup(id) {
  var g = pricingCache.filter(function (x) { return x.id === id; })[0];
  if (!g) { MD.toast('分组不存在，请刷新', 'warn'); return; }
  var next = !g.enabled;
  var body = { name: g.name, multiplier: g.multiplier, models: g.models || [], enabled: next, remark: g.remark };
  MD.api.put('/admin/pricing-groups/' + id, body).then(function () {
    MD.toast(next ? '分组已启用，倍率立即生效' : '分组已停用', 'success');
    loadPricingGroups();
  }).catch(function (err) { MD.toast(err.message || '操作失败', 'error'); });
}
function deletePricingGroup(id) {
  var g = pricingCache.filter(function (x) { return x.id === id; })[0];
  MD.confirm('确定删除定价分组「' + esc(g ? g.name : '#' + id) + '」吗？删除后该分组覆盖的模型将回到默认倍率 1.0。', function () {
    MD.api.del('/admin/pricing-groups/' + id).then(function () {
      MD.toast('分组已删除', 'success');
      loadPricingGroups();
    }).catch(function (err) { MD.toast(err.message || '删除失败', 'error'); });
  }, { title: '删除定价分组', okLabel: '删除', danger: true });
}

/* =====================================================================
   视图 · 通知中心（发送 / 列表）
   ===================================================================== */
var notifPage = 1;
function loadNotifications(page) {
  notifPage = page || 1;
  var tbody = el('notif-tbody');
  tbody.innerHTML = MD.loadingRow(7);
  MD.api.get('/admin/notifications?page=' + notifPage + '&size=20').then(function (res) {
    if (!res) { tbody.innerHTML = ''; return; }
    var d = res.data || {}, items = d.items || [];
    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="7" style="padding:0;border:0;">' +
        emptyRow(7, 'bell', '暂无通知', '点击右上角「发送通知」向用户发送站内通知') + '</td></tr>';
      return;
    }
    tbody.innerHTML = items.map(function (n) {
      var typeTag = n.type === 'announcement'
        ? '<span class="tag tag--warn">公告</span>'
        : '<span class="tag tag--info">系统</span>';
      var readTag = n.is_read
        ? '<span class="tag tag--gray">已读</span>'
        : '<span class="tag tag--success">未读</span>';
      return '<tr>' +
        '<td>' + n.id + '</td>' +
        '<td><b style="color:var(--text-1)">#' + n.user_id + '</b> <span class="td-sub">' + esc(n.user_email || '') + '</span></td>' +
        '<td><b style="color:var(--text-1)">' + esc(n.title) + '</b></td>' +
        '<td><span class="td-sub" style="max-width:320px;display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="' + esc(n.content) + '">' + esc(n.content) + '</span></td>' +
        '<td>' + typeTag + '</td>' +
        '<td>' + readTag + '</td>' +
        '<td>' + MD.fmtDate(n.created_at) + '</td></tr>';
    }).join('');
    MD.renderPager(el('notif-pager'), notifPage, d.total || 0, 20, loadNotifications);
  }).catch(function (err) {
    tbody.innerHTML = emptyRow(7, 'alert', '加载失败', err.message);
  });
}
function openSendNotifForm() {
  MD.openModal({
    title: '发送站内通知',
    body:
      '<div class="field"><label>发送对象<span class="req">*</span></label>' +
      '<div class="model-checks">' +
      '<label class="model-check"><input type="radio" name="nt-target" value="specific" checked>指定用户</label>' +
      '<label class="model-check"><input type="radio" name="nt-target" value="all">全员发放</label>' +
      '</div>' +
      '<div class="hint">全员发放：所有状态为 active 的账号</div></div>' +
      '<div class="field" id="nt-uid-field"><label>用户 ID<span class="req">*</span></label>' +
      '<input class="input" id="nt-uid" type="number" min="1" placeholder="如 1"></div>' +
      '<div class="field"><label>通知类型</label>' +
      '<div class="model-checks">' +
      '<label class="model-check"><input type="radio" name="nt-type" value="system" checked>系统消息</label>' +
      '<label class="model-check"><input type="radio" name="nt-type" value="announcement">平台公告</label>' +
      '</div></div>' +
      '<div class="field"><label>标题<span class="req">*</span></label><input class="input" id="nt-title" maxlength="100" placeholder="例如：8 月活动补偿已发放"></div>' +
      '<div class="field"><label>内容<span class="req">*</span></label><textarea class="input" id="nt-content" rows="4" maxlength="1900" placeholder="通知正文，支持换行"></textarea></div>',
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '确认发送', cls: 'btn-primary', onClick: submitSendNotif }
    ]
  });
  var radios = document.querySelectorAll('input[name="nt-target"]');
  radios.forEach(function (r) {
    r.addEventListener('change', function () {
      el('nt-uid-field').style.display = r.value === 'specific' ? '' : 'none';
      el('nt-uid').required = r.value === 'specific';
    });
  });
}
function submitSendNotif() {
  var targetEl = document.querySelector('input[name="nt-target"]:checked');
  var target = targetEl ? targetEl.value : 'specific';
  var uid = 0;
  if (target === 'specific') {
    uid = parseInt((el('nt-uid').value || '').trim(), 10);
    if (!uid || uid <= 0) { MD.toast('请输入有效的用户 ID', 'warn'); return; }
  }
  var title = (el('nt-title').value || '').trim();
  var content = (el('nt-content').value || '').trim();
  if (!title) { MD.toast('请输入通知标题', 'warn'); return; }
  if (!content) { MD.toast('请输入通知内容', 'warn'); return; }
  var ntype = document.querySelector('input[name="nt-type"]:checked');
  var btn = document.querySelector('.modal__foot .btn-primary');
  if (btn) { btn.disabled = true; btn.textContent = '发送中…'; }
  MD.api.post('/admin/notifications', { user_id: uid, title: title, content: content, type: ntype ? ntype.value : 'system' }).then(function (res) {
    var d = res.data || {};
    MD.toast('已发送 ' + d.issued + ' 条通知' + (d.target === 'all' ? '（全员）' : ''), 'success');
    MD.closeModal();
    loadNotifications(1);
  }).catch(function (e) {
    MD.toast(e.message || '发送失败', 'error');
  }).finally(function () {
    if (btn) { btn.disabled = false; btn.textContent = '确认发送'; }
  });
}

/* =====================================================================
   视图 · Token 授信审核
   ===================================================================== */
var creditPage = 1;
function loadCreditApps(page) {
  creditPage = page || 1;
  var tbody = el('credit-tbody');
  tbody.innerHTML = MD.loadingRow(8);
  var status = (el('credit-status-filter') && el('credit-status-filter').value) || '';
  MD.api.get('/admin/credit-applications?page=' + creditPage + '&size=20' + (status ? '&status=' + status : '')).then(function (res) {
    if (!res) { tbody.innerHTML = ''; return; }
    var d = res.data || {}, items = d.items || [];
    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="8" style="padding:0;border:0;">' +
        emptyRow(8, 'card', '暂无授信申请', '用户累计消费满 ¥5000 后提交的授信申请会展示在这里') + '</td></tr>';
      return;
    }
    tbody.innerHTML = items.map(function (a) {
      var stMap = { pending: ['待审核', 'tag--warn'], approved: ['已通过', 'tag--success'], rejected: ['已驳回', 'tag--danger'] };
      var st = stMap[a.status] || ['未知', 'tag--gray'];
      var quota = a.status === 'approved'
        ? '<b style="font-family:var(--font-num);color:var(--c-success);">' + MD.fmtNum(a.granted_tokens) + '</b> Tokens'
        : (a.status === 'rejected' ? '<span class="td-sub" style="color:var(--c-danger);">' + esc(a.reject_reason || '') + '</span>' : '—');
      var actions = '';
      if (a.status === 'pending') {
        actions = '<button class="btn-ghost btn-sm" onclick="openApproveCredit(' + a.id + ')">通过</button>' +
          '<button class="btn-ghost btn-sm" style="color:#e5484d;" onclick="openRejectCredit(' + a.id + ')">驳回</button>';
      }
      if (a.status === 'approved') {
        if (a.credit_used > 0) {
          actions = '<button class="btn-ghost btn-sm" onclick="openCollectCredit(' + a.user_id + ')">催账</button>' +
            '<button class="btn-ghost btn-sm" style="color:#e5484d;" onclick="banCreditUser(' + a.user_id + ')">封号</button>';
        } else {
          actions = '<span class="td-sub">无欠款</span>';
        }
      }
      var usedCell = a.status === 'approved'
        ? (a.credit_used > 0
          ? '<b style="font-family:var(--font-num);color:#b45309;">' + MD.fmtNum(a.credit_used) + '</b>'
          : '<span style="color:var(--text-4);">0</span>')
        : '—';
      var availCell = a.status === 'approved'
        ? '<b style="font-family:var(--font-num);">' + MD.fmtNum(a.credit_available) + '</b>'
        : '—';
      return '<tr>' +
        '<td>' + a.id + '</td>' +
        '<td><b style="color:var(--text-1)">#' + a.user_id + '</b> <span class="td-sub">' + esc(a.user_email || '') + '</span></td>' +
        '<td class="num">¥' + MD.fmtMoney(a.consumed_total) + '</td>' +
        '<td><span class="tag ' + st[1] + '">' + st[0] + '</span></td>' +
        '<td>' + quota + '</td>' +
        '<td>' + usedCell + '</td>' +
        '<td>' + availCell + '</td>' +
        '<td>' + MD.fmtDate(a.created_at) + '</td>' +
        '<td class="row-actions">' + actions + '</td></tr>';
    }).join('');
    MD.renderPager(el('credit-pager'), creditPage, d.total || 0, 20, loadCreditApps);
  }).catch(function (err) {
    tbody.innerHTML = emptyRow(8, 'alert', '加载失败', err.message);
  });
}
function openApproveCredit(id) {
  MD.openModal({
    title: '通过授信申请 #' + id,
    body:
      '<div class="field"><label>授信 Token 额度<span class="req">*</span></label>' +
      '<input class="input input--mono" id="credit-tokens" type="number" min="1" step="1" placeholder="如 1000000">' +
      '<div class="hint">通过后该用户获得对应 Token 授信额度，调用时余额不足将自动垫付并计入待还额度</div></div>',
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '确认通过', cls: 'btn-primary', onClick: function () {
        var tokens = parseInt((el('credit-tokens').value || '').trim(), 10);
        if (!tokens || tokens <= 0) { MD.toast('请填写有效的授信额度', 'warn'); return; }
        MD.api.post('/admin/credit-applications/' + id + '/approve', { granted_tokens: tokens }).then(function () {
          MD.toast('已通过，授信额度已发放', 'success');
          MD.closeModal();
          loadCreditApps(creditPage);
        }).catch(function (e) { MD.toast(e.message || '操作失败', 'error'); });
      } }
    ]
  });
}
function openRejectCredit(id) {
  MD.openModal({
    title: '驳回授信申请 #' + id,
    body:
      '<div class="field"><label>驳回原因<span class="req">*</span></label>' +
      '<textarea class="input" id="credit-reject-reason" rows="3" maxlength="500" placeholder="如：消费记录不足，请继续使用后再申请"></textarea></div>',
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '确认驳回', cls: 'btn-primary', onClick: function () {
        var reason = (el('credit-reject-reason').value || '').trim();
        if (!reason) { MD.toast('请填写驳回原因', 'warn'); return; }
        MD.api.post('/admin/credit-applications/' + id + '/reject', { reason: reason }).then(function () {
          MD.toast('已驳回', 'success');
          MD.closeModal();
          loadCreditApps(creditPage);
        }).catch(function (e) { MD.toast(e.message || '操作失败', 'error'); });
      } }
    ]
  });
}
function openCollectCredit(userId) {
  MD.openModal({
    title: '催账 #' + userId,
    body:
      '<div class="field"><label>催账说明（选填）</label>' +
      '<textarea class="input" id="collect-note" rows="3" maxlength="500" placeholder="如：请于 3 日内归还授信额度"></textarea>' +
      '<div class="hint">将发送站内催款通知并记录催账日志</div></div>',
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '发送催账', cls: 'btn-primary', onClick: function () {
        var note = (el('collect-note').value || '').trim();
        MD.api.post('/admin/credit-collect', { user_id: userId, note: note }).then(function () {
          MD.toast('催账已发送', 'success');
          MD.closeModal();
          loadCollections(1);
        }).catch(function (e) { MD.toast(e.message || '催账失败', 'error'); });
      } }
    ]
  });
}
function banCreditUser(userId) {
  MD.confirm('确定封禁该用户？封禁后其账号将无法登录与调用 API，须管理员手动解封。', function () {
    MD.api.put('/admin/users/' + userId + '/status', { status: 'disabled' }).then(function () {
      MD.toast('已封禁该用户', 'success');
      loadCreditApps(creditPage);
    }).catch(function (e) { MD.toast(e.message || '操作失败', 'error'); });
  }, { title: '封号确认', okLabel: '确认封号', danger: true });
}
var collectPage = 1;
function loadCollections(page) {
  collectPage = page || 1;
  var box = el('collect-list');
  if (!box) return;
  MD.api.get('/admin/credit-collections?page=' + collectPage + '&size=20').then(function (res) {
    if (!res) { box.innerHTML = ''; return; }
    var d = res.data || {}, items = d.items || [];
    MD.renderPager(el('collect-pager'), collectPage, d.total || 0, 20, loadCollections);
    if (!items.length) {
      box.innerHTML = MD.emptyState('clock', '暂无催账记录', '对欠款用户发送催账后，记录会展示在这里');
      return;
    }
    box.innerHTML = items.map(function (c) {
      return '<div class="notif-card">' +
        '<div class="notif-card__head"><span class="notif-card__title">催账 #' + c.id + ' · ' + esc(c.user_email || '#' + c.user_id) + '</span>' +
        '<span class="tag tag--warn">待还 ' + MD.fmtNum(c.tokens_due) + '</span></div>' +
        '<div class="notif-card__content">' + esc(c.note || '') + '</div>' +
        '<div class="notif-card__time">' + MD.fmtDate(c.created_at) + '</div></div>';
    }).join('');
  }).catch(function (err) {
    box.innerHTML = '<div class="hint">加载失败：' + esc(err.message) + '</div>';
  });
}

/* =====================================================================
   视图 · 支付接口（易支付）
   ===================================================================== */
function loadPayPage() {
  MD.api.get('/admin/config').then(function (res) {
    var list = res.data || [];
    var v = function (key) {
      var c = list.filter(function (x) { return x.key === key; })[0];
      return c ? String(c.value == null ? '' : c.value) : '';
    };
    el('pay-epay-enabled').value = v('pay_epay_enabled') === 'true' ? 'true' : 'false';
    el('pay-epay-gateway').value = v('pay_epay_gateway');
    el('pay-epay-pid').value = v('pay_epay_pid');
    el('pay-epay-key').value = v('pay_epay_key');
    el('pay-epay-sign-upper').value = v('pay_epay_sign_upper') !== 'false' ? 'true' : 'false';
    el('pay-epay-default-type').value = v('pay_epay_default_type') || 'alipay';
    var pill = el('pay-status-pill');
    var enabled = v('pay_epay_enabled') === 'true';
    var gateway = v('pay_epay_gateway');
    var pid = v('pay_epay_pid');
    var key = v('pay_epay_key');
    if (enabled && gateway && pid && key) {
      pill.className = 'health-pill health-pill--ok';
      pill.innerHTML = '<span data-icon="check:14"></span>已启用 · 配置完整';
    } else if (enabled) {
      pill.className = 'health-pill health-pill--warn';
      pill.innerHTML = '<span data-icon="alert:14"></span>已启用 · 配置不完整';
    } else {
      pill.className = 'health-pill health-pill--info';
      pill.innerHTML = '<span data-icon="card:14"></span>未启用';
    }
  }).catch(function (err) {
    MD.toast(err.message || '配置加载失败', 'error');
  });
}
function savePayConfig() {
  var items = [
    { key: 'pay_epay_enabled', value: el('pay-epay-enabled').value },
    { key: 'pay_epay_gateway', value: (el('pay-epay-gateway').value || '').trim().replace(/\/+$/, '') },
    { key: 'pay_epay_pid', value: (el('pay-epay-pid').value || '').trim() },
    { key: 'pay_epay_key', value: (el('pay-epay-key').value || '').trim() },
    { key: 'pay_epay_sign_upper', value: el('pay-epay-sign-upper').value },
    { key: 'pay_epay_default_type', value: el('pay-epay-default-type').value }
  ];
  var enabled = items[0].value === 'true';
  if (enabled) {
    if (!items[1].value) { MD.toast('启用易支付前请填写网关地址', 'warn'); return; }
    if (!items[2].value) { MD.toast('启用易支付前请填写商户 ID (PID)', 'warn'); return; }
    if (!items[3].value) { MD.toast('启用易支付前请填写商户密钥 (KEY)', 'warn'); return; }
  }
  MD.api.put('/admin/config/batch', { group: 'epay', items: items }).then(function () {
    MD.toast('支付接口配置已保存', 'success');
    loadPayPage();
  }).catch(function (err) { MD.toast(err.message || '保存失败', 'error'); });
}

/* =====================================================================
   视图 · 发票审核
   ===================================================================== */
var invPage = 1;
function loadInvoices(page) {
  invPage = page || 1;
  var tbody = el('inv-tbody');
  tbody.innerHTML = MD.loadingRow(9);
  var status = (el('inv-status-filter') && el('inv-status-filter').value) || '';
  MD.api.get('/admin/invoices?page=' + invPage + '&size=20' + (status ? '&status=' + status : '')).then(function (res) {
    if (!res) { tbody.innerHTML = ''; return; }
    var d = res.data || {}, items = d.items || [];
    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="9" style="padding:0;border:0;">' +
        emptyRow(9, 'card', '暂无发票申请', '用户提交开票申请后会展示在这里') + '</td></tr>';
      return;
    }
    tbody.innerHTML = items.map(function (inv) {
      var stMap = { pending: ['待审核', 'tag--warn'], issued: ['已开具', 'tag--success'], rejected: ['已驳回', 'tag--danger'] };
      var st = stMap[inv.status] || ['未知', 'tag--gray'];
      var type = (inv.title_type === 'company' ? '企业' : '个人') + ' · ' + (inv.invoice_type === 'vat' ? '专票' : '普票');
      var detail = '<div class="td-sub">' + esc(type) + (inv.tax_no ? ' · 税号 ' + esc(inv.tax_no) : '') + '</div>' +
        (inv.email ? '<div class="td-sub">收件 ' + esc(inv.email) + '</div>' : '') +
        (inv.bank_name ? '<div class="td-sub">' + esc(inv.bank_name) + ' ' + esc(inv.bank_account || '') + '</div>' : '') +
        (inv.reject_reason ? '<div class="td-sub" style="color:var(--c-danger);">驳回原因：' + esc(inv.reject_reason) + '</div>' : '');
      var actions = '';
      if (inv.status === 'pending') {
        actions = '<button class="btn-ghost btn-sm" onclick="openIssueInv(' + inv.id + ')">开具</button>' +
          '<button class="btn-ghost btn-sm" style="color:#e5484d;" onclick="openRejectInv(' + inv.id + ')">驳回</button>';
      } else if (inv.status === 'issued') {
        actions = '<button class="btn-ghost btn-sm" onclick="showInvDetail(' + inv.id + ')">查看</button>';
      }
      return '<tr>' +
        '<td>' + inv.id + '</td>' +
        '<td><b style="color:var(--text-1)">#' + inv.user_id + '</b> <span class="td-sub">' + esc(inv.user_email || '') + '</span></td>' +
        '<td><b style="color:var(--text-1)">' + esc(inv.title) + '</b>' + detail + '</td>' +
        '<td><span class="td-sub">' + type + '</span></td>' +
        '<td class="num">¥' + MD.fmtMoney(inv.amount) + '</td>' +
        '<td><span class="tag ' + st[1] + '">' + st[0] + '</span></td>' +
        '<td><span class="mono">' + esc(inv.invoice_no || '-') + '</span></td>' +
        '<td>' + MD.fmtDate(inv.created_at) + '</td>' +
        '<td class="row-actions">' + actions + '</td></tr>';
    }).join('');
    MD.renderPager(el('inv-pager'), invPage, d.total || 0, 20, loadInvoices);
  }).catch(function (err) {
    tbody.innerHTML = emptyRow(9, 'alert', '加载失败', err.message);
  });
}
function openIssueInv(id) {
  MD.openModal({
    title: '开具发票 #' + id,
    body:
      '<div class="field"><label>发票号码<span class="req">*</span></label>' +
      '<input class="input" id="inv-no" maxlength="100" placeholder="如 FP2026XXXXXXXX"><div class="hint">请核对申请信息后填写实际开具的发票号码</div></div>',
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '确认开具', cls: 'btn-primary', onClick: function () {
        var no = (el('inv-no').value || '').trim();
        if (!no) { MD.toast('请填写发票号码', 'warn'); return; }
        MD.api.post('/admin/invoices/' + id + '/issue', { invoice_no: no }).then(function () {
          MD.toast('已开具', 'success');
          MD.closeModal();
          loadInvoices(invPage);
        }).catch(function (e) { MD.toast(e.message || '操作失败', 'error'); });
      } }
    ]
  });
}
function openRejectInv(id) {
  MD.openModal({
    title: '驳回发票申请 #' + id,
    body:
      '<div class="field"><label>驳回原因<span class="req">*</span></label>' +
      '<textarea class="input" id="inv-reject-reason" rows="3" maxlength="500" placeholder="如：抬头信息有误，请重新提交"></textarea></div>',
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '确认驳回', cls: 'btn-primary', onClick: function () {
        var reason = (el('inv-reject-reason').value || '').trim();
        if (!reason) { MD.toast('请填写驳回原因', 'warn'); return; }
        MD.api.post('/admin/invoices/' + id + '/reject', { reason: reason }).then(function () {
          MD.toast('已驳回', 'success');
          MD.closeModal();
          loadInvoices(invPage);
        }).catch(function (e) { MD.toast(e.message || '操作失败', 'error'); });
      } }
    ]
  });
}
function showInvDetail() {
  MD.toast('请参考表格中的发票信息', 'info');
}

/* =====================================================================
   视图 · 加油包管理（CRUD）
   ===================================================================== */
var pkgCache = [];
function loadTokenPackages() {
  var tbody = el('pkg-tbody');
  tbody.innerHTML = MD.loadingRow(8);
  MD.api.get('/admin/token-packages').then(function (res) {
    pkgCache = (res && res.data) || [];
    if (!pkgCache.length) {
      tbody.innerHTML = '<tr><td colspan="8" style="padding:0;border:0;">' +
        emptyRow(8, 'card', '暂无加油包', '点击右上角「新建加油包」创建') + '</td></tr>';
      return;
    }
    tbody.innerHTML = pkgCache.map(function (p) {
      var statusTag = p.status === 'active'
        ? '<span class="tag tag--success">启用</span>'
        : '<span class="tag tag--gray">停用</span>';
      return '<tr>' +
        '<td>' + p.sort_order + '</td>' +
        '<td><b style="color:var(--text-1)">' + esc(p.name) + '</b></td>' +
        '<td><span class="td-sub">' + esc(p.description || '-') + '</span></td>' +
        '<td class="num">' + MD.fmtNum(p.tokens) + '</td>' +
        '<td class="num">' + (p.bonus_tokens ? MD.fmtNum(p.bonus_tokens) : '-') + '</td>' +
        '<td class="num">¥' + MD.fmtMoney(p.price) + '</td>' +
        '<td>' + statusTag + '</td>' +
        '<td class="row-actions">' +
        '<button class="btn-ghost btn-sm" onclick="openPkgForm(' + p.id + ')">编辑</button>' +
        (p.status === 'active'
          ? '<button class="btn-ghost btn-sm" onclick="togglePkg(' + p.id + ',\'inactive\')">停用</button>'
          : '<button class="btn-ghost btn-sm" onclick="togglePkg(' + p.id + ',\'active\')">启用</button>') +
        '<button class="btn-ghost btn-sm" style="color:#e5484d;" onclick="delPkg(' + p.id + ')">删除</button>' +
        '</td></tr>';
    }).join('');
  }).catch(function (err) {
    tbody.innerHTML = emptyRow(8, 'alert', '加载失败', err.message);
  });
}
function openPkgForm(id) {
  var p = null;
  if (id) { for (var i = 0; i < pkgCache.length; i++) if (pkgCache[i].id === id) { p = pkgCache[i]; break; } }
  MD.openModal({
    title: p ? '编辑加油包' : '新建加油包',
    body:
      '<div class="field"><label>名称<span class="req">*</span></label><input class="input" id="pkg-name" maxlength="100" value="' + (p ? esc(p.name) : '') + '" placeholder="如：体验包"></div>' +
      '<div class="field"><label>描述</label><input class="input" id="pkg-desc" maxlength="500" value="' + (p ? esc(p.description || '') : '') + '" placeholder="一句话介绍"></div>' +
      '<div class="field"><label>Token 数<span class="req">*</span></label><input class="input" id="pkg-tokens" type="number" min="1" value="' + (p ? p.tokens : '') + '" placeholder="如 100000"></div>' +
      '<div class="field"><label>赠送 Token</label><input class="input" id="pkg-bonus" type="number" min="0" value="' + (p ? p.bonus_tokens || 0 : 0) + '"></div>' +
      '<div class="field"><label>价格（元）<span class="req">*</span></label><input class="input" id="pkg-price" type="number" min="0.01" step="0.01" value="' + (p ? p.price : '') + '" placeholder="如 10"></div>' +
      '<div class="field"><label>排序（小在前）</label><input class="input" id="pkg-sort" type="number" value="' + (p ? p.sort_order : 0) + '"></div>' +
      '<div class="field"><label>状态</label>' +
      '<div class="model-checks">' +
      '<label class="model-check"><input type="radio" name="pkg-status" value="active"' + (!p || p.status === 'active' ? ' checked' : '') + '>启用</label>' +
      '<label class="model-check"><input type="radio" name="pkg-status" value="inactive"' + (p && p.status === 'inactive' ? ' checked' : '') + '>停用</label>' +
      '</div></div>',
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: p ? '保存修改' : '创建', cls: 'btn-primary', onClick: function () { submitPkg(id); } }
    ]
  });
}
function submitPkg(id) {
  var name = (el('pkg-name').value || '').trim();
  var tokens = parseInt((el('pkg-tokens').value || '').trim(), 10);
  var price = parseFloat((el('pkg-price').value || '').trim());
  if (!name) { MD.toast('请输入名称', 'warn'); return; }
  if (!tokens || tokens <= 0) { MD.toast('请输入有效的 Token 数', 'warn'); return; }
  if (!price || price <= 0) { MD.toast('请输入有效的价格', 'warn'); return; }
  var statusEl = document.querySelector('input[name="pkg-status"]:checked');
  var body = {
    name: name,
    description: (el('pkg-desc').value || '').trim(),
    tokens: tokens,
    bonus_tokens: parseInt((el('pkg-bonus').value || '0').trim(), 10) || 0,
    price: price,
    sort_order: parseInt((el('pkg-sort').value || '0').trim(), 10) || 0,
    status: statusEl ? statusEl.value : 'active'
  };
  var req = id ? MD.api.put('/admin/token-packages/' + id, body) : MD.api.post('/admin/token-packages', body);
  req.then(function (res) {
    MD.toast(id ? '已保存修改' : '已创建加油包', 'success');
    MD.closeModal();
    loadTokenPackages();
  }).catch(function (e) {
    MD.toast(e.message || '保存失败', 'error');
  });
}
function togglePkg(id, status) {
  MD.api.put('/admin/token-packages/' + id + '/status', { status: status }).then(function (res) {
    MD.toast(status === 'active' ? '已启用' : '已停用', 'success');
    loadTokenPackages();
  }).catch(function (e) {
    MD.toast(e.message || '操作失败', 'error');
  });
}
function delPkg(id) {
  MD.confirm('确定删除该加油包吗？<br><span class="hint">删除后不可恢复，已购用户不受影响</span>', function () {
    MD.api.del('/admin/token-packages/' + id).then(function (res) {
      MD.toast('已删除', 'success');
      loadTokenPackages();
    }).catch(function (e) {
      MD.toast(e.message || '删除失败', 'error');
    });
  }, { title: '删除加油包', okLabel: '删除', danger: true, html: true });
}

/* =====================================================================
   视图 · 重置券（发放 / 列表）
   ===================================================================== */
var rcPage = 1;
function loadResetCoupons(page) {
  rcPage = page || 1;
  var tbody = el('rc-tbody');
  tbody.innerHTML = MD.loadingRow(6);
  var q = (el('rc-user-filter') && el('rc-user-filter').value.trim()) || '';
  MD.api.get('/admin/reset-coupons?page=' + rcPage + '&size=20' + (q ? '&user_id=' + encodeURIComponent(q) : '')).then(function (res) {
    if (!res) { tbody.innerHTML = ''; return; }
    var d = res.data || {}, items = d.items || [];
    if (!items.length) {
      tbody.innerHTML = '<tr><td colspan="6" style="padding:0;border:0;">' +
        emptyRow(6, 'card', '暂无重置券', '点击右上角「发放重置券」发放') + '</td></tr>';
      return;
    }
    tbody.innerHTML = items.map(function (c) {
      var statusTag = c.status === 'used'
        ? '<span class="tag tag--gray">已使用</span>'
        : '<span class="tag tag--success">未使用</span>';
      return '<tr>' +
        '<td>' + c.id + '</td>' +
        '<td class="mono">' + esc(c.code) + '</td>' +
        '<td><b style="color:var(--text-1)">#' + c.user_id + '</b> <span class="td-sub">' + esc(c.user_email || '') + '</span></td>' +
        '<td>' + statusTag + '</td>' +
        '<td><span class="td-sub">' + esc(c.note || '-') + '</span></td>' +
        '<td>' + (c.used_at ? MD.fmtDate(c.used_at) : '-') + '</td>' +
        '<td>' + MD.fmtDate(c.created_at) + '</td></tr>';
    }).join('');
    MD.renderPager(el('rc-pager'), rcPage, d.total || 0, 20, loadResetCoupons);
  }).catch(function (err) {
    tbody.innerHTML = emptyRow(6, 'alert', '加载失败', err.message);
  });
}
function openIssueCouponForm() {
  MD.openModal({
    title: '发放重置券',
    body:
      '<div class="field"><label>发放对象<span class="req">*</span></label>' +
      '<div class="model-checks">' +
      '<label class="model-check"><input type="radio" name="rc-target" value="specific" checked onclick="this.parentElement.classList.toggle(\'is-on\',true)">指定用户</label>' +
      '<label class="model-check"><input type="radio" name="rc-target" value="all" onclick="this.parentElement.classList.toggle(\'is-on\',true)">全员发放</label>' +
      '</div>' +
      '<div class="hint">全员发放：每个有效账号各获得 1 张重置券</div></div>' +
      '<div class="field" id="rc-uid-field"><label>用户 ID<span class="req">*</span></label>' +
      '<input class="input" id="rc-uid" type="number" min="1" placeholder="如 1"></div>' +
      '<div class="field"><label>每人数量</label><input class="input" id="rc-count" type="number" min="1" max="10" value="1" style="max-width:120px;">' +
      '<div class="hint">每人最多 10 张，默认 1 张</div></div>' +
      '<div class="field"><label>备注</label><input class="input" id="rc-note" placeholder="例如：8 月活动补偿" maxlength="200"></div>',
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '确认发放', cls: 'btn-primary', onClick: submitIssueCoupons }
    ]
  });
  var radios = document.querySelectorAll('input[name="rc-target"]');
  radios.forEach(function (r) {
    r.addEventListener('change', function () {
      el('rc-uid-field').style.display = r.value === 'specific' ? '' : 'none';
      el('rc-uid').required = r.value === 'specific';
    });
  });
}
function submitIssueCoupons() {
  var target = document.querySelector('input[name="rc-target"]:checked');
  var targetVal = target ? target.value : 'specific';
  var uid = 0;
  if (targetVal === 'specific') {
    uid = parseInt((el('rc-uid').value || '').trim(), 10);
    if (!uid || uid <= 0) { MD.toast('请输入有效的用户 ID', 'warn'); return; }
  }
  var count = parseInt((el('rc-count').value || '1'), 10) || 1;
  if (count < 1) count = 1;
  if (count > 10) count = 10;
  var note = (el('rc-note').value || '').trim();
  var btn = document.querySelector('.modal__foot .btn-primary');
  if (btn) { btn.disabled = true; btn.textContent = '发放中…'; }
  MD.api.post('/admin/reset-coupons', { user_id: uid, count: count, note: note }).then(function (res) {
    var d = res.data || {};
    MD.toast('已发放 ' + d.issued + ' 张重置券（' + (d.target === 'all' ? '全员' : '指定用户') + '）', 'success');
    MD.closeModal();
    loadResetCoupons(1);
  }).catch(function (e) {
    MD.toast(e.message || '发放失败', 'error');
  }).finally(function () {
    if (btn) { btn.disabled = false; btn.textContent = '确认发放'; }
  });
}

/* =====================================================================
   视图 · 模型价格（数据库价格表）
   ===================================================================== */
var modelPricesCache = [];
function loadModelPrices() {
  var tbody = el('model-price-tbody');
  tbody.innerHTML = MD.loadingRow(6);
  MD.api.get('/admin/model-prices').then(function (res) {
    modelPricesCache = res.data || [];
    if (!modelPricesCache.length) {
      tbody.innerHTML = emptyRow(6, 'card', '暂无模型价格', '新增渠道模型后点击右上角「新建价格」配置单价；未配置价格的模型不可调用');
      return;
    }
    tbody.innerHTML = modelPricesCache.map(function (p) {
      return '<tr>' +
        '<td><b style="color:var(--text-1)">' + esc(p.model) + '</b></td>' +
        '<td><span class="tag tag--warning">¥ ' + esc(p.input_price) + '</span></td>' +
        '<td><span class="tag tag--warning">¥ ' + esc(p.output_price) + '</span></td>' +
        '<td>' + (p.enabled
          ? '<span class="tag tag--success">启用</span>'
          : '<span class="tag tag--gray">停用</span>') + '</td>' +
        '<td><span class="td-sub">' + esc(p.remark || '') + '</span></td>' +
        '<td><div class="t-actions">' +
          '<button class="btn-link" onclick="toggleModelPrice(' + p.id + ')">' + (p.enabled ? '停用' : '启用') + '</button>' +
          '<button class="btn-link" onclick="openModelPriceForm(' + p.id + ')">' + MD.icon('edit', 14) + '编辑</button>' +
          '<button class="btn-link btn-link--danger" onclick="deleteModelPrice(' + p.id + ')">' + MD.icon('trash', 14) + '删除</button>' +
        '</div></td></tr>';
    }).join('');
  }).catch(function (err) {
    MD.toast(err.message || '模型价格加载失败', 'error');
    tbody.innerHTML = emptyRow(6, 'alert', '加载失败', err.message);
  });
}
function openModelPriceForm(id) {
  var p = null;
  if (id) {
    p = modelPricesCache.filter(function (x) { return x.id === id; })[0];
    if (!p) { MD.toast('价格配置不存在，请刷新', 'warn'); return; }
  }
  var f = function (name, label, value, req, hint) {
    return '<div class="field"><label>' + label + (req ? '<span class="req">*</span>' : '') + '</label>' +
      '<input class="input" id="mp-' + name + '" value="' + esc(value == null ? '' : value) + '"' + (name === 'model' ? '' : ' type="number" step="0.0001" min="0"') + '>' +
      (hint ? '<div class="hint">' + hint + '</div>' : '') + '</div>';
  };
  var body = '<div class="form-grid">' +
    '<div class="field full"><label>模型名称<span class="req">*</span></label>' +
      '<input class="input" id="mp-model" list="mp-model-list" value="' + esc(p ? p.model : '') + '" placeholder="与渠道中模型名精确一致，如 gpt-4o / claude-3-5-sonnet">' +
      '<datalist id="mp-model-list"></datalist>' +
      '<div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin-top:8px;">' +
        '<button type="button" class="btn-ghost" id="mp-fetch-btn" onclick="refreshModelList()">从渠道获取模型</button>' +
        '<span class="hint" id="mp-model-count" style="margin:0;"></span>' +
      '</div></div>' +
    f('input', '输入价（¥/1M token）', p ? p.input_price : '', true, '每百万输入 token 的价格，如 2.5') +
    f('output', '输出价（¥/1M token）', p ? p.output_price : '', true, '每百万输出 token 的价格，如 10') +
    f('cache-read', '缓存读价格（¥/1M token）', p ? p.cache_read_price : '', false, '缓存命中计费价，留空=默认输入价×10%（低于输入价才有意义）') +
    f('cache-write', '缓存写价格（¥/1M token）', p ? p.cache_write_price : '', false, '缓存写入计费价，留空=默认输入价×125%') +
    '<div class="field"><label>状态</label>' +
      '<label class="switch-row"><input type="checkbox" id="mp-enabled"' + ((!p || p.enabled) ? ' checked' : '') + '><span class="switch"></span><span style="font-size:13px;">启用该价格</span></label></div>' +
    '<div class="field full"><label>备注</label><input class="input" id="mp-remark" value="' + esc(p ? p.remark : '') + '" placeholder="用途说明"></div>' +
    '</div>';
  MD.openModal({
    title: id ? '编辑模型价格' : '新建模型价格', wide: '620px', body: body,
    buttons: [
      { label: '取消', cls: 'btn-ghost', onClick: MD.closeModal },
      { label: '保存', cls: 'btn-primary', onClick: function () { saveModelPrice(id); } }
    ]
  });
  refreshModelList();
}
function refreshModelList() {
  var box = el('mp-model-list'), info = el('mp-model-count'), btn = el('mp-fetch-btn');
  if (info) info.textContent = '获取中…';
  if (btn) btn.disabled = true;
  MD.api.get('/admin/channels').then(function (res) {
    var list = res.data || [];
    var seen = {}, out = [];
    list.forEach(function (ch) {
      if (!ch.enabled) return;
      (ch.models || []).forEach(function (m) {
        m = String(m).trim();
        if (!m || m.indexOf('*') >= 0 || seen[m]) return;
        seen[m] = true;
        out.push(m);
      });
    });
    out.sort();
    box.innerHTML = out.map(function (m) { return '<option value="' + esc(m) + '">'; }).join('');
    if (info) info.textContent = out.length ? '已载入 ' + out.length + ' 个启用渠道的模型' : '启用渠道未配置具体模型';
  }).catch(function (err) {
    if (info) info.textContent = '获取失败';
    MD.toast(err.message || '获取模型列表失败', 'error');
  }).finally(function () {
    if (btn) btn.disabled = false;
  });
}
function mpVal(name) { var e = el('mp-' + name); return e ? e.value.trim() : ''; }
function saveModelPrice(id) {
  var model = mpVal('model'), input = mpVal('input'), output = mpVal('output');
  if (!model) { MD.toast('请填写模型名称', 'warn'); return; }
  if (input === '' || isNaN(Number(input)) || Number(input) < 0) { MD.toast('请填写有效的输入价（≥ 0）', 'warn'); return; }
  if (output === '' || isNaN(Number(output)) || Number(output) < 0) { MD.toast('请填写有效的输出价（≥ 0）', 'warn'); return; }
  var body = {
    model: model,
    input_price: input,
    output_price: output,
    cache_read_price: mpVal('cache-read'),
    cache_write_price: mpVal('cache-write'),
    enabled: !!(el('mp-enabled') && el('mp-enabled').checked),
    remark: mpVal('remark')
  };
  var req = id ? MD.api.put('/admin/model-prices/' + id, body) : MD.api.post('/admin/model-prices', body);
  req.then(function () {
    MD.toast(id ? '价格已更新，立即生效' : '价格已创建，模型已可调用', 'success');
    MD.closeModal();
    loadModelPrices();
  }).catch(function (err) { MD.toast(err.message || '保存失败', 'error'); });
}
function toggleModelPrice(id) {
  var p = modelPricesCache.filter(function (x) { return x.id === id; })[0];
  if (!p) { MD.toast('价格配置不存在，请刷新', 'warn'); return; }
  var next = !p.enabled;
  var body = { model: p.model, input_price: p.input_price, output_price: p.output_price, cache_read_price: p.cache_read_price || '', cache_write_price: p.cache_write_price || '', enabled: next, remark: p.remark };
  MD.api.put('/admin/model-prices/' + id, body).then(function () {
    MD.toast(next ? '模型已启用并可调用' : '模型已停用，调用将被拒绝', 'success');
    loadModelPrices();
  }).catch(function (err) { MD.toast(err.message || '操作失败', 'error'); });
}
function deleteModelPrice(id) {
  var p = modelPricesCache.filter(function (x) { return x.id === id; })[0];
  MD.confirm('确定删除模型「' + esc(p ? p.model : '#' + id) + '」的价格配置吗？删除后该模型将不可调用。', function () {
    MD.api.del('/admin/model-prices/' + id).then(function () {
      MD.toast('价格已删除', 'success');
      loadModelPrices();
    }).catch(function (err) { MD.toast(err.message || '删除失败', 'error'); });
  }, { title: '删除模型价格', okLabel: '删除', danger: true });
}

/* =====================================================================
   视图 6 · 收入分析
   ===================================================================== */
function loadRevenue() {
  var s = el('rev-start').value || daysAgo(13);
  var e = el('rev-end').value || dstr(new Date());
  if (s > e) { var t = s; s = e; e = t; }
  el('rev-start').value = s; el('rev-end').value = e;
  el('rev-range-label').textContent = s + ' ~ ' + e;
  el('rev-stats').innerHTML =
    metricCard('coin', '总收入', '-', '区间累计') +
    metricCard('card', '订阅收入', '-', '成功订阅订单') +
    metricCard('wallet', '充值收入', '-', '成功充值订单');
  el('rev-bars').innerHTML = ''; el('rev-axis').innerHTML = '';
  el('rev-tbody').innerHTML = MD.loadingRow(4);

  MD.api.get('/admin/analytics/revenue?start_date=' + s + '&end_date=' + e).then(function (res) {
    var series = buildSeries(s, e, res.data || []);
    var total = series.reduce(function (acc, it) { return acc + it.value; }, 0);

    renderBars(el('rev-bars'), el('rev-axis'), series.slice(-31));
    el('rev-tbody').innerHTML = series.length
      ? series.slice().reverse().map(function (it) {
          var pct = total > 0 ? (it.value / total * 100).toFixed(1) + '%' : '-';
          return '<tr><td class="mono">' + it.date + '</td>' +
            '<td class="num amt-in">¥ ' + MD.fmtMoney(it.value) + '</td>' +
            '<td class="num">' + MD.fmtNum(it.count || 0) + '</td>' +
            '<td class="num">' + pct + '</td></tr>';
        }).join('')
      : emptyRow(4, 'chart', '该区间暂无收入数据');

    // 订阅 / 充值拆分（后端按日接口不区分类型，这里按订单类型在客户端聚合）
    function sumByType(type) {
      return MD.api.get('/admin/orders?type=' + type + '&status=success&size=100').then(function (r) {
        var sum = 0;
        ((r.data && r.data.items) || []).forEach(function (o) {
          var d = String(o.created_at || '').slice(0, 10);
          if (d >= s && d <= e) sum += parseFloat(o.amount || 0);
        });
        return sum;
      }).catch(function () { return 0; });
    }
    Promise.all([sumByType('subscription'), sumByType('recharge')]).then(function (parts) {
      el('rev-stats').innerHTML =
        metricCard('coin', '总收入', '¥ ' + MD.fmtMoney(total), '区间累计') +
        metricCard('card', '订阅收入', '¥ ' + MD.fmtMoney(parts[0]), '成功订阅订单') +
        metricCard('wallet', '充值收入', '¥ ' + MD.fmtMoney(parts[1]), '成功充值订单');
    });
  }).catch(function (err) {
    MD.toast(err.message || '收入数据加载失败', 'error');
    el('rev-tbody').innerHTML = emptyRow(4, 'alert', '加载失败', err.message);
  });
}

/* =====================================================================
   视图 7 · 系统配置（分类表单）
   ===================================================================== */
var CONFIG_GROUPS = [
  { group: 'site', title: '网站配置', fields: [
    { key: 'site_name',        label: '网站名称', type: 'text' },
    { key: 'site_url',         label: '网站地址', type: 'text', hint: '平台对外访问地址' },
    { key: 'site_logo',        label: 'Logo 图片地址', type: 'text', hint: '建议使用 CDN 图片链接' },
    { key: 'site_description', label: '网站描述', type: 'textarea' },
    { key: 'site_icp',         label: 'ICP 备案号', type: 'text' },
    { key: 'site_footer',      label: '页脚信息', type: 'text' }
  ]},
  { group: 'legal', title: '法律条款', fields: [
    { key: 'legal_terms',   label: '服务条款内容', type: 'textarea', hint: '用于落地页「服务条款」弹窗，支持多行；不填则显示默认提示', rows: 10 },
    { key: 'legal_privacy', label: '隐私政策内容', type: 'textarea', hint: '用于落地页「隐私政策」弹窗，支持多行；不填则显示默认提示', rows: 10 }
  ]},
  { group: 'contact', title: '联系方式', fields: [
    { key: 'contact_email',    label: '客服邮箱', type: 'text' },
    { key: 'contact_phone',    label: '联系电话', type: 'text' },
    { key: 'contact_wechat',   label: '客服微信', type: 'text' },
    { key: 'contact_address',  label: '公司地址', type: 'text' },
    { key: 'contact_worktime', label: '服务时间', type: 'text', placeholder: '如 周一至周五 9:00-18:00' }
  ]},
  { group: 'notify', title: '邮箱/短信', fields: [
    { key: 'smtp_host',      label: 'SMTP 服务器', type: 'text', placeholder: '如 smtp.example.com' },
    { key: 'smtp_port',      label: 'SMTP 端口',   type: 'text', placeholder: '如 465 或 587' },
    { key: 'smtp_username',  label: '邮箱账号',    type: 'text' },
    { key: 'smtp_password',  label: '邮箱授权码',  type: 'password', hint: '部分邮箱需使用授权码而非登录密码' },
    { key: 'smtp_from',      label: '发件人地址',  type: 'text' },
    { key: 'sms_provider',   label: '短信服务商',  type: 'select', options: [['', '不使用'], ['aliyun', '阿里云'], ['tencent', '腾讯云'], ['twilio', 'Twilio']] },
    { key: 'sms_access_key', label: 'AccessKey ID', type: 'text' },
    { key: 'sms_secret',     label: 'AccessKey Secret', type: 'password' },
    { key: 'sms_sign',       label: '短信签名',    type: 'text' }
  ]},
  { group: 'payment', title: '支付接口', fields: [
    { key: 'payment_provider',           label: '支付通道', type: 'select', options: [['', '未选择'], ['alipay', '支付宝'], ['wechat', '微信支付'], ['stripe', 'Stripe']] },
    { key: 'payment_alipay_app_id',      label: '支付宝 AppID', type: 'text' },
    { key: 'payment_alipay_private_key', label: '支付宝应用私钥', type: 'password', hint: 'PKCS8 格式私钥' },
    { key: 'payment_alipay_public_key',  label: '支付宝公钥', type: 'text' },
    { key: 'payment_wechat_mch_id',      label: '微信商户号', type: 'text' },
    { key: 'payment_wechat_api_key',     label: '微信 API 密钥', type: 'password' },
    { key: 'payment_stripe_secret',      label: 'Stripe Secret Key', type: 'password' },
    { key: 'payment_callback_url',       label: '支付回调地址', type: 'text' }
  ]},
  { group: 'epay', title: '易支付', fields: [
    { key: 'pay_epay_enabled',     label: '启用易支付', type: 'select', options: [['false', '关闭'], ['true', '启用']] },
    { key: 'pay_epay_gateway',     label: '易支付网关地址', type: 'text', placeholder: '如 https://pay.example.com', hint: '网关根地址，用于 submit.php / api.php 调用' },
    { key: 'pay_epay_pid',         label: '商户 ID (PID)', type: 'text' },
    { key: 'pay_epay_key',         label: '商户密钥 (KEY)', type: 'password', hint: '用于 MD5 签名，请勿泄露' },
    { key: 'pay_epay_sign_upper',  label: '签名大写', type: 'select', options: [['true', '大写（默认，彩虹易支付）'], ['false', '小写']] },
    { key: 'pay_epay_default_type', label: '默认支付方式', type: 'select', options: [['alipay', '支付宝'], ['wxpay', '微信支付'], ['qqpay', 'QQ 钱包']] }
  ]},
  { group: 'oauth', title: '亦 OpenID', fields: [
    { key: 'oauth_enabled',       label: '启用亦 OpenID 登录', type: 'select', options: [['false', '关闭'], ['true', '启用']] },
    { key: 'oauth_server',        label: '授权服务器地址', type: 'text', placeholder: '如 https://account.yiziyun.com', hint: '授权、换取 token、获取用户信息的根地址' },
    { key: 'oauth_client_id',     label: 'Client ID', type: 'text' },
    { key: 'oauth_client_secret', label: 'Client Secret', type: 'password', hint: '请勿泄露' },
    { key: 'oauth_redirect_uri',  label: '回调地址', type: 'text', placeholder: 'https://mass.yiziyun.com/api/v1/auth/openid/callback', hint: '需与授权应用登记的回调地址一致' }
  ]}
];
var configCache = [];
function loadConfig() {
  MD.api.get('/admin/config').then(function (res) {
    configCache = res.data || [];
    renderCfgPanes();
  }).catch(function (err) {
    MD.toast(err.message || '配置加载失败', 'error');
  });
}
function cfgValue(key) {
  var c = configCache.filter(function (x) { return x.key === key; })[0];
  return c ? String(c.value == null ? '' : c.value) : '';
}
function renderCfgPanes() {
  CONFIG_GROUPS.forEach(function (g) {
    var rows = g.fields.map(function (f) {
      var val = cfgValue(f.key);
      var control;
      if (f.type === 'textarea') {
        control = '<textarea class="textarea" id="cfg-' + f.key + '" placeholder="' + esc(f.placeholder || '') + '">' + esc(val) + '</textarea>';
      } else if (f.type === 'select') {
        var opts = f.options.map(function (o) {
          return '<option value="' + esc(o[0]) + '"' + (val === o[0] ? ' selected' : '') + '>' + esc(o[1]) + '</option>';
        }).join('');
        control = '<select class="select" id="cfg-' + f.key + '">' + opts + '</select>';
      } else {
        control = '<input class="input" type="' + f.type + '" id="cfg-' + f.key + '" value="' + esc(val) + '" placeholder="' + esc(f.placeholder || '') + '">';
      }
      return '<div class="field">' +
        '<label>' + esc(f.label) + '</label>' + control +
        (f.hint ? '<div class="hint">' + esc(f.hint) + '</div>' : '') +
        '</div>';
    }).join('');
    var pane = el('cfg-pane-' + g.group);
    if (!pane) return;
    pane.innerHTML = '<div class="cfg-form">' + rows +
      '<div class="cfg-actions"><button class="btn-primary" onclick="saveCfgGroup(\'' + g.group + '\')"><span data-icon="check:15"></span><span>保存' + g.title + '</span></button></div>' +
      '</div>';
  });
}
function saveCfgGroup(group) {
  var g = CONFIG_GROUPS.filter(function (x) { return x.group === group; })[0];
  if (!g) return;
  var items = g.fields.map(function (f) {
    var node = el('cfg-' + f.key);
    return { key: f.key, value: node ? node.value : '' };
  });
  MD.api.put('/admin/config/batch', { group: group, items: items }).then(function () {
    MD.toast('已保存' + g.title, 'success');
  }).catch(function (err) { MD.toast(err.message || '保存失败', 'error'); });
}
function switchCfgTab(group) {
  var tabs = document.querySelectorAll('.cfg-tab');
  for (var i = 0; i < tabs.length; i++) {
    tabs[i].classList.toggle('is-active', tabs[i].getAttribute('data-cfg') === group);
  }
  var panes = document.querySelectorAll('.cfg-pane');
  for (var j = 0; j < panes.length; j++) {
    panes[j].classList.toggle('is-active', panes[j].id === 'cfg-pane-' + group);
  }
}

/* =====================================================================
   视图 8 · 系统日志
   ===================================================================== */
function logsQuery() {
  App.logs.page = 1;
  App.logs.level = el('log-level').value;
  App.logs.module = el('log-module').value.trim();
  App.logs.start = el('log-start').value;
  App.logs.end = el('log-end').value;
  loadLogs();
}
function loadLogs() {
  var st = App.logs;
  var tbody = el('logs-tbody');
  tbody.innerHTML = MD.loadingRow(6);
  var q = '/admin/logs?page=' + st.page + '&size=' + st.size +
    (st.level ? '&level=' + st.level : '') +
    (st.module ? '&module=' + encodeURIComponent(st.module) : '') +
    (st.start ? '&start_date=' + st.start : '') +
    (st.end ? '&end_date=' + st.end : '');
  MD.api.get(q).then(function (res) {
    var data = res.data || {}, items = data.items || [];
    if (!items.length) {
      tbody.innerHTML = emptyRow(6, 'file', '暂无日志记录');
    } else {
      tbody.innerHTML = items.map(function (l) {
        return '<tr>' +
          '<td class="mono" style="white-space:nowrap">' + MD.fmtDate(l.created_at) + '</td>' +
          '<td>' + tagOf(LOG_LEVEL, l.level) + '</td>' +
          '<td class="mono">' + esc(l.module || '-') + '</td>' +
          '<td>' + esc(l.action || '-') + '</td>' +
          '<td class="cell-clip" title="' + esc(l.message) + '">' + esc(l.message || '-') + '</td>' +
          '<td class="mono">' + esc(l.ip || '-') + '</td></tr>';
      }).join('');
    }
    el('logs-count').textContent = countText(data.total || 0, data.page || st.page, data.size || st.size);
    MD.renderPager(el('logs-pager'), data.page || st.page, data.total || 0, data.size || st.size, function (p) { st.page = p; loadLogs(); });
  }).catch(function (err) {
    MD.toast(err.message || '日志加载失败', 'error');
    tbody.innerHTML = emptyRow(6, 'alert', '加载失败', err.message);
  });
}

/* =====================================================================
   视图 9 · 系统健康
   ===================================================================== */
function serviceCard(icon, name, desc, ok) {
  var pill = ok == null
    ? '<span class="health-pill health-pill--info">' + MD.icon('clock', 14) + '检测中</span>'
    : (ok ? '<span class="health-pill">' + MD.icon('check', 14) + '运行正常</span>'
          : '<span class="health-pill health-pill--danger">' + MD.icon('alert', 14) + '服务异常</span>');
  return '<article class="card metric-card"><div class="m-label">' + MD.icon(icon, 15) + esc(name) + '</div>' +
    '<div style="margin:12px 0 10px">' + pill + '</div>' +
    '<div class="m-sub">' + esc(desc) + '</div></article>';
}
function loadHealth() {
  var box = el('health-services');
  box.innerHTML =
    serviceCard('database', '数据库', 'MySQL / 主存储连接', null) +
    serviceCard('bolt', 'Redis', '缓存与限流存储', null) +
    serviceCard('server', 'API 服务', '网关与业务服务', null);
  MD.api.get('/admin/health').then(function (res) {
    var h = res.data || {};
    box.innerHTML =
      serviceCard('database', '数据库', 'MySQL / 主存储连接', !!h.database) +
      serviceCard('bolt', 'Redis', '缓存与限流存储', !!h.redis) +
      serviceCard('server', 'API 服务', '网关与业务服务', !!h.api);
  }).catch(function (err) {
    MD.toast(err.message || '健康检测失败', 'error');
    box.innerHTML =
      serviceCard('database', '数据库', 'MySQL / 主存储连接', false) +
      serviceCard('bolt', 'Redis', '缓存与限流存储', false) +
      serviceCard('server', 'API 服务', '网关与业务服务', false);
  });

  var mbox = el('health-metrics');
  mbox.innerHTML = '<article class="card metric-card"><div class="m-label"><span class="spinner"></span>指标加载中…</div></article>';
  MD.api.get('/admin/metrics').then(function (res) {
    var list = res.data || [];
    var req = 0, okReq = 0, tokens = 0, latency = 0, users = 0, revenue = 0, n = list.length;
    list.forEach(function (m) {
      req += m.total_requests || 0;
      okReq += m.success_requests || 0;
      tokens += m.total_tokens || 0;
      latency += m.avg_latency_ms || 0;
      if ((m.active_users || 0) > users) users = m.active_users;
      revenue += m.revenue || 0;
    });
    var rate = req > 0 ? (okReq / req * 100).toFixed(1) + '%' : '-';
    var lat = n > 0 ? Math.round(latency / n) : 0;
    mbox.innerHTML =
      metricCard('chart', '总请求', MD.fmtNum(req), '近 7 天累计') +
      metricCard('checkCircle', '成功率', esc(rate), '成功 ' + MD.fmtNum(okReq) + ' 次') +
      metricCard('cube', 'Token 总量', MD.fmtNum(tokens), '输入 + 输出') +
      metricCard('clock', '平均延迟', MD.fmtNum(lat) + ' <small>ms</small>', '采样点均值') +
      metricCard('users', '活跃用户', MD.fmtNum(users), '峰值') +
      metricCard('coin', '收入', '¥ ' + MD.fmtMoney(revenue), '近 7 天累计');
  }).catch(function (err) {
    MD.toast(err.message || '指标加载失败', 'error');
    mbox.innerHTML = metricCard('alert', '指标加载失败', '-', err.message || '');
  });
}
