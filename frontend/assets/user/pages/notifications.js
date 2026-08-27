// 通知页
import { whenReady } from '../state.js';
import { api, esc } from '../api.js';
import { $, safe, injectIcons } from '../ui.js';

const MD = window.MD;
let notifPage = 1;

export function refreshNotifBadge() {
  safe(api.get('/user/notifications/unread-count')).then((res) => {
    if (!res) return;
    const n = (res.data || {}).unread || 0;
    const dot = $('hd-bell-dot');
    if (dot) {
      dot.style.display = n > 0 ? '' : 'none';
      dot.textContent = n > 99 ? '99+' : n;
      dot.classList.toggle('is-num', n > 9);
    }
  });
}

export function loadNotifPanel() {
  const list = $('notif-panel-list');
  if (!list) return;
  list.innerHTML = '<div style="padding:18px;text-align:center;color:var(--text-3);font-size:13px;"><span class="spinner"></span></div>';
  safe(api.get('/user/notifications?page=1&size=6')).then((res) => {
    if (!res) { list.innerHTML = ''; return; }
    const items = (res.data || {}).items || [];
    if (!items.length) {
      list.innerHTML = '<div style="padding:26px;text-align:center;color:var(--text-3);font-size:13px;">暂无通知</div>';
      return;
    }
    list.innerHTML = items.map((n) =>
      '<div class="notif-item' + (n.is_read ? ' is-read' : '') + '" onclick="goView(\'notifications\')">' +
      '<div class="notif-item__dot"></div>' +
      '<div class="notif-item__body">' +
      '<div class="notif-item__title">' + esc(n.title) + '</div>' +
      '<div class="notif-item__time">' + MD.fmtDate(n.created_at) + '</div>' +
      '</div></div>'
    ).join('');
    refreshNotifBadge();
  });
}

export function loadNotifications() {
  const list = $('notif-list');
  list.innerHTML = '<div class="card card--flat" style="text-align:center;padding:30px;color:var(--text-3);font-size:13px;"><span class="spinner"></span>正在加载通知…</div>';
  safe(api.get('/user/notifications?page=' + notifPage + '&size=15')).then((res) => {
    if (!res) { list.innerHTML = ''; return; }
    const d = res.data || {}, items = d.items || [];
    MD.renderPager($('notif-pager'), notifPage, d.total || 0, 15, (p) => { notifPage = p; loadNotifications(); });
    if (!items.length) {
      list.innerHTML = '<div class="card card--flat" style="text-align:center;padding:30px;">' +
        MD.emptyState('bell', '暂无通知', '平台公告和系统消息会出现在这里') + '</div>';
      return;
    }
    list.innerHTML = items.map((n) =>
      '<div class="card notif-card' + (n.is_read ? ' is-read' : '') + '"' + (n.is_read ? '' : ' onclick="readNotif(' + n.id + ')"') + '>' +
      '<div class="notif-card__tag"><span class="tag ' + (n.is_read ? 'tag--gray' : 'tag--success') + '">' + (n.is_read ? '已读' : '未读') + '</span></div>' +
      '<div class="notif-card__body">' +
      '<div class="notif-card__title">' + esc(n.title) + '</div>' +
      '<div class="notif-card__content">' + esc(n.content) + '</div>' +
      '<div class="notif-card__time">' + MD.fmtDate(n.created_at) + '</div>' +
      '</div></div>'
    ).join('');
    refreshNotifBadge();
  });
}

export function readNotif(id) {
  safe(api.put('/user/notifications/' + id + '/read')).then((res) => {
    if (!res) return;
    loadNotifications();
  });
}

export function markAllNotif() {
  safe(api.put('/user/notifications/read-all')).then((res) => {
    if (!res) return;
    MD.toast('已全部标记为已读', 'success');
    if ($('notif-panel') && $('notif-panel').classList.contains('is-open')) loadNotifPanel();
    else loadNotifications();
  });
}

whenReady().then(() => { loadNotifications(); injectIcons(); });

window.readNotif = readNotif;
window.markAllNotif = markAllNotif;
window.refreshNotifBadge = refreshNotifBadge;
window.loadNotifPanel = loadNotifPanel;
