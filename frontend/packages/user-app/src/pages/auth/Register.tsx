import { useEffect, useState } from 'react';
import { Form, Input, Button, App, Segmented, Checkbox, Modal, Typography } from 'antd';
import {
  LockOutlined,
  MailOutlined,
  UserOutlined,
  MobileOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import { useNavigate, Link } from 'react-router-dom';
import { request } from '../../api/request';
import { useAuthStore } from '../../store/auth';
import type { ApiResponse, UserInfo } from '@mass/shared';
import AuthShell from './AuthShell';

export default function Register() {
  const navigate = useNavigate();
  const { message } = App.useApp();
  const loadSiteConfig = useAuthStore((s) => s.loadSiteConfig);
  const setToken = useAuthStore((s) => s.setToken);
  const [loading, setLoading] = useState(false);
  const [channel, setChannel] = useState<'email' | 'sms'>('email');
  const [countdown, setCountdown] = useState(0);
  const [agreement, setAgreement] = useState<null | 'service' | 'privacy'>(null);

  useEffect(() => {
    loadSiteConfig();
  }, [loadSiteConfig]);

  useEffect(() => {
    if (countdown <= 0) return;
    const t = setTimeout(() => setCountdown((c) => c - 1), 1000);
    return () => clearTimeout(t);
  }, [countdown]);

  const getAccount = () => document.querySelector<HTMLInputElement>('#reg_account')?.value || '';

  const sendCode = async () => {
    const account = getAccount();
    if (channel === 'email') {
      if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(account)) {
        message.warning('请先填写正确的邮箱');
        return;
      }
      await request.post('/auth/send-code', { method: 'email', email: account });
    } else {
      if (!/^1\d{10}$/.test(account)) {
        message.warning('请先填写正确的手机号');
        return;
      }
      await request.post('/auth/send-code', { method: 'sms', phone: account });
    }
    message.success('验证码已发送');
    setCountdown(60);
  };

  const onRegister = async (values: Record<string, string>) => {
    setLoading(true);
    try {
      const data = await request
        .post<ApiResponse<{ token: string; user: UserInfo }>>('/auth/register', {
          email: channel === 'email' ? values.account : undefined,
          phone: channel === 'sms' ? values.account : undefined,
          password: values.password,
          nickname: values.nickname,
          verify_method: channel,
          verify_code: values.verify_code,
        })
        .then((r) => r.data.data);
      if (!data) return;
      setToken(data.token, data.user);
      message.success('注册成功，正在进入控制台');
      navigate('/', { replace: true });
    } catch {
      /* 拦截器已提示 */
    } finally {
      setLoading(false);
    }
  };

  return (
    <AuthShell>
      <div className="auth-fade">
        <h1 className="auth-title">创建账户</h1>
        <p className="auth-subtitle">开始使用企业级 AI 服务</p>

        <Segmented
          block
          value={channel}
          onChange={(v) => setChannel(v as 'email' | 'sms')}
          options={[
            { label: '邮箱注册', value: 'email' },
            { label: '手机号注册', value: 'sms' },
          ]}
          style={{ marginBottom: 18 }}
        />

        <Form className="auth-form" layout="vertical" size="large" requiredMark={false} onFinish={onRegister}>
          <Form.Item
            name="nickname"
            label="姓名"
            rules={[{ required: true, min: 2, max: 50, message: '姓名 2-50 个字符' }]}
          >
            <Input prefix={<UserOutlined />} placeholder="请输入姓名" autoComplete="nickname" />
          </Form.Item>

          <Form.Item
            name="account"
            label={channel === 'email' ? '邮箱' : '手机号'}
            rules={
              channel === 'email'
                ? [{ required: true, type: 'email', message: '请输入正确的邮箱' }]
                : [{ required: true, pattern: /^1\d{10}$/, message: '请输入正确的手机号' }]
            }
          >
            <Input
              id="reg_account"
              prefix={channel === 'email' ? <MailOutlined /> : <MobileOutlined />}
              placeholder={channel === 'email' ? '请输入邮箱' : '请输入手机号'}
              maxLength={channel === 'sms' ? 11 : undefined}
              autoComplete={channel === 'email' ? 'email' : 'tel'}
            />
          </Form.Item>

          <Form.Item
            name="verify_code"
            label="验证码"
            rules={[{ required: true, message: '请输入验证码' }]}
            style={{ marginBottom: 16 }}
          >
            <div className="auth-code-field">
              <Input
                className="auth-code-input"
                prefix={<SafetyOutlined />}
                placeholder="请输入验证码"
                maxLength={6}
                autoComplete="one-time-code"
              />
              <Button className="auth-code-btn" onClick={sendCode} disabled={countdown > 0}>
                {countdown > 0 ? `${countdown}s 后重发` : '获取验证码'}
              </Button>
            </div>
          </Form.Item>

          <Form.Item
            name="password"
            label="密码"
            rules={[{ required: true, min: 6, message: '密码至少 6 位' }]}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder="请设置密码（至少 6 位）"
              autoComplete="new-password"
            />
            <div className="auth-pwd-hint">建议使用字母、数字与符号的组合，提升账户安全性</div>
          </Form.Item>

          <Form.Item
            name="confirm"
            label="确认密码"
            dependencies={['password']}
            rules={[
              { required: true, message: '请再次输入密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) return Promise.resolve();
                  return Promise.reject(new Error('两次输入的密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder="请再次输入密码"
              autoComplete="new-password"
            />
          </Form.Item>

          <Form.Item
            name="agree"
            valuePropName="checked"
            rules={[
              {
                validator: (_, v) =>
                  v ? Promise.resolve() : Promise.reject(new Error('请先阅读并同意服务协议与隐私政策')),
              },
            ]}
          >
            <Checkbox>
              我已阅读并同意
              <span className="auth-link" onClick={(e) => { e.preventDefault(); setAgreement('service'); }}>
                《服务协议》
              </span>
              和
              <span className="auth-link" onClick={(e) => { e.preventDefault(); setAgreement('privacy'); }}>
                《隐私政策》
              </span>
            </Checkbox>
          </Form.Item>

          <Button type="primary" htmlType="submit" block loading={loading} className="auth-cta">
            创建账户
          </Button>
        </Form>

        <p className="auth-switch">
          已有账号？<Link to="/login">立即登录</Link>
        </p>
      </div>

      <Modal
        open={agreement !== null}
        onCancel={() => setAgreement(null)}
        footer={null}
        title={agreement === 'service' ? '服务协议' : agreement === 'privacy' ? '隐私政策' : ''}
        width={640}
      >
        {agreement === 'service' && (
          <Typography.Paragraph style={{ whiteSpace: 'pre-wrap', lineHeight: 1.9 }}>
            {`服务协议

1. 服务说明
   本协议为您与本平台（以下简称“平台”）之间关于使用平台所提供的各类 AI 能力与相关服务的约定。平台将依约向您提供模型调用、API 接入、订阅套餐及额度管理等功能。

2. 账户与资格
   您需提供真实、准确、完整的注册信息，并对账户下的全部活动负责。平台有权对违法违规或异常使用的账户进行限制或封禁。

3. 使用规范
   您承诺不利用平台从事任何违反法律法规、侵害第三方权益或危害网络安全的行为，包括但不限于生成违法内容、攻击系统、滥用接口等。

4. 费用与订阅
   部分功能需付费使用。订阅套餐一经购买，相关额度与权益按套餐规则生效；自动续费可在账户中随时关闭。

5. 责任限制
   平台按“现状”提供服务，对于因不可抗力、网络故障或第三方原因导致的服务中断，平台不承担责任。

6. 协议变更
   平台保留根据业务需要修订本协议的权利，修订后将以适当方式公示，继续使用视为接受变更。`}
          </Typography.Paragraph>
        )}
        {agreement === 'privacy' && (
          <Typography.Paragraph style={{ whiteSpace: 'pre-wrap', lineHeight: 1.9 }}>
            {`隐私政策

1. 我们收集的信息
   为提供注册与登录、实名认证、计费与安全风控等必要功能，我们可能收集您的邮箱/手机号、昵称、密码（加密存储）、实名信息及使用日志。

2. 信息的使用
   所收集的信息仅用于身份核验、订单与额度核算、客服支持、安全风控及法律法规要求的场景，不会用于与您无关的商业营销。

3. 信息的保护
   我们采用加密传输与存储、访问控制等技术措施保护您的信息，防止未经授权的访问、泄露或篡改。

4. 信息的共享
   除法律法规要求、司法机关调取或您主动授权外，我们不会向第三方披露您的个人信息。

5. 您的权利
   您有权查询、更正自己的信息，并可申请注销账户。注销后我们将在合理期限内删除或匿名化处理您的个人数据。

6. 联系我们
   如对本政策有任何疑问，可通过平台公示的渠道与我们联系。`}
          </Typography.Paragraph>
        )}
      </Modal>
    </AuthShell>
  );
}
