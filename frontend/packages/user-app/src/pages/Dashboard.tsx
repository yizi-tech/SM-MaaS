import { useEffect, useState } from 'react';
import { Row, Col, Card, Statistic, Progress, Button, Table, Tag, Empty, Typography, Spin } from 'antd';
import {
  WalletOutlined,
  GiftOutlined,
  AccountBookOutlined,
  ProfileOutlined,
  ThunderboltOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import { useAuthStore } from '../store/auth';
import { request } from '../api/request';
import { formatMoney, tagColor, typeLabel } from '@mass/shared';

interface UsageSummary {
  start: string;
  end: string;
  total_tokens: number;
  total_cost: string;
  daily: { date: string; tokens: number; cost: string }[];
}

interface TxItem {
  id: number;
  transaction_no: string;
  type: string;
  amount: string;
  status: string;
  description: string;
  created_at: string;
}

export default function Dashboard() {
  const navigate = useNavigate();
  const { user, quota, refreshQuota } = useAuthStore();
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [recentTx, setRecentTx] = useState<TxItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    refreshQuota();
    Promise.all([
      request.get('/user/usage').then((r) => r.data.data as UsageSummary),
      request.get('/user/transactions?page=1&size=6').then((r) => r.data.data?.items || []),
    ])
      .then(([u, txs]) => {
        setUsage(u);
        setRecentTx(txs);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (loading || !user) return <Spin size="large" style={{ display: 'block', margin: '80px auto' }} />;

  const sub = quota?.subscription;
  const tokenCredits = quota?.token_credits ?? user.token_credits;
  const balance = quota?.balance ?? user.balance;
  const credit = quota?.credit ?? { limit: 0, used: 0, available: 0 };
  const maxTokens = Math.max(sub?.included_tokens || 0, 1);

  const txColumns = [
    { title: '类型', dataIndex: 'type', render: (t: string) => <Tag color={tagColor(t)}>{typeLabel[t] || t}</Tag> },
    { title: '金额', dataIndex: 'amount', render: (a: string) => <span className="money">{formatMoney(a)}</span> },
    { title: '说明', dataIndex: 'description', ellipsis: true },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('MM-DD HH:mm') },
  ];

  return (
    <div>
      <div className="page-header">
        <h2>你好，{user.nickname} 👋</h2>
        <p>集中查看账户资源、实时额度与近 30 天调用成本</p>
      </div>

      <div className="stat-grid">
        <Card className="data-card">
          <Statistic
            title="账户余额"
            value={parseFloat(balance)}
            precision={2}
            prefix={<WalletOutlined style={{ color: '#2563eb' }} />}
            suffix="¥"
          />
          <Button type="link" size="small" style={{ padding: 0 }} onClick={() => navigate('/billing')}>
            去充值 <RightOutlined />
          </Button>
        </Card>
        <Card className="data-card">
          <Statistic
            title="Token 加油包额度"
            value={tokenCredits}
            prefix={<GiftOutlined style={{ color: '#7c3aed' }} />}
          />
          <Button type="link" size="small" style={{ padding: 0 }} onClick={() => navigate('/token-packages')}>
            购买加油包 <RightOutlined />
          </Button>
        </Card>
        <Card className="data-card">
          <Statistic
            title="Token 授信额度"
            value={credit.available}
            prefix={<AccountBookOutlined style={{ color: '#059669' }} />}
          />
          <Button type="link" size="small" style={{ padding: 0 }} onClick={() => navigate('/credit')}>
            授信详情 <RightOutlined />
          </Button>
        </Card>
        <Card className="data-card">
          <Statistic
            title="近 30 天消费"
            value={parseFloat(usage?.total_cost || '0')}
            precision={2}
            prefix={<ThunderboltOutlined style={{ color: '#ea580c' }} />}
            suffix="¥"
          />
          <Button type="link" size="small" style={{ padding: 0 }} onClick={() => navigate('/usage')}>
            查看账单 <RightOutlined />
          </Button>
        </Card>
      </div>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={14}>
          <Card
            title={<span><ProfileOutlined /> 当前订阅</span>}
            extra={!sub && <Button type="link" onClick={() => navigate('/plans')}>查看套餐</Button>}
          >
            {sub ? (
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                  <div>
                    <Typography.Title level={4} style={{ margin: 0 }}>{sub.plan_name}</Typography.Title>
                    <Typography.Text type="secondary">
                      {dayjs(sub.start_at).format('YYYY-MM-DD')} ~ {dayjs(sub.end_at).format('YYYY-MM-DD')}
                      {sub.auto_renew ? '（自动续费）' : '（到期不续）'}
                    </Typography.Text>
                  </div>
                  <Tag color={tagColor(sub.status)}>{typeLabel[sub.status] || sub.status}</Tag>
                </div>
                <Progress
                  percent={Math.min(100, (sub.used_tokens / maxTokens) * 100)}
                  format={() => `${sub.used_tokens.toLocaleString()} / ${sub.included_tokens.toLocaleString()} tokens`}
                  strokeColor={{ '0%': '#2563eb', '100%': '#06b6d4' }}
                />
                <Typography.Text type="secondary">剩余 {sub.remaining_tokens.toLocaleString()} tokens</Typography.Text>
                <div style={{ marginTop: 16 }}>
                  <Button size="small" onClick={() => navigate('/plans')}>续费 / 升级</Button>
                  <Button size="small" style={{ marginLeft: 8 }} onClick={() => navigate('/reset-coupons')}>使用重置券</Button>
                </div>
              </div>
            ) : (
              <Empty description="当前没有生效的订阅套餐" image={Empty.PRESENTED_IMAGE_SIMPLE}>
                <Button type="primary" onClick={() => navigate('/plans')}>去订阅</Button>
              </Empty>
            )}
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card title="最近交易">
            {recentTx.length ? (
              <Table
                rowKey="id"
                columns={txColumns}
                dataSource={recentTx}
                size="small"
                pagination={false}
                scroll={{ x: 480 }}
              />
            ) : (
              <Empty description="暂无交易记录" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
}
