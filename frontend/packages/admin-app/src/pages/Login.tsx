import { useEffect, useState } from 'react';
import { Form, Input, Button, App, Typography } from 'antd';
import { LockOutlined, MailOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import type { ApiResponse, LoginResponse } from '@mass/shared';
import { request } from '../api/request';
import { useAdminAuthStore } from '../store/auth';

export default function Login() {
  const navigate = useNavigate();
  const { message } = App.useApp();
  const setLogin = useAdminAuthStore((s) => s.setLogin);
  const loadSiteConfig = useAdminAuthStore((s) => s.loadSiteConfig);
  const siteName = useAdminAuthStore((s) => s.siteName);
  const siteLogo = useAdminAuthStore((s) => s.siteLogo);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    loadSiteConfig();
  }, [loadSiteConfig]);

  const onFinish = async (values: { email: string; password: string }) => {
    setLoading(true);
    try {
      const data = await request.post<ApiResponse<LoginResponse>>('/auth/login', values).then((r) => r.data.data);
      if (!data) return;
      if (data.user.role !== 'admin') {
        message.error('该账号无管理权限');
        return;
      }
      setLogin(data.token, data.user);
      message.success('登录成功');
      navigate('/', { replace: true });
    } catch {
      /* 拦截器已提示 */
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="admin-login-page">
      <div className="admin-login-card">
        <div className="admin-login-logo">
          {siteLogo ? (
            <img src={siteLogo} alt={siteName} />
          ) : (
            <span className="admin-login-mark">{siteName.trim().charAt(0) || 'M'}</span>
          )}
          <span className="admin-login-name">{siteName}</span>
        </div>
        <Typography.Title level={4} className="admin-login-title">
          管理后台登录
        </Typography.Title>
        <p className="admin-login-subtitle">请使用管理员账号登录</p>

        <Form className="admin-login-form" layout="vertical" size="large" requiredMark={false} onFinish={onFinish}>
          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { required: true, message: '请输入邮箱' },
              { type: 'email', message: '邮箱格式不正确' },
            ]}
          >
            <Input prefix={<MailOutlined />} placeholder="管理员邮箱" autoComplete="email" />
          </Form.Item>
          <Form.Item label="密码" name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" autoComplete="current-password" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 8 }}>
            <Button type="primary" htmlType="submit" block loading={loading} className="admin-login-cta">
              登录
            </Button>
          </Form.Item>
        </Form>
      </div>
    </div>
  );
}
