import { useEffect, useState } from 'react';
import { Row, Col, Card, Button, Typography, Tag, Modal, Descriptions, App, Spin, Empty, Table, Space } from 'antd';
import { CheckCircleOutlined, CrownOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { Plan, Subscription } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, formatNumber, tagColor, typeLabel } from '@mass/shared';
import { useAuthStore } from '../store/auth';

export default function Plans() {
  const { message } = App.useApp();
  const refreshQuota = useAuthStore((s) => s.refreshQuota);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [subs, setSubs] = useState<Subscription[]>([]);
  const [loading, setLoading] = useState(true);
  const [confirmPlan, setConfirmPlan] = useState<Plan | null>(null);
  const [subscribing, setSubscribing] = useState(false);

  const load = () => {
    setLoading(true);
    Promise.all([
      request.get('/plans').then((r) => r.data.data || []),
      request.get('/user/subscriptions').then((r) => r.data.data || []),
    ])
      .then(([p, s]) => {
        setPlans(p);
        setSubs(s);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const activeSub = subs.find((s) => s.status === 'active');

  const onSubscribe = async () => {
    if (!confirmPlan) return;
    setSubscribing(true);
    try {
      await request.post('/user/subscribe', { plan_id: confirmPlan.id });
      message.success('订阅成功');
      setConfirmPlan(null);
      refreshQuota();
      load();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSubscribing(false);
    }
  };

  const onCancelSub = async (id: number) => {
    await request.post(`/user/subscriptions/${id}/cancel`);
    message.success('已取消订阅');
    load();
  };

  const onToggleAutoRenew = async (id: number, autoRenew: boolean) => {
    await request.post(`/user/subscriptions/${id}/auto-renew`, { auto_renew: autoRenew });
    message.success(autoRenew ? '已开启自动续费' : '已关闭自动续费');
    load();
  };

  const subColumns = [
    { title: '套餐', dataIndex: 'plan_name', render: (n: string) => <Typography.Text strong>{n}</Typography.Text> },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '生效期', render: (_: unknown, r: Subscription) => `${dayjs(r.start_at).format('YYYY-MM-DD')} ~ ${dayjs(r.end_at).format('YYYY-MM-DD')}` },
    { title: '已用额度', render: (_: unknown, r: Subscription) => `${formatNumber(r.used_tokens)} / ${formatNumber(r.included_tokens)} tokens` },
    { title: '自动续费', dataIndex: 'auto_renew', render: (a: boolean) => (a ? <Tag color="green">开启</Tag> : <Tag>关闭</Tag>) },
    {
      title: '操作',
      render: (_: unknown, r: Subscription) =>
        r.status === 'active' && (
          <Button type="link" danger size="small" onClick={() => onCancelSub(r.id)}>
            取消订阅
          </Button>
        ),
    },
  ];

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '80px auto' }} />;

  return (
    <div>
      <div className="page-header">
        <h2>套餐订阅</h2>
        <p>选择适合你的套餐，订阅额度随用随减，支持余额支付</p>
      </div>

      {activeSub && (
        <Card style={{ marginBottom: 20, background: 'linear-gradient(120deg,#eff6ff,#e0f2fe)' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
            <div>
              <Space>
                <CrownOutlined style={{ color: '#d97706', fontSize: 20 }} />
                <Typography.Title level={5} style={{ margin: 0 }}>
                  当前订阅：{activeSub.plan_name}
                </Typography.Title>
              </Space>
              <div style={{ marginTop: 6 }}>
                <Typography.Text>
                  剩余 <b className="money">{formatNumber(activeSub.included_tokens - activeSub.used_tokens)}</b> /{' '}
                  {formatNumber(activeSub.included_tokens)} tokens，到期时间 {dayjs(activeSub.end_at).format('YYYY-MM-DD')}
                </Typography.Text>
              </div>
            </div>
            <div style={{ display: 'flex', gap: 8 }}>
              {activeSub.auto_renew ? (
                <Button onClick={() => onToggleAutoRenew(activeSub.id, false)}>
                  关闭自动续费
                </Button>
              ) : (
                <Button onClick={() => onToggleAutoRenew(activeSub.id, true)}>
                  开启自动续费
                </Button>
              )}
              <Button type="primary" onClick={() => document.getElementById('plans-grid')?.scrollIntoView({ behavior: 'smooth' })}>
                续费 / 升级
              </Button>
            </div>
          </div>
        </Card>
      )}

      <Row gutter={[16, 16]} id="plans-grid">
        {plans.map((p) => (
          <Col xs={24} sm={12} lg={8} key={p.id}>
            <Card
              hoverable
              style={{ height: '100%' }}
              styles={{
                body: { display: 'flex', flexDirection: 'column', height: '100%' },
              }}
            >
              <Typography.Title level={4} style={{ marginTop: 0 }}>{p.name}</Typography.Title>
              <Typography.Paragraph type="secondary" style={{ minHeight: 40 }}>
                {p.description}
              </Typography.Paragraph>
              <div style={{ marginBottom: 16 }}>
                <Typography.Text style={{ fontSize: 28, fontWeight: 700, color: '#2563eb' }} className="money">
                  {formatMoney(p.price)}
                </Typography.Text>
                <Typography.Text type="secondary"> / {p.duration_days} 天</Typography.Text>
              </div>
              <ul style={{ listStyle: 'none', padding: 0, flex: 1 }}>
                {[
                  `含 ${formatNumber(p.included_tokens)} tokens 额度`,
                  `RPM ${p.rpm} / TPM ${p.tpm.toLocaleString()}`,
                  `并发上限 ${p.concurrent_limit}`,
                  p.model_access?.length ? `模型白名单：${p.model_access.join(', ')}` : '全部模型可用',
                  p.max_purchase > 0 ? `每人限购 ${p.max_purchase} 次` : '不限购买次数',
                ].map((t) => (
                  <li key={t} style={{ marginBottom: 8, color: '#475569' }}>
                    <CheckCircleOutlined style={{ color: '#059669', marginRight: 8 }} />
                    {t}
                  </li>
                ))}
              </ul>
              <Button
                type={activeSub?.plan_name === p.name ? 'default' : 'primary'}
                block
                disabled={activeSub?.plan_name === p.name}
                onClick={() => setConfirmPlan(p)}
              >
                {activeSub?.plan_name === p.name ? '当前套餐' : '立即订阅'}
              </Button>
            </Card>
          </Col>
        ))}
      </Row>

      {subs.length > 0 && (
        <Card title="我的订阅记录" style={{ marginTop: 20 }}>
          <Table rowKey="id" columns={subColumns} dataSource={subs} pagination={false} scroll={{ x: 700 }} />
        </Card>
      )}

      <Modal
        title="确认订阅"
        open={!!confirmPlan}
        onOk={onSubscribe}
        confirmLoading={subscribing}
        onCancel={() => setConfirmPlan(null)}
      >
        {confirmPlan && (
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="套餐">{confirmPlan.name}</Descriptions.Item>
            <Descriptions.Item label="价格">{formatMoney(confirmPlan.price)}</Descriptions.Item>
            <Descriptions.Item label="时长">{confirmPlan.duration_days} 天</Descriptions.Item>
            <Descriptions.Item label="包含额度">{formatNumber(confirmPlan.included_tokens)} tokens</Descriptions.Item>
            <Descriptions.Item label="购买限制">{confirmPlan.max_purchase > 0 ? `每人限购 ${confirmPlan.max_purchase} 次` : '不限购买次数'}</Descriptions.Item>
            <Descriptions.Item label="说明">将从账户余额中扣除订阅费用，订阅生效后立即获得套餐额度。</Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  );
}
