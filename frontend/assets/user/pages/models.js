// 模型广场
import { api, toast, esc, fmtMoney } from '../api.js';

export function initModels() {
  loadModels();
}

export function loadModels() {
  api.get('/models').then((res) => {
    const list = (res && res.data) || [];
    const grid = document.getElementById('model-grid');
    if (!grid) return;
    if (!list.length) { grid.innerHTML = '<div class="card"><p style="color:var(--text-3);font-size:13px;">暂无可用模型</p></div>'; return; }
    grid.innerHTML = list.map((m) => {
      const isAnthropic = m.provider === 'anthropic';
      const features = (m.features || []).map((f) => {
        return '<span class="tag ' + (f === '多模态' ? 'tag--info' : 'tag--gray') + '">' + esc(f) + '</span>';
      }).join('');
      return '' +
        '<div class="md-card">' +
          '<div class="md-card__top">' +
            '<div class="md-card__badge ' + (isAnthropic ? 'is-anth' : '') + '">' + (isAnthropic ? 'Anthropic' : 'OpenAI') + '</div>' +
            '<span class="md-card__status"><i></i>可用</span>' +
          '</div>' +
          '<div class="md-card__name">' + esc(m.name) + '</div>' +
          '<div class="md-card__id">' + esc(m.id) + '</div>' +
          (m.description ? '<p class="md-card__desc">' + esc(m.description) + '</p>' : '') +
          '<div class="md-card__feats">' + features + '</div>' +
          '<div class="md-card__meta">' +
            '<div class="md-card__meta-item"><span>上下文</span><b>' + esc(m.context || '-') + '</b></div>' +
            '<div class="md-card__meta-item"><span>输入</span><b>' + esc(m.input_price || '-') + '</b><small>/1M</small></div>' +
            '<div class="md-card__meta-item"><span>输出</span><b>' + esc(m.output_price || '-') + '</b><small>/1M</small></div>' +
            '<div class="md-card__meta-item"><span>缓存读</span><b>' + esc(m.cache_read_price || '-') + '</b><small>/1M</small></div>' +
            '<div class="md-card__meta-item"><span>缓存写</span><b>' + esc(m.cache_write_price || '-') + '</b><small>/1M</small></div>' +
          '</div>' +
        '</div>';
    }).join('');
  }).catch((e) => toast(e.message, 'error'));
}

import { onUserReady } from '../main.js';
onUserReady(initModels);
