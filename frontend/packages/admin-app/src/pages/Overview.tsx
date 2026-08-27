import { useEffect, useState } from 'react';
import { Row, Col, Card, Statistic, DatePicker, Typography, Spin } from 'antd';
import {
  DollarOutlined,
  UserOutlined,
  CrownOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import {
  AreaChart,
  Area,
  LineChart,
  Line,
  PieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import dayjs, { Dayjs } from 'dayjs';
import type { AnalyticsOverview, DailyAnalyticsItem } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney } from '@mass/shared';

const PIE_COLORS = ['#2563eb', '#e2e8f0'];

export default function Overview() {
  const [overview, setOverview] = useState<AnalyticsOverview | null>(null);
  const [daily, setDaily] = useState<DailyAnalyticsItem[]>([]);
  const [range, setRange] = useState<[Dayjs, Dayjs]>([dayjs().subtract(29, 'day'), dayjs()]);
  const [loading, setLoading] = useState(true);
  const [chartLoading, setChartLoading] = useState(false);

  useEffect(() => {
    request
      .get('/admin/analytics/overview')
      .then((r) => setOverview(r.data.data as AnalyticsOverview))
      .catch(() => {});
  }, []);

  useEffect(() => {
    setChartLoading(true);
    request
      .get('/admin/analytics/daily', {
        params: {
          start_date: range[0].format('YYYY-MM-DD'),
          end_date: range[1].format('YYYY-MM-DD'),
        },
      })
      .then((r) => setDaily((r.data.data as DailyAnalyticsItem[]) || []))
      .catch(() => {})
      .finally(() => {
        setChartLoading(false);
        setLoading(false);
      });
  }, [range]);

  if (loading || !overview) return <Spin size="large" style={{ display: 'block', margin: '80px auto' }} />;

  const pieData = [
    { name: '有效订阅', value: overview.active_subscriptions },
    { name: '未订阅 / 免费', value: Math.max(overview.total_users - overview.active_subscriptions, 0) },
  ];

  const tickFmt = (d: string) => d.slice(5);

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
        <div>
          <h2>数据概览</h2>
          <p>平台整体运营数据一览</p>
        </div>
        <DatePicker.RangePicker
          value={range}
          allowClear={false}
          onChange={(v) => v && setRange(v as [Dayjs, Dayjs])}
        />
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card className="data-card">
            <Statistic title="累计收入" value={parseFloat(overview.total_revenue)} precision={2} prefix={<DollarOutlined style={{ color: '#d97706' }} />} suffix="¥" />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card className="data-card">
            <Statistic title="活跃用户" value={overview.active_users} prefix={<UserOutlined style={{ color: '#059669' }} />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card className="data-card">
            <Statistic title="付费订阅数" value={overview.active_subscriptions} prefix={<CrownOutlined style={{ color: '#7c3aed' }} />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card className="data-card">
            <Statistic title="今日请求量" value={overview.today_requests} prefix={<ThunderboltOutlined style={{ color: '#ea580c' }} />} />
          </Card>
        </Col>
      </Row>

      <Spin spinning={chartLoading}>
        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col xs={24} lg={12}>
            <Card title="收入趋势">
              <ResponsiveContainer width="100%" height={280}>
                <AreaChart data={daily} margin={{ top: 10, right: 16, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="rev" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#2563eb" stopOpacity={0.8} />
                      <stop offset="95%" stopColor="#2563eb" stopOpacity={0.05} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#eee" />
                  <XAxis dataKey="date" tickFormatter={tickFmt} fontSize={12} />
                  <YAxis fontSize={12} width={48} />
                  <Tooltip formatter={(v) => [`¥${v}`, '收入']} />
                  <Area type="monotone" dataKey="revenue" name="收入" stroke="#2563eb" fill="url(#rev)" />
                </AreaChart>
              </ResponsiveContainer>
            </Card>
          </Col>
          <Col xs={24} lg={12}>
            <Card title="请求量趋势">
              <ResponsiveContainer width="100%" height={280}>
                <AreaChart data={daily} margin={{ top: 10, right: 16, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="req" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#ea580c" stopOpacity={0.8} />
                      <stop offset="95%" stopColor="#ea580c" stopOpacity={0.05} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#eee" />
                  <XAxis dataKey="date" tickFormatter={tickFmt} fontSize={12} />
                  <YAxis fontSize={12} width={48} />
                  <Tooltip formatter={(v) => [v, '请求量']} />
                  <Area type="monotone" dataKey="requests" name="请求量" stroke="#ea580c" fill="url(#req)" />
                </AreaChart>
              </ResponsiveContainer>
            </Card>
          </Col>
        </Row>

        <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
          <Col xs={24} lg={12}>
            <Card title="新增用户趋势">
              <ResponsiveContainer width="100%" height={280}>
                <LineChart data={daily} margin={{ top: 10, right: 16, left: 0, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#eee" />
                  <XAxis dataKey="date" tickFormatter={tickFmt} fontSize={12} />
                  <YAxis fontSize={12} width={48} />
                  <Tooltip formatter={(v) => [v, '新增用户']} />
                  <Line type="monotone" dataKey="new_users" name="新增用户" stroke="#059669" strokeWidth={2} dot={false} />
                  <Line type="monotone" dataKey="new_subs" name="新增订阅" stroke="#7c3aed" strokeWidth={2} dot={false} />
                  <Legend />
                </LineChart>
              </ResponsiveContainer>
            </Card>
          </Col>
          <Col xs={24} lg={12}>
            <Card title="订阅构成">
              <ResponsiveContainer width="100%" height={280}>
                <PieChart>
                  <Pie
                    data={pieData}
                    dataKey="value"
                    nameKey="name"
                    innerRadius={60}
                    outerRadius={100}
                    paddingAngle={2}
                    label={(e: { name?: string; percent?: number }) =>
                      `${e.name} ${((e.percent || 0) * 100).toFixed(0)}%`
                    }
                  >
                    {pieData.map((_, i) => (
                      <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip />
                  <Legend />
                </PieChart>
              </ResponsiveContainer>
            </Card>
          </Col>
        </Row>
      </Spin>
    </div>
  );
}
