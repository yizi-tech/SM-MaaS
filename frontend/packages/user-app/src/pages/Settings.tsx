import { useEffect, useState } from 'react';
import {
  Card, Form, Input, InputNumber, Button, App, Tabs, Descriptions, Tag, Avatar, Typography, Alert,
} from 'antd';
import { UserOutlined, LinkOutlined, DisconnectOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router-dom';
import { useAuthStore } from '../store/auth';
import { request } from '../api/request';
import { tagColor, typeLabel } from '@mass/shared';

export default function Settings() {
  const { message } = App.useApp();
  const { user, loadSession, setToken } = useAuthStore();
  const [searchParams, setSearchParams] = useSearchParams();
  const [saving, setSaving] = useState(false);
  const [pwdSaving, setPwdSaving] = useState(false);
  const [countdown, setCountdown] = useState(0);
  const [openidStatus, setOpenidStatus] = useState<{ bound: boolean; username?: string } | null>(null);
  const [profileForm] = Form.useForm();
  const [pwdForm] = Form.useForm();

  useEffect(() => {
    if (user) profileForm.setFieldsValue({ nickname: user.nickname, phone: user.phone, qq: user.qq });
  }, [user, profileForm]);

  useEffect(() => {
    request
      .get('/user/openid/status')
      .then((r) => setOpenidStatus(r.data.data))
      .catch(() => setOpenidStatus({ bound: false }));
  }, []);

  // 亦 OpenID 绑定回调落地：提示成功并清理 URL 参数
  useEffect(() => {
    if (searchParams.get('oauth_bound')) {
      message.success('亦 OpenID 绑定成功');
      request
        .get('/user/openid/status')
        .then((r) => setOpenidStatus(r.data.data))
        .catch(() => {});
      setSearchParams({}, { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (countdown <= 0) return;
    const t = setTimeout(() => setCountdown((c) => c - 1), 1000);
    return () => clearTimeout(t);
  }, [countdown]);

  const saveProfile = async (values: { nickname: string; phone?: string; qq?: string }) => {
    setSaving(true);
    try {
      await request.put('/user/profile', values);
      message.success('资料已更新');
      await loadSession();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSaving(false);
    }
  };

  const sendCode = async () => {
    const email = user?.email;
    if (!email) return;
    await request.post('/user/password/send-code', { method: 'email', email });
    message.success('验证码已发送到注册邮箱');
    setCountdown(60);
  };

  const changePassword = async (values: { old_password: string; new_password: string; verify_code: string }) => {
    setPwdSaving(true);
    try {
      await request.put('/user/password', {
        old_password: values.old_password,
        new_password: values.new_password,
        verify_method: 'email',
        verify_code: values.verify_code,
      });
      message.success('密码已修改');
      pwdForm.resetFields();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setPwdSaving(false);
    }
  };

  const bindOpenID = () => {
    window.location.href = '/api/v1/auth/openid/authorize?intent=bind';
  };

  const unbindOpenID = async () => {
    await request.post('/user/openid/unbind');
    message.success('已解绑亦 OpenID');
    setOpenidStatus({ bound: false });
  };

  const [alertSaving, setAlertSaving] = useState(false);
  const saveAlert = async (values: { threshold: number }) => {
    setAlertSaving(true);
    try {
      await request.put('/user/token-alert', { threshold: values.threshold ?? 0 });
      message.success(values.threshold > 0 ? '已开启余额预警' : '已关闭余额预警');
      await loadSession();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setAlertSaving(false);
    }
  };

  const tabItems = [
    {
      key: 'profile',
      label: '基本资料',
      children: (
        <div style={{ maxWidth: 480 }}>
          <Descriptions column={1} size="small" style={{ marginBottom: 20 }}>
            <Descriptions.Item label="邮箱">{user?.email}</Descriptions.Item>
            <Descriptions.Item label="实名状态">
              <Tag color={tagColor(user?.real_name_status || 'unverified')}>
                {typeLabel[user?.real_name_status || 'unverified'] || user?.real_name_status}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="注册时间">
              {user ? new Date(user.created_at).toLocaleString('zh-CN') : '-'}
            </Descriptions.Item>
          </Descriptions>
          <Form form={profileForm} layout="vertical" onFinish={saveProfile}>
            <Form.Item name="nickname" label="昵称" rules={[{ required: true, min: 2, max: 50, message: '昵称 2-50 个字符' }]}>
              <Input maxLength={50} />
            </Form.Item>
            <Form.Item name="phone" label="手机号" rules={[{ pattern: /^$|^1\d{10}$/, message: '手机号格式不正确' }]}>
              <Input maxLength={11} placeholder="选填" />
            </Form.Item>
            <Form.Item name="qq" label="QQ" rules={[{ pattern: /^$|^\d{5,12}$/, message: 'QQ 格式不正确' }]}>
              <Input maxLength={20} placeholder="选填" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={saving}>
              保存资料
            </Button>
          </Form>
        </div>
      ),
    },
    {
      key: 'password',
      label: '修改密码',
      children: (
        <div style={{ maxWidth: 480 }}>
          <Alert
            style={{ marginBottom: 16 }}
            type="info"
            showIcon
            message="修改密码需要先获取注册邮箱的验证码"
          />
          <Form form={pwdForm} layout="vertical" onFinish={changePassword}>
            <Form.Item name="old_password" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]}>
              <Input.Password autoComplete="current-password" />
            </Form.Item>
            <Form.Item name="new_password" label="新密码" rules={[{ required: true, min: 6, message: '新密码至少 6 位' }]}>
              <Input.Password autoComplete="new-password" />
            </Form.Item>
            <Form.Item name="verify_code" label="邮箱验证码" rules={[{ required: true, message: '请输入验证码' }]}>
              <div style={{ display: 'flex', gap: 10 }}>
                <Input placeholder="6 位验证码" maxLength={6} />
                <Button onClick={sendCode} disabled={countdown > 0}>
                  {countdown > 0 ? `${countdown}s 后重发` : '获取验证码'}
                </Button>
              </div>
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={pwdSaving}>
              修改密码
            </Button>
          </Form>
        </div>
      ),
    },
    {
      key: 'token-alert',
      label: '余额预警',
      children: (
        <div style={{ maxWidth: 480 }}>
          <Alert
            style={{ marginBottom: 16 }}
            type="info"
            showIcon
            message="当您可用的 Token 余额（加油包余额 + 当前订阅套餐剩余额度）低于设定阈值时，系统会向注册邮箱发送提醒邮件。"
          />
          <Form
            layout="vertical"
            initialValues={{ threshold: user?.token_alert_threshold || 0 }}
            onFinish={saveAlert}
          >
            <Form.Item
              name="threshold"
              label="预警阈值（Token）"
              tooltip="低于该可用 Token 数量时发送邮件提醒；设为 0 表示关闭提醒"
              rules={[{ required: true, type: 'integer', min: 0, message: '请输入 >= 0 的整数' }]}
            >
              <InputNumber min={0} step={1000} style={{ width: '100%' }} placeholder="例如 100000" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={alertSaving}>
              保存预警设置
            </Button>
          </Form>
        </div>
      ),
    },
    {
      key: 'openid',
      label: '亦 OpenID 绑定',
      children: (
        <div style={{ maxWidth: 480 }}>
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="绑定状态">
              {openidStatus?.bound ? <Tag color="green">已绑定</Tag> : <Tag>未绑定</Tag>}
            </Descriptions.Item>
            {openidStatus?.bound && openidStatus.username && (
              <Descriptions.Item label="亦 OpenID 账号">{openidStatus.username}</Descriptions.Item>
            )}
          </Descriptions>
          <div style={{ marginTop: 16 }}>
            {openidStatus?.bound ? (
              <Button danger onClick={unbindOpenID} icon={<DisconnectOutlined />}>
                解绑亦 OpenID
              </Button>
            ) : (
              <Button type="primary" onClick={bindOpenID} icon={<LinkOutlined />}>
                绑定亦 OpenID 账号
              </Button>
            )}
          </div>
        </div>
      ),
    },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
        <Avatar size={56} src={user?.avatar} icon={<UserOutlined />} style={{ background: '#2563eb' }} />
        <div>
          <h2 style={{ margin: 0 }}>{user?.nickname}</h2>
          <p style={{ margin: 0 }}>{user?.email}</p>
        </div>
      </div>
      <Card>
        <Tabs items={tabItems} />
      </Card>
    </div>
  );
}
