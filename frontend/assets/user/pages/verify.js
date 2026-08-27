// 实名认证页
import { api, toast, esc } from '../api.js';
import { safe, injectIcons } from '../ui.js';

const MD = window.MD;
let _verifyUploads = { front: null, back: null };

export function initVerify() {
  loadVerify();
}

export function loadVerify() {
  const box = document.getElementById('verify-body');
  box.innerHTML = '<div class="verify-banner"><div class="vb-ic vb-ic--warn"><span class="spinner" style="border-color:#ffd6a8;border-top-color:var(--c-warn);"></span></div>' +
    '<div><div class="vb-title">正在加载认证状态…</div></div></div>';
  safe(api.get('/user/identity-verification')).then((res) => {
    if (!res) { box.innerHTML = ''; return; }
    renderVerify(res.data || { status: 'unverified' });
  });
}

function renderVerify(v) {
  const box = document.getElementById('verify-body');
  const st = v.status || 'unverified';

  if (st === 'pending') {
    box.innerHTML = '<div class="verify-banner">' +
      '<div class="vb-ic vb-ic--warn">' + MD.icon('clock', 20) + '</div>' +
      '<div><div class="vb-title">实名认证审核中 <span class="tag tag--warn">审核中</span></div>' +
      '<div class="vb-desc">你提交的实名信息（' + esc(v.real_name || '—') + '）正在人工审核，通常需要 1~3 个工作日，请耐心等待。' +
      (v.updated_at ? '<br>提交时间：' + MD.fmtDate(v.updated_at) : '') + '</div></div></div>';
    return;
  }

  if (st === 'verified') {
    box.innerHTML = '<div class="verify-banner">' +
      '<div class="vb-ic vb-ic--success">' + MD.icon('checkCircle', 20) + '</div>' +
      '<div style="flex:1;"><div class="vb-title">已通过实名认证 <span class="tag tag--success">已认证</span></div>' +
      '<div class="vb-desc">感谢配合，你的账号已解锁完整能力。</div>' +
      '<div class="divider"></div>' +
      '<div class="desc-grid">' +
      '<div class="desc-item"><span class="k">真实姓名</span><span class="v">' + esc(v.real_name || '—') + '</span></div>' +
      '<div class="desc-item"><span class="k">认证时间</span><span class="v">' + MD.fmtDate(v.updated_at) + '</span></div>' +
      '</div></div></div>';
    return;
  }

  if (st === 'rejected') {
    box.innerHTML = '<div class="verify-banner" style="margin-bottom:16px;">' +
      '<div class="vb-ic vb-ic--danger">' + MD.icon('alert', 20) + '</div>' +
      '<div><div class="vb-title">认证未通过 <span class="tag tag--danger">已拒绝</span></div>' +
      '<div class="vb-desc">拒绝原因：' + esc(v.reject_reason || '信息不符合要求') + '<br>请修正后重新提交。</div></div></div>' +
      verifyFormHTML(v.real_name || '');
  } else {
    box.innerHTML = '<div class="card card--narrow" style="margin-bottom:16px;">' +
      '<div class="card__title">' + MD.icon('shield', 16) + ' 为什么需要实名认证？</div>' +
      '<p style="font-size:13px;color:var(--text-3);line-height:1.7;">根据监管要求，使用 LLM API 调用、充值提现等能力前需完成实名认证。你的信息仅用于身份核验，将被严格保密。</p></div>' +
      verifyFormHTML('');
  }
  injectIcons();
}

export function pickIdCard(side) {
  const inp = side === 'front' ? document.getElementById('vf-front-file') : document.getElementById('vf-back-file');
  if (inp) inp.click();
}

export function onIdCardPick(side) {
  const fileInput = side === 'front' ? document.getElementById('vf-front-file') : document.getElementById('vf-back-file');
  if (!fileInput || !fileInput.files.length) return;
  const file = fileInput.files[0];
  if (!/^image\/(jpeg|png|webp)$/i.test(file.type)) { toast('仅支持 JPG / PNG / WebP 图片', 'warn'); return; }
  if (file.size > 5 * 1024 * 1024) { toast('图片不能超过 5MB', 'warn'); return; }

  const reader = new FileReader();
  reader.onload = (e) => {
    const preview = side === 'front' ? document.getElementById('vf-front-preview') : document.getElementById('vf-back-preview');
    const placeholder = side === 'front' ? document.getElementById('vf-front-ph') : document.getElementById('vf-back-ph');
    const statusEl = side === 'front' ? document.getElementById('vf-front-status') : document.getElementById('vf-back-status');
    if (preview) { preview.src = e.target.result; preview.style.display = 'block'; }
    if (placeholder) placeholder.style.display = 'none';
    if (statusEl) { statusEl.className = 'upload-box__status is-loading'; statusEl.textContent = '上传中…'; }
  };
  reader.readAsDataURL(file);

  const fd = new FormData();
  fd.append('file', file);
  api.upload('/user/upload', fd).then((res) => {
    const url = (res && res.data && res.data.url) || '';
    const statusEl = side === 'front' ? document.getElementById('vf-front-status') : document.getElementById('vf-back-status');
    if (!url) { toast('上传失败，请重试', 'error'); return; }
    _verifyUploads[side] = url;
    if (statusEl) { statusEl.className = 'upload-box__status is-ok'; statusEl.textContent = '已上传，点击可更换'; }
    toast(side === 'front' ? '证件正面已上传' : '证件反面已上传', 'success');
  }).catch((e) => {
    const statusEl = side === 'front' ? document.getElementById('vf-front-status') : document.getElementById('vf-back-status');
    if (statusEl) { statusEl.className = 'upload-box__status is-error'; statusEl.textContent = '上传失败'; }
    toast(e.message || '上传失败', 'error');
  });
}

function verifyFormHTML(prefillName) {
  _verifyUploads = { front: null, back: null };
  return '<form class="card card--narrow" id="verify-form" novalidate>' +
    '<div class="card__title">提交实名信息</div>' +
    '<div class="form-grid">' +
    '<div class="field"><label>真实姓名<span class="req">*</span></label>' +
    '<input class="input" id="vf-name" value="' + esc(prefillName) + '" placeholder="与证件一致的姓名"></div>' +
    '<div class="field"><label>身份证号<span class="req">*</span></label>' +
    '<input class="input input--mono" id="vf-idnum" maxlength="18" placeholder="18 位身份证号码"></div>' +
    '<div class="field"><label>证件正面<span class="req">*</span></label>' +
    '<div class="upload-box" id="vf-front-box" onclick="pickIdCard(\'front\')">' +
      '<input type="file" accept="image/jpeg,image/png,image/webp" hidden id="vf-front-file" onchange="onIdCardPick(\'front\')">' +
      '<img class="upload-box__preview" id="vf-front-preview" alt="正面预览">' +
      '<div class="upload-box__ph" id="vf-front-ph">' + MD.icon('upload', 22) + '<span>点击上传证件正面</span></div>' +
      '<div class="upload-box__status" id="vf-front-status"></div>' +
    '</div></div>' +
    '<div class="field"><label>证件反面<span class="req">*</span></label>' +
    '<div class="upload-box" id="vf-back-box" onclick="pickIdCard(\'back\')">' +
      '<input type="file" accept="image/jpeg,image/png,image/webp" hidden id="vf-back-file" onchange="onIdCardPick(\'back\')">' +
      '<img class="upload-box__preview" id="vf-back-preview" alt="反面预览">' +
      '<div class="upload-box__ph" id="vf-back-ph">' + MD.icon('upload', 22) + '<span>点击上传证件反面</span></div>' +
      '<div class="upload-box__status" id="vf-back-status"></div>' +
    '</div></div>' +
    '</div>' +
    '<div class="hint" style="margin-bottom:14px;">请上传清晰的证件照片，支持 JPG / PNG / WebP，单张不超过 5MB。</div>' +
    '<button class="btn-primary" type="button" onclick="submitVerify()">' + MD.icon('shield', 14) + '<span>提交审核</span></button>' +
    '</form>';
}

export function submitVerify() {
  const name = document.getElementById('vf-name').value.trim();
  const idnum = document.getElementById('vf-idnum').value.trim();
  const front = _verifyUploads.front;
  const back = _verifyUploads.back;
  if (!name) { toast('请输入真实姓名', 'warn'); return; }
  if (!/^\d{17}[\dXx]$/.test(idnum)) { toast('请输入有效的 18 位身份证号', 'warn'); return; }
  if (!front || !back) { toast('请上传证件正反面照片', 'warn'); return; }
  api.post('/user/identity-verification', {
    real_name: name, id_number: idnum, id_card_front: front, id_card_back: back
  }).then(() => {
    toast('实名认证已提交，请耐心等待审核', 'success');
    loadVerify();
  }).catch((e) => toast(e.message, 'error'));
}

window.pickIdCard = pickIdCard;
window.onIdCardPick = onIdCardPick;
window.submitVerify = submitVerify;

import { onUserReady } from '../main.js';
onUserReady(initVerify);
