// 授信 + Token 加油包（dashboard / plans 共用）
import { currentUser } from '../state.js';
import { api, toast, fmtMoney, fmtNum, icon, esc, openModal, closeModal, confirm } from '../api.js';
import { safe, injectIcons } from '../ui.js';
import { refreshBalance } from '../layout.js';

export function fmtTokens(n) {
  n = Number(n) || 0;
  if (n >= 1e8) return (n / 1e8).toFixed(1).replace(/\.0$/, '') + ' 亿';
  if (n >= 1e4) return (n / 1e4).toFixed(1).replace(/\.0$/, '') + ' 万';
  return String(n);
}

let creditCheckedAt = 0;
const creditDismissKey = 'credit_apply_dismissed';

export function checkCreditPopup() {
  if (!currentUser) return;
  safe(api.get('/user/credit/status')).then((res) => {
    if (!res || !res.data || !res.data.can_apply) return;
    const dismissed = parseInt(localStorage.getItem(creditDismissKey) || '0', 10);
    if (dismissed && (Date.now() - dismissed) < 7 * 24 * 3600 * 1000) return;
    if (Date.now() - creditCheckedAt < 60000) return;
    creditCheckedAt = Date.now();
    openModal({
      title: '开通 Token 授信',
      wide: '480px',
      body:
        '<div style="display:flex;gap:12px;align-items:flex-start;">' +
        '<span style="font-size:34px;line-height:1;">🎉</span>' +
        '<div><p style="font-size:14px;line-height:1.8;color:var(--text-2);">恭喜！你的累计消费已达到 <b style="color:var(--brand-600);">¥' + fmtMoney(res.data.consumed_total) + '</b>，符合开通 <b>Token 授信</b> 的资格。</p>' +
        '<p style="font-size:13px;line-height:1.8;color:var(--text-3);margin-top:8px;">提交申请后由平台管理员审核，审核通过后将为你开通专属的 Token 授信额度，额度可直接用于 API 调用，先使用后结算。</p></div></div>',
      buttons: [
        { label: '暂不申请', cls: 'btn-ghost', onClick: () => { localStorage.setItem(creditDismissKey, String(Date.now())); closeModal(); } },
        { label: '申请开通', cls: 'btn-primary', onClick: () => {
          api.post('/user/credit/apply', {}).then(() => { toast('授信申请已提交，等待管理员审核', 'success'); closeModal(); renderCreditBanner(); })
            .catch((e) => toast(e.message || '申请失败', 'error'));
        } }
      ]
    });
  });
}

export function renderCreditCard() {
  const wrap = document.getElementById('credit-card-wrap');
  if (!wrap) return;
  safe(api.get('/user/credit/status')).then((res) => {
    const d = res && res.data ? res.data : null;
    if (!d) return;
    const limit = d.credit_limit || 0, used = d.credit_used || 0, avail = d.credit_available || 0;
    const app = d.application || null;
    if (limit <= 0) { wrap.style.display = 'none'; return; }
    let statusHtml = '';
    if (app && app.status === 'pending') statusHtml = '<span class="tag tag--warn">审核中</span>';
    const repayBtn = used > 0 ? '<button class="btn-primary btn-sm" onclick="openRepayModal()"><span data-ic="refresh" data-sz="14"></span><span>还款</span></button>' : '';
    const applyBtn = d.can_apply ? '<button class="btn-ghost btn-sm" onclick="applyCreditNow()">申请授信</button>' : '';
    wrap.innerHTML =
      '<div class="credit-mini">' +
      '<div class="credit-mini__name"><i><span data-ic="shield" data-sz="16"></span></i><span>Token 授信</span>' + statusHtml + '</div>' +
      '<div class="credit-mini__nums">' +
      '<div class="credit-mini__num"><div class="k">授信总额度</div><div class="v">' + fmtNum(limit) + '</div></div>' +
      '<div class="credit-mini__num is-warn"><div class="k">待还额度</div><div class="v">' + fmtNum(used) + '</div></div>' +
      '<div class="credit-mini__num"><div class="k">可用额度</div><div class="v">' + fmtNum(avail) + '</div></div>' +
      '</div>' +
      '<div class="credit-mini__ops">' + repayBtn + applyBtn + '</div>' +
      '</div>';
    wrap.style.display = 'block';
    injectIcons();
  });
}

export function applyCreditNow() {
  confirm('确认提交 Token 授信申请？审核通过后将为你开通授信额度。', () => {
    api.post('/user/credit/apply', {}).then(() => {
      toast('授信申请已提交，等待管理员审核', 'success');
      renderCreditCard();
      renderCreditBanner();
    }).catch((e) => toast(e.message || '申请失败', 'error'));
  });
}

export function openRepayModal() {
  api.get('/user/credit/status').then((res) => {
    const d = res.data || {};
    const used = d.credit_used || 0, credits = d.token_credits != null ? d.token_credits : 0;
    openModal({
      title: '归还授信额度',
      body:
        '<div class="field"><label>归还数量（Tokens）<span class="req">*</span></label>' +
        '<input class="input input--mono" id="repay-tokens" type="number" min="1" step="1" placeholder="最多可还 ' + used + '">' +
        '<div class="hint">待还 ' + fmtNum(used) + ' Tokens · 当前 Token 额度（加油包）' + fmtNum(credits) + ' Tokens</div></div>',
      buttons: [
        { label: '取消', cls: 'btn-ghost', onClick: closeModal },
        { label: '确认还款', cls: 'btn-primary', onClick: () => {
          const tokens = parseInt((document.getElementById('repay-tokens').value || '').trim(), 10);
          if (!tokens || tokens <= 0) { toast('请输入有效的归还数量', 'warn'); return; }
          api.post('/user/credit/repay', { tokens: tokens }).then(() => {
            toast('还款成功', 'success'); closeModal(); renderCreditCard(); renderCreditBanner(); loadTokenPackages();
          }).catch((e) => toast(e.message || '还款失败', 'error'));
        } }
      ]
    });
  });
}

export function renderCreditBanner() {
  const banner = document.getElementById('credit-banner');
  if (!banner) return;
  safe(api.get('/user/credit/status')).then((res) => {
    const app = res && res.data ? res.data.application : null;
    if (!app || app.status === 'rejected') { banner.style.display = 'none'; return; }
    let html;
    if (app.status === 'pending') {
      html = '<div class="credit-banner credit-banner--warn"><span data-ic="clock" data-sz="16"></span>' +
        '<div><b>Token 授信申请审核中</b><span class="credit-banner__sub">已提交，等待管理员审核，审核结果将以通知形式告知</span></div></div>';
    } else {
      html = '<div class="credit-banner credit-banner--ok"><span data-ic="check" data-sz="16"></span>' +
        '<div><b>Token 授信已生效</b><span class="credit-banner__sub">授信额度 ' + fmtNum(app.granted_tokens) + ' Tokens 已发放至你的账户</span></div></div>';
    }
    banner.innerHTML = html;
    banner.style.display = 'block';
    injectIcons();
  });
}

export function loadTokenPackages() {
  const pill = document.getElementById('token-credit-pill');
  if (pill && currentUser) pill.textContent = fmtTokens(currentUser.token_credits || 0);
  api.get('/user/token-packages').then((res) => {
    const list = (res && res.data) || [];
    const grid = document.getElementById('token-pkg-grid');
    if (!grid) return;
    if (!list.length) { grid.innerHTML = '<div class="card"><p style="color:var(--text-3);font-size:13px;">暂无在售加油包</p></div>'; return; }
    const hot = Math.min(1, list.length - 1);
    grid.innerHTML = list.map((p, i) => {
      const total = (Number(p.tokens) || 0) + (Number(p.bonus_tokens) || 0);
      const bonus = Number(p.bonus_tokens) > 0
        ? '<span class="tp-card__bonus">' + icon('star', 12) + ' 赠送 ' + fmtTokens(p.bonus_tokens) + ' Tokens</span>' : '';
      const flag = i === hot ? '<span class="tp-card__flag">性价比之选</span>' : '';
      return '<div class="tp-card' + (i === hot ? ' is-hot' : '') + '">' + flag +
        '<div class="tp-card__name">' + esc(p.name) + '</div>' +
        '<div class="tp-card__desc">' + esc(p.description || '') + '</div>' +
        '<div class="tp-card__tokens"><b>' + fmtTokens(total) + '</b><span>Tokens</span></div>' +
        bonus +
        '<div class="tp-card__price"><b>¥' + fmtMoney(p.price) + '</b>&nbsp;<small>一次性</small></div>' +
        '<button class="btn-primary tp-card__btn" onclick="purchaseTokenPkg(' + p.id + ')">' + icon('bolt', 14) + '<span>立即购买</span></button>' +
        '</div>';
    }).join('');
    injectIcons();
  }).catch((e) => toast(e.message, 'error'));
}

export function purchaseTokenPkg(id) {
  confirm('确认使用余额购买该 Token 加油包吗？', () => {
    api.post('/user/token-packages/' + id + '/purchase')
      .then((res) => {
        const d = (res && res.data) || {};
        toast('购买成功，Token 额度 +' + fmtTokens(d.token_credits - (currentUser.token_credits || 0)), 'success');
        currentUser.token_credits = d.token_credits;
        updateTokenCreditUI();
        return refreshBalance();
      })
      .catch((e) => toast(e.message, 'error'));
  }, { title: '购买确认', okLabel: '确认购买' });
}

export function updateTokenCreditUI() {
  const pill = document.getElementById('token-credit-pill');
  if (pill && currentUser) pill.textContent = fmtTokens(currentUser.token_credits || 0);
}

window.openRepayModal = openRepayModal;
window.applyCreditNow = applyCreditNow;
window.purchaseTokenPkg = purchaseTokenPkg;
