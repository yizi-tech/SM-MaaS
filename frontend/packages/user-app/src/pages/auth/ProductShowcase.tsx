import { useAuthStore } from '../../store/auth';

/** 左侧产品视觉区：悬浮 Dashboard 卡片组（纯展示，无业务逻辑） */
function Stat({ label, value, delta }: { label: string; value: string; delta?: string }) {
  return (
    <div className="sc-stat">
      <span className="sc-stat-label">{label}</span>
      <span className="sc-stat-value">
        {value}
        {delta && <em className="sc-delta">{delta}</em>}
      </span>
    </div>
  );
}

export default function ProductShowcase() {
  const siteName = useAuthStore((s) => s.siteName);

  return (
    <div className="showcase" aria-hidden="true">
      <div className="showcase-stage">
        {/* 主网关概览卡（中心，轻微透视） */}
        <div className="sc-card sc-main">
          <div className="sc-main-head">
            <span className="sc-dot-row">
              <i />
              <i />
              <i />
            </span>
            <span className="sc-main-title">{siteName} · 实时概览</span>
          </div>
          <div className="sc-chart">
            {[42, 55, 48, 68, 60, 82, 74, 92].map((h, i) => (
              <span key={i} style={{ height: `${h}%` }} />
            ))}
          </div>
          <div className="sc-stats-grid">
            <Stat label="Tokens 本月" value="37.2M" delta="+18.4%" />
            <Stat label="Requests" value="128,492" />
            <Stat label="P99 延迟" value="82ms" />
            <Stat label="成功率" value="99.98%" />
          </div>
        </div>

        {/* API Usage */}
        <div className="sc-card sc-float sc-float--usage">
          <span className="sc-card-label">API Usage</span>
          <div>
            <span className="sc-big">37.2M</span>
            <span className="sc-unit">tokens</span>
          </div>
          <span className="sc-delta--up">↑ 18.4%</span>
        </div>

        {/* Model 状态 */}
        <div className="sc-card sc-float sc-float--model">
          <span className="sc-card-label">Model</span>
          <div className="sc-model-name">
            MiniMax H3 <span className="sc-online"><i />Online</span>
          </div>
          <div className="sc-kv">
            <span>Context</span>
            <b>128K</b>
          </div>
          <div className="sc-kv">
            <span>Latency</span>
            <b>82ms</b>
          </div>
        </div>

        {/* Inference */}
        <div className="sc-card sc-float sc-float--inference">
          <span className="sc-card-label">Inference</span>
          <div className="sc-kv">
            <span>Requests</span>
            <b>128,492</b>
          </div>
          <div className="sc-kv">
            <span>Success Rate</span>
            <b className="sc-good">99.98%</b>
          </div>
        </div>

        {/* Models 列表 */}
        <div className="sc-card sc-float sc-float--list sc-list">
          <span className="sc-card-label">Models</span>
          <ul>
            <li>MiniMax H3</li>
            <li>DeepSeek</li>
            <li>Qwen</li>
            <li>Emind</li>
          </ul>
        </div>

        {/* API Endpoint */}
        <div className="sc-endpoint">
          <span className="sc-mono">POST /v1/chat/completions</span>
          <span className="sc-online"><i />Operational</span>
        </div>
      </div>
    </div>
  );
}
