import { useEffect, useMemo, useState } from 'react';
import { Card, Tag, Input, App, Segmented, Tooltip } from 'antd';
import { CopyOutlined, SearchOutlined } from '@ant-design/icons';
import { Link } from 'react-router-dom';
import type { ModelCatalogEntry } from '@mass/shared';
import { request } from '../api/request';

const PROVIDER_META: Record<string, { label: string; color: string; hex: string }> = {
  openai: { label: 'OpenAI', color: 'blue', hex: '#10a37f' },
  anthropic: { label: 'Anthropic', color: 'orange', hex: '#d97757' },
  deepseek: { label: 'DeepSeek', color: 'geekblue', hex: '#4d6bfe' },
  qwen: { label: 'Qwen', color: 'purple', hex: '#7c3aed' },
  minimax: { label: 'MiniMax', color: 'volcano', hex: '#f97316' },
};

function providerMeta(p: string) {
  return PROVIDER_META[p] ?? { label: p, color: 'default', hex: '#94a3b8' };
}

export default function Models() {
  const { message } = App.useApp();
  const [models, setModels] = useState<ModelCatalogEntry[]>([]);
  const [eligible, setEligible] = useState<Set<string>>(new Set());
  const [keyword, setKeyword] = useState('');
  const [provider, setProvider] = useState<string>('all');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    request
      .get('/models')
      .then((r) => setModels(r.data.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
    // 当前用户实际符合「无限火力」条件的模型（需模型活动开启 + 持有覆盖该模型的付费订阅）。
    request
      .get('/user/unlimited-firepower')
      .then((r) => setEligible(new Set(r.data.data?.models || [])))
      .catch(() => {});
  }, []);

  const copy = (id: string) => {
    navigator.clipboard.writeText(id);
    message.success(`已复制 ${id}`);
  };

  const providers = useMemo(
    () => ['all', ...Array.from(new Set(models.map((m) => m.provider)))],
    [models],
  );

  const filtered = useMemo(
    () =>
      models.filter((m) => {
        const okP = provider === 'all' || m.provider === provider;
        const k = keyword.trim().toLowerCase();
        const okK =
          !k ||
          m.name.toLowerCase().includes(k) ||
          m.id.toLowerCase().includes(k) ||
          m.provider.toLowerCase().includes(k);
        return okP && okK;
      }),
    [models, keyword, provider],
  );

  const renderCards = () => {
    if (loading) {
      return Array.from({ length: 6 }).map((_, i) => (
        <Card key={i} loading className="model-card" />
      ));
    }
    if (filtered.length === 0) {
      return (
        <div className="model-empty">暂无可用模型（需管理端配置模型价格与渠道）</div>
      );
    }
    return filtered.map((m) => {
      const pm = providerMeta(m.provider);
      const ufActive = m.unlimited_enabled && eligible.has(m.id);
      return (
        <Card
          key={m.id}
          className={`model-card${ufActive ? ' model-card--unlimited' : ''}`}
          hoverable
          style={{ borderTop: `3px solid ${pm.hex}` }}
        >
          <div className="model-card-head">
            <div className="model-card-title">
              <div className="model-name">{m.name}</div>
              <Tooltip title="点击复制模型 ID">
                <span className="model-id" onClick={() => copy(m.id)}>
                  <code>{m.id}</code>
                  <CopyOutlined />
                </span>
              </Tooltip>
            </div>
            <Tag color={pm.color} bordered={false} className="provider-tag">
              {pm.label}
            </Tag>
          </div>

          <p className="model-desc">{m.description}</p>

          <div className="model-meta">
            <span className="model-chip">上下文 {m.context}</span>
            {m.unlimited_enabled &&
              (ufActive ? (
                <span className="uf-badge uf-active">🔥 无限火力 · 已激活</span>
              ) : (
                <span className="uf-badge uf-locked">
                  🔥 待解锁<Link to="/plans" className="uf-badge-link">去订阅</Link>
                </span>
              ))}
            {m.features?.map((f) => (
              <Tag key={f} bordered={false} color="processing" className="model-feature">
                {f}
              </Tag>
            ))}
          </div>

          <div className="model-price">
            <div>
              <span>输入</span>
              <b className={ufActive ? 'price-free' : ''}>{m.input_price}</b>
            </div>
            <div>
              <span>输出</span>
              <b className={ufActive ? 'price-free' : ''}>{m.output_price}</b>
            </div>
            {m.cache_read_price && (
              <div>
                <span>缓存读</span>
                <b className={ufActive ? 'price-free' : ''}>{m.cache_read_price}</b>
              </div>
            )}
          </div>

          {ufActive && <div className="model-unlimited-note">调用本模型免 Token 扣费</div>}
        </Card>
      );
    });
  };

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>模型市场</h2>
          <p>平台当前可调用的模型，价格为每百万 token 计价（¥/1M）</p>
        </div>
      </div>

      <div className="model-toolbar">
        <Segmented
          value={provider}
          onChange={(v) => setProvider(v as string)}
          options={providers.map((p) => ({
            label: p === 'all' ? '全部' : providerMeta(p).label,
            value: p,
          }))}
        />
        <Input
          prefix={<SearchOutlined />}
          placeholder="搜索模型名称 / ID / 提供商"
          style={{ width: 280 }}
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          allowClear
        />
      </div>

      <div className="model-grid">{renderCards()}</div>
    </div>
  );
}
