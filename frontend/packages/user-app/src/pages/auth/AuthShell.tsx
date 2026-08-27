import { useAuthStore } from '../../store/auth';
import ProductShowcase from './ProductShowcase';

/**
 * 登录/注册共享外壳：浅色背景 + 中央白色圆角容器
 * 左侧产品视觉区（≥1024px 显示）+ 右侧表单区
 */
export default function AuthShell({ children }: { children: React.ReactNode }) {
  const siteName = useAuthStore((s) => s.siteName);
  const siteLogo = useAuthStore((s) => s.siteLogo);

  return (
    <div className="auth-bg">
      <div className="auth-shell">
        <div className="auth-showcase">
          <ProductShowcase />
        </div>
        <div className="auth-panel">
          <div className="auth-logo">
            {siteLogo ? (
              <img src={siteLogo} alt={siteName} />
            ) : (
              <span className="auth-logo-mark">{siteName.trim().charAt(0) || 'M'}</span>
            )}
            <span className="auth-logo-name">{siteName}</span>
          </div>
          <div className="auth-content">{children}</div>
          <p className="auth-foot">企业级 AI 基础设施 · 统一网关 · 弹性计费</p>
        </div>
      </div>
    </div>
  );
}
