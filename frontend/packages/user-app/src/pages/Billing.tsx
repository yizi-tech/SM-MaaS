import { useEffect, useState } from 'react';
import { Card, InputNumber, Button, App, Result, Typography, Row, Col, QRCode } from 'antd';
import {
  AlipayCircleOutlined,
  WechatOutlined,
  QqOutlined,
  WalletOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import { useAuthStore } from '../store/auth';
import { request } from '../api/request';
import { formatMoney } from '@mass/shared';

type Method = 'epay' | 'wechat' | 'alipay';
type EpaySub = 'alipay' | 'wxpay' | 'qqpay';

interface OrderInfo {
  out_trade_no: string;
  qr: string;
  openUrl?: string;
  amount: string;
}

export default function Billing() {
  const { message } = App.useApp();
  const refreshQuota = useAuthStore((s) => s.refreshQuota);
  const balance = useAuthStore((s) => s.quota?.balance);
  const [methods, setMethods] = useState<string[]>(['balance']);
  const [amount, setAmount] = useState<number>(100);
  const [method, setMethod] = useState<Method>('epay');
  const [epaySub, setEpaySub] = useState<EpaySub>('alipay');
  const [creating, setCreating] = useState(false);
  const [order, setOrder] = useState<OrderInfo | null>(null);
  const [paid, setPaid] = useState(false);

  const epayEnabled = methods.includes('epay');
  const wechatEnabled = methods.includes('wechat');
  const alipayEnabled = methods.includes('alipay');

  useEffect(() => {
    request
      .get('/user/payment-config')
      .then((r) => {
        const m = r.data.data?.methods || ['balance'];
        setMethods(m);
        const avail: Method[] = [];
        if (m.includes('wechat')) avail.push('wechat');
        if (m.includes('alipay')) avail.push('alipay');
        if (m.includes('epay')) avail.push('epay');
        if (avail.length) setMethod(avail[0]);
      })
      .catch(() => {});
  }, []);

  const presetAmounts = [50, 100, 200, 500, 1000];

  const createOrder = async () => {
    if (!amount || amount < 1) {
      message.warning('充值金额需在 ¥1 - ¥50000 之间');
      return;
    }
    setCreating(true);
    try {
      let data: any;
      if (method === 'wechat') {
        data = await request
          .post('/user/recharge/wechat', { amount: String(amount) })
          .then((r) => r.data.data);
        setOrder({
          out_trade_no: data.out_trade_no,
          qr: data.code_url,
          openUrl: data.code_url,
          amount: data.amount,
        });
      } else if (method === 'alipay') {
        data = await request
          .post('/user/recharge/alipay', { amount: String(amount) })
          .then((r) => r.data.data);
        setOrder({
          out_trade_no: data.out_trade_no,
          qr: data.qr_code,
          openUrl: data.qr_code,
          amount: data.amount,
        });
      } else {
        data = await request
          .post('/user/recharge/epay', { amount: String(amount), pay_type: epaySub })
          .then((r) => r.data.data);
        setOrder({
          out_trade_no: data.out_trade_no,
          qr: data.pay_url,
          openUrl: data.pay_url,
          amount: data.amount,
        });
      }
      setPaid(false);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setCreating(false);
    }
  };

  // 支付成功后轮询订单状态
  useEffect(() => {
    if (!order || paid) return;
    const timer = setInterval(async () => {
      try {
        const data = await request
          .get('/user/recharge/status', { params: { out_trade_no: order.out_trade_no } })
          .then((r) => r.data.data);
        if (data.status === 'success') {
          setPaid(true);
          clearInterval(timer);
          refreshQuota();
        }
      } catch {
        /* 忽略轮询错误 */
      }
    }, 3000);
    return () => clearInterval(timer);
  }, [order, paid, refreshQuota]);

  const openPay = () => {
    if (order?.openUrl) window.open(order.openUrl, '_blank', 'noopener');
  };

  const renderOrder = () => {
    if (paid) {
      return (
        <Result
          status="success"
          title="充值成功"
          subTitle={`订单 ${order?.out_trade_no} 已到账 ¥${order?.amount}`}
          extra={
            <Button type="primary" onClick={() => setOrder(null)}>
              继续充值
            </Button>
          }
        />
      );
    }
    return (
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 32, justifyContent: 'center', alignItems: 'center', padding: '16px 0' }}>
        <div style={{ textAlign: 'center' }}>
          <div
            style={{
              padding: 12,
              border: '1px solid #e5e7eb',
              borderRadius: 12,
              background: '#fff',
              display: 'inline-block',
            }}
          >
            <QRCode value={order?.qr || ''} size={176} />
          </div>
          <div style={{ marginTop: 8 }}>
            <Typography.Text type="secondary">扫码支付</Typography.Text>
          </div>
        </div>
        <div style={{ minWidth: 240, flex: 1, maxWidth: 360 }}>
          <Typography.Title level={4} className="money" style={{ marginTop: 0 }}>
            充值金额 {formatMoney(order?.amount)}
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ wordBreak: 'break-all' }}>
            订单号：{order?.out_trade_no}
          </Typography.Paragraph>
          <Button type="primary" size="large" block onClick={openPay} icon={<CheckCircleOutlined />}>
            前往支付
          </Button>
          <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
            支付完成后本页将自动确认（也可在新窗口支付后手动刷新查看余额）
          </Typography.Paragraph>
        </div>
      </div>
    );
  };

  const methodOptions: { value: Method; label: string; icon: React.ReactNode; enabled: boolean }[] = [
    {
      value: 'epay',
      label: '易支付（支付宝 / 微信 / QQ）',
      icon: <AlipayCircleOutlined style={{ color: '#1677ff', fontSize: 24 }} />,
      enabled: epayEnabled,
    },
    {
      value: 'wechat',
      label: '微信支付（原生）',
      icon: <WechatOutlined style={{ color: '#07c160', fontSize: 24 }} />,
      enabled: wechatEnabled,
    },
    {
      value: 'alipay',
      label: '支付宝（原生）',
      icon: <AlipayCircleOutlined style={{ color: '#1677ff', fontSize: 24 }} />,
      enabled: alipayEnabled,
    },
  ];

  const epaySubs: { value: EpaySub; label: string; icon: React.ReactNode }[] = [
    { value: 'alipay', label: '支付宝', icon: <AlipayCircleOutlined style={{ color: '#1677ff', fontSize: 22 }} /> },
    { value: 'wxpay', label: '微信', icon: <WechatOutlined style={{ color: '#07c160', fontSize: 22 }} /> },
    { value: 'qqpay', label: 'QQ 钱包', icon: <QqOutlined style={{ color: '#12b7f5', fontSize: 22 }} /> },
  ];

  const anyEnabled = epayEnabled || wechatEnabled || alipayEnabled;

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', flexWrap: 'wrap', gap: 12 }}>
        <div>
          <h2>充值中心</h2>
          <p>支持支付宝 / 微信 / QQ 钱包，充值即时到账</p>
        </div>
        <div
          style={{
            background: 'linear-gradient(120deg,#eff6ff,#e0f2fe)',
            border: '1px solid #bfdbfe',
            borderRadius: 12,
            padding: '10px 18px',
          }}
        >
          <Typography.Text type="secondary">当前余额</Typography.Text>
          <div style={{ fontSize: 22, fontWeight: 700 }} className="money">
            {balance ? formatMoney(balance) : '—'}
          </div>
        </div>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={14}>
          <Card title={<span><WalletOutlined /> 充值金额</span>}>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, marginBottom: 20 }}>
              {presetAmounts.map((v) => (
                <div
                  key={v}
                  onClick={() => setAmount(v)}
                  style={{
                    flex: '1 1 28%',
                    minWidth: 88,
                    textAlign: 'center',
                    padding: '14px 0',
                    borderRadius: 10,
                    cursor: 'pointer',
                    border: amount === v ? '1.5px solid #2563eb' : '1px solid #e5e7eb',
                    background: amount === v ? '#eff6ff' : '#fff',
                    color: amount === v ? '#2563eb' : '#0f172a',
                    fontWeight: 600,
                    fontSize: 16,
                    transition: 'all .15s',
                  }}
                >
                  ¥{v}
                </div>
              ))}
            </div>
            <InputNumber
              size="large"
              min={1}
              max={50000}
              value={amount}
              onChange={(v) => setAmount(v ?? 0)}
              style={{ width: '100%' }}
              addonBefore="¥"
              placeholder="自定义金额"
            />
            <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0 }}>
              充值金额范围 ¥1.00 - ¥50000.00
            </Typography.Paragraph>
          </Card>
        </Col>

        <Col xs={24} md={10}>
          <Card title="支付方式" style={{ height: '100%' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              {methodOptions
                .filter((o) => o.enabled)
                .map((o) => {
                  const active = method === o.value;
                  return (
                    <div
                      key={o.value}
                      onClick={() => setMethod(o.value)}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 12,
                        padding: '12px 14px',
                        borderRadius: 10,
                        cursor: 'pointer',
                        border: active ? '1.5px solid #2563eb' : '1px solid #e5e7eb',
                        background: active ? '#eff6ff' : '#fff',
                      }}
                    >
                      {o.icon}
                      <span style={{ fontWeight: 600 }}>{o.label}</span>
                      {active && <CheckCircleOutlined style={{ color: '#2563eb', marginLeft: 'auto' }} />}
                    </div>
                  );
                })}
            </div>
            {method === 'epay' && (
              <div style={{ marginTop: 16 }}>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  易支付渠道
                </Typography.Text>
                <div style={{ display: 'flex', gap: 10, marginTop: 8, flexWrap: 'wrap' }}>
                  {epaySubs.map((s) => {
                    const active = epaySub === s.value;
                    return (
                      <div
                        key={s.value}
                        onClick={() => setEpaySub(s.value)}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 8,
                          padding: '8px 12px',
                          borderRadius: 8,
                          cursor: 'pointer',
                          border: active ? '1.5px solid #2563eb' : '1px solid #e5e7eb',
                          background: active ? '#eff6ff' : '#fff',
                        }}
                      >
                        {s.icon}
                        <span style={{ fontWeight: 600, fontSize: 13 }}>{s.label}</span>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
            {!anyEnabled && (
              <Typography.Paragraph type="warning" style={{ marginTop: 16, marginBottom: 0 }}>
                支付渠道未启用，请联系管理员配置支付网关。
              </Typography.Paragraph>
            )}
          </Card>
        </Col>
      </Row>

      <Card style={{ marginTop: 16 }}>
        {!order ? (
          <Button
            type="primary"
            size="large"
            block
            disabled={!anyEnabled}
            loading={creating}
            onClick={createOrder}
          >
            立即充值 ¥{amount || 0}
          </Button>
        ) : (
          renderOrder()
        )}
      </Card>
    </div>
  );
}
