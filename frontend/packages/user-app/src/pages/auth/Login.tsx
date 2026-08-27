import { useEffect, useState } from 'react';
import { Form, Input, Button, App, Divider, Checkbox } from 'antd';
import { useNavigate, Link, useSearchParams } from 'react-router-dom';
import { request } from '../../api/request';
import { useAuthStore } from '../../store/auth';
import type { ApiResponse, LoginResponse } from '@mass/shared';
import AuthShell from './AuthShell';

const SAVED_EMAIL_KEY = 'mass_saved_email';

export default function Login() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [openidEnabled, setOpenidEnabled] = useState(false);
  const [remember, setRemember] = useState(() => !!localStorage.getItem(SAVED_EMAIL_KEY));
  const setToken = useAuthStore((s) => s.setToken);
  const adoptToken = useAuthStore((s) => s.adoptToken);
  const loadSiteConfig = useAuthStore((s) => s.loadSiteConfig);

  const openForgot = () => {
    modal.info({
      title: '重置密码',
      content:
        '当前平台通过「邮箱 + 验证码」完成身份校验。若忘记密码，请在注册使用的邮箱下重新获取验证码登录；如需强制重置，请联系平台管理员。',
      okText: '知道了',
    });
  };

  // 亦 OpenID 回调落地：oauth_token 直接登录 / oauth_bound 提示绑定成功
  useEffect(() => {
    loadSiteConfig();
    const oauthToken = params.get('oauth_token');
    if (oauthToken) {
      // 必须同步 zustand 内存状态：RequireAuth 读的是 store 而非 localStorage，
      // 否则跳转 / 后会被弹回 /login，丢失 query 导致快捷登录死循环
      adoptToken(oauthToken);
      navigate('/', { replace: true });
      return;
    }
    if (params.get('oauth_bound')) {
      message.success('亦 OpenID 绑定成功');
    }
    request
      .get('/auth/openid/config')
      .then((r) => setOpenidEnabled(r.data.data?.enabled === true))
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const onFinish = async (values: { email: string; password: string }) => {
    setLoading(true);
    try {
      const data = await request.post<ApiResponse<LoginResponse>>('/auth/login', values).then((r) => r.data.data);
      if (!data) return;
      // 记住我：仅保存邮箱用于下次预填，不保存密码
      if (remember) localStorage.setItem(SAVED_EMAIL_KEY, values.email);
      else localStorage.removeItem(SAVED_EMAIL_KEY);
      setToken(data.token, data.user);
      message.success('登录成功');
      navigate('/', { replace: true });
    } catch {
      /* 拦截器已提示 */
    } finally {
      setLoading(false);
    }
  };

  const startOpenIDLogin = () => {
    window.location.href = '/api/v1/auth/openid/authorize?intent=login';
  };

  return (
    <AuthShell>
      <div className="auth-fade">
        <h1 className="auth-title">欢迎回来</h1>
        <p className="auth-subtitle">登录你的 AI 平台账户</p>

        <Form
          className="auth-form"
          layout="vertical"
          size="large"
          requiredMark={false}
          initialValues={{ email: localStorage.getItem(SAVED_EMAIL_KEY) || '' }}
          onFinish={onFinish}
          autoComplete="on"
        >
          <Form.Item
            label="邮箱 / 用户名"
            name="email"
            rules={[
              { required: true, message: '请输入邮箱或用户名' },
              { type: 'email', message: '邮箱格式不正确' },
            ]}
          >
            <Input placeholder="请输入邮箱或用户名" autoComplete="email" />
          </Form.Item>
          <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="请输入密码" autoComplete="current-password" />
          </Form.Item>

          <div className="auth-row">
            <Checkbox checked={remember} onChange={(e) => setRemember(e.target.checked)}>
              记住我
            </Checkbox>
            <span className="auth-forgot" onClick={openForgot}>
              忘记密码？
            </span>
          </div>

          <Button type="primary" htmlType="submit" block loading={loading} className="auth-cta">
            {loading ? '登录中...' : '登录'}
          </Button>
        </Form>

        {openidEnabled && (
          <>
            <Divider plain className="auth-divider">或</Divider>
            <button type="button" className="auth-alt-btn" onClick={startOpenIDLogin}>
              使用亦 OpenID 快捷登录
            </button>
          </>
        )}

        <p className="auth-switch">
          还没有账号？<Link to="/register">立即注册</Link>
        </p>
      </div>
    </AuthShell>
  );
}
