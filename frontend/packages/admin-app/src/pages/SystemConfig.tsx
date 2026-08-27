import { useEffect, useMemo, useState } from 'react';
import { Card, Form, Input, Switch, Button, Tabs, Typography, App, Divider } from 'antd';
import {
  SaveOutlined,
  ReloadOutlined,
  SkinOutlined,
  SafetyCertificateOutlined,
  PayCircleOutlined,
  AlipayCircleOutlined,
  WechatOutlined,
} from '@ant-design/icons';
import type { SystemConfig } from '@mass/shared';
import { request } from '../api/request';

interface GroupMeta {
  key: string;
  label: string;
  desc: string;
  icon: React.ReactNode;
  /** 组内小节：key -> 标题（按顺序渲染） */
  sections: { key: string; title: string }[];
}

type FieldType = 'switch' | 'textarea' | 'password';

interface FieldMeta {
  label: string;
  type?: FieldType;
  placeholder?: string;
  desc?: string;
  section: string;
  /** 独占整行（textarea 等） */
  full?: boolean;
  /** 等宽字体（密钥 / 证书等多行文本） */
  mono?: boolean;
}

const groups: GroupMeta[] = [
  {
    key: 'site',
    label: '站点品牌',
    desc: '展示在控制台与落地页的品牌信息，公开可读',
    icon: <SkinOutlined />,
    sections: [
      { key: 'basic', title: '基本信息' },
      { key: 'brand', title: '品牌展示' },
      { key: 'legal', title: '法务与页脚' },
    ],
  },
  {
    key: 'oauth',
    label: 'OpenID 登录',
    desc: '亦 OpenID 第三方登录（OAuth2）配置',
    icon: <SafetyCertificateOutlined />,
    sections: [{ key: 'conn', title: '连接配置' }],
  },
  {
    key: 'pay',
    label: '易支付',
    desc: '易支付网关充值配置，保存后立即生效（无需重启）',
    icon: <PayCircleOutlined style={{ color: '#722ed1' }} />,
    sections: [{ key: 'gateway', title: '网关参数' }],
  },
  {
    key: 'wechat',
    label: '微信支付',
    desc: '原生微信支付（V3 Native 扫码），保存后立即生效（无需重启）',
    icon: <WechatOutlined style={{ color: '#07c160' }} />,
    sections: [{ key: 'wx', title: '微信支付参数' }],
  },
  {
    key: 'alipay',
    label: '支付宝',
    desc: '原生支付宝（扫码预创建），保存后立即生效（无需重启）',
    icon: <AlipayCircleOutlined style={{ color: '#1677ff' }} />,
    sections: [{ key: 'ali', title: '支付宝参数' }],
  },
];

const fieldMeta: Record<string, FieldMeta> = {
  site_name: { label: '站点名称', placeholder: 'MASS 平台', desc: '浏览器标题与侧边栏品牌名', section: 'basic' },
  site_url: { label: '站点 URL', placeholder: 'https://mass.yiziyun.com', desc: 'OAuth 回调与邮件链接的基准地址', section: 'basic' },
  site_logo: {
    label: 'Logo URL',
    type: 'textarea',
    placeholder: 'https://mass.yiziyun.com/landing-assets/logo.png',
    desc: '控制台侧边栏与登录页 logo，建议 512×512 PNG',
    section: 'brand',
    full: true,
  },
  site_description: {
    label: '站点描述',
    type: 'textarea',
    placeholder: '一句话介绍你的平台…',
    desc: '落地页 SEO 与分享摘要文案',
    section: 'brand',
    full: true,
  },
  site_icp: { label: 'ICP 备案号', placeholder: '粤ICP备xxxxxxxx号', desc: '展示于页脚', section: 'brand' },
  site_footer: {
    label: '页脚文案',
    type: 'textarea',
    placeholder: '© 2026 MASS Platform.',
    section: 'legal',
    full: true,
  },
  legal_terms: {
    label: '服务条款',
    type: 'textarea',
    placeholder: '支持 HTML 或纯文本',
    desc: '注册页与用户协议入口展示',
    section: 'legal',
    full: true,
  },
  legal_privacy: {
    label: '隐私政策',
    type: 'textarea',
    placeholder: '支持 HTML 或纯文本',
    desc: '注册页与隐私政策入口展示',
    section: 'legal',
    full: true,
  },
  oauth_enabled: {
    label: '启用亦 OpenID 登录',
    type: 'switch',
    desc: '开启后在用户登录页展示「快捷登录」入口',
    section: 'conn',
  },
  oauth_server: {
    label: '授权服务器',
    placeholder: 'https://account.yiziyun.com',
    desc: 'OpenID 服务基址',
    section: 'conn',
  },
  oauth_client_id: { label: 'Client ID', placeholder: '', section: 'conn' },
  oauth_client_secret: { label: 'Client Secret', type: 'password', section: 'conn' },
  oauth_redirect_uri: {
    label: '回调地址',
    placeholder: 'https://mass.yiziyun.com/oauth/yiziauth-login',
    desc: '必须与授权服务器登记的完全一致',
    section: 'conn',
    full: true,
  },
  pay_epay_enabled: {
    label: '启用易支付充值',
    type: 'switch',
    desc: '关闭后用户端隐藏在线充值入口',
    section: 'gateway',
  },
  pay_epay_gateway: { label: '网关地址', placeholder: 'https://pay.example.com', section: 'gateway' },
  pay_epay_pid: { label: '商户 PID', section: 'gateway' },
  pay_epay_key: { label: '商户密钥', type: 'password', section: 'gateway' },
  pay_epay_sign_upper: {
    label: '签名使用大写',
    type: 'switch',
    desc: '部分网关要求签名结果字母大写',
    section: 'gateway',
  },
  pay_wechat_enabled: {
    label: '启用微信支付',
    type: 'switch',
    desc: '关闭后用户端隐藏微信支付入口',
    section: 'wx',
  },
  pay_wechat_appid: { label: 'AppID', section: 'wx' },
  pay_wechat_mchid: { label: '商户号 MchID', section: 'wx' },
  pay_wechat_api_key: { label: 'APIv3 密钥', type: 'password', section: 'wx' },
  pay_wechat_serial: { label: '商户证书序列号', section: 'wx' },
  pay_wechat_private_key: {
    label: '商户 API 私钥',
    type: 'textarea',
    placeholder: '-----BEGIN PRIVATE KEY-----',
    desc: '商户平台下载的 apiclient_key.pem',
    section: 'wx',
    full: true,
    mono: true,
  },
  pay_wechat_notify_url: {
    label: '异步回调地址',
    placeholder: 'https://mass.yiziyun.com/api/v1/pay/wechat/notify',
    desc: '微信支付结果推送地址',
    section: 'wx',
    full: true,
  },
  pay_alipay_enabled: {
    label: '启用支付宝',
    type: 'switch',
    desc: '关闭后用户端隐藏支付宝入口',
    section: 'ali',
  },
  pay_alipay_appid: { label: 'AppId', section: 'ali' },
  pay_alipay_gateway: {
    label: '网关地址',
    placeholder: 'https://openapi.alipay.com/gateway.do',
    section: 'ali',
  },
  pay_alipay_private_key: {
    label: '应用私钥',
    type: 'textarea',
    placeholder: '-----BEGIN PRIVATE KEY-----',
    desc: '应用公钥对应的私钥（PKCS8）',
    section: 'ali',
    full: true,
    mono: true,
  },
  pay_alipay_public_key: {
    label: '支付宝公钥',
    type: 'textarea',
    placeholder: '支付宝开放平台获取的应用公钥证书/公钥',
    desc: '用于验签异步通知',
    section: 'ali',
    full: true,
    mono: true,
  },
  pay_alipay_notify_url: {
    label: '异步回调地址',
    placeholder: 'https://mass.yiziyun.com/api/v1/pay/alipay/notify',
    section: 'ali',
    full: true,
  },
  pay_alipay_return_url: {
    label: '同步回跳地址',
    placeholder: 'https://mass.yiziyun.com/user/recharge',
    desc: '支付完成后浏览器跳转地址',
    section: 'ali',
    full: true,
  },
};

function SwitchField({ name, label, desc }: { name: string; label: string; desc?: string }) {
  return (
    <div className="config-switch-row">
      <div style={{ minWidth: 0 }}>
        <Typography.Text strong style={{ fontSize: 13.5 }}>{label}</Typography.Text>
        {desc && (
          <Typography.Paragraph type="secondary" style={{ margin: 0, fontSize: 12 }}>
            {desc}
          </Typography.Paragraph>
        )}
      </div>
      <Form.Item name={name} valuePropName="checked" noStyle>
        <Switch />
      </Form.Item>
    </div>
  );
}

export default function SystemConfig() {
  const { message } = App.useApp();
  const [configs, setConfigs] = useState<SystemConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const load = () => {
    setLoading(true);
    request
      .get('/admin/config')
      .then((r) => setConfigs(r.data.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(), []);

  const grouped = useMemo(() => {
    const map: Record<string, SystemConfig[]> = {};
    for (const c of configs) {
      const g = c.group || 'site';
      if (!map[g]) map[g] = [];
      map[g].push(c);
    }
    return map;
  }, [configs]);

  const save = async (group: string, values: Record<string, string>) => {
    const items = Object.entries(values)
      .filter(([, v]) => v !== undefined && v !== null)
      .map(([key, value]) => ({ key, value: String(value) }));
    if (items.length === 0) return;
    setSaving(true);
    try {
      await request.put('/admin/config/batch', { group, items });
      message.success('已保存');
      load();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <div className="page-header">
        <h2>系统配置</h2>
        <p>管理平台品牌、第三方登录与支付网关配置</p>
      </div>

      <Tabs
        type="card"
        items={groups.map((g) => {
          const groupConfigs = grouped[g.key] || [];
          const existingKeys = new Set(groupConfigs.map((c) => c.key));
          const prefix =
            g.key === 'oauth' ? 'oauth_'
            : g.key === 'pay' ? 'pay_epay_'
            : g.key === 'wechat' ? 'pay_wechat_'
            : g.key === 'alipay' ? 'pay_alipay_'
            : '';
          const formItems = Object.entries(fieldMeta).filter(([key]) =>
            g.key === 'site'
              ? !key.startsWith('oauth_') &&
                !key.startsWith('pay_epay_') &&
                !key.startsWith('pay_wechat_') &&
                !key.startsWith('pay_alipay_')
              : key.startsWith(prefix)
          );
          const initialValues: Record<string, string | boolean> = {};
          for (const [key, meta] of formItems) {
            if (!existingKeys.has(key) && meta.type !== 'switch') continue;
            const cfg = groupConfigs.find((c) => c.key === key);
            initialValues[key] = cfg ? (meta.type === 'switch' ? cfg.value === 'true' || cfg.value === '1' : cfg.value) : (meta.type === 'switch' ? false : '');
          }

          return {
            key: g.key,
            label: (
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: 7 }}>
                {g.icon}
                {g.label}
              </span>
            ),
            children: (
              <Card
                loading={loading}
                style={{ maxWidth: 920 }}
                styles={{ body: { paddingTop: 4 } }}
                title={
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 10, flexWrap: 'wrap' }}>
                    <span>{g.label}</span>
                    <Typography.Text type="secondary" style={{ fontSize: 12.5, fontWeight: 400 }}>
                      {g.desc}
                    </Typography.Text>
                  </div>
                }
              >
                {formItems.length > 0 ? (
                  <Form
                    layout="vertical"
                    requiredMark={false}
                    initialValues={initialValues}
                    onFinish={(values) => save(g.key, values as Record<string, string>)}
                    key={g.key}
                  >
                    {g.sections.map((sec, idx) => {
                      const secFields = formItems.filter(([, m]) => m.section === sec.key);
                      if (secFields.length === 0) return null;
                      return (
                        <div key={sec.key}>
                          <Divider
                            orientation="left"
                            orientationMargin="0"
                            style={{ marginTop: idx === 0 ? 8 : 24, marginBottom: 18 }}
                          >
                            <span className="config-section-title">{sec.title}</span>
                          </Divider>
                          <div className="config-grid">
                            {secFields.map(([key, meta]) =>
                              meta.type === 'switch' ? (
                                <SwitchField key={key} name={key} label={meta.label} desc={meta.desc} />
                              ) : (
                                <Form.Item
                                  key={key}
                                  name={key}
                                  label={meta.label}
                                  extra={meta.desc}
                                  className={meta.full ? 'full' : ''}
                                  valuePropName="value"
                                  style={{ marginBottom: 18 }}
                                >
                                  {meta.type === 'textarea' ? (
                                    <Input.TextArea
                                      rows={3}
                                      autoSize={{ minRows: 3, maxRows: 14 }}
                                      placeholder={meta.placeholder}
                                      maxLength={5000}
                                      className={meta.mono ? 'config-mono' : ''}
                                    />
                                  ) : meta.type === 'password' ? (
                                    <Input.Password placeholder={meta.placeholder} autoComplete="new-password" />
                                  ) : (
                                    <Input placeholder={meta.placeholder} />
                                  )}
                                </Form.Item>
                              ),
                            )}
                          </div>
                        </div>
                      );
                    })}
                    <div className="config-footer">
                      <Button icon={<ReloadOutlined />} onClick={load}>
                        刷新
                      </Button>
                      <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
                        保存修改
                      </Button>
                    </div>
                  </Form>
                ) : (
                  <Typography.Text type="secondary">该分组暂无配置项，保存后将自动创建。</Typography.Text>
                )}
              </Card>
            ),
          };
        })}
      />
    </div>
  );
}
