import { useEffect, useState } from 'react';
import { Card, Tabs, Table, Tag, Typography, Select, Space, Button, Drawer, Descriptions, DatePicker, App } from 'antd';
import { DownloadOutlined, EyeOutlined } from '@ant-design/icons';
import dayjs, { Dayjs } from 'dayjs';
import type { BillingRecord, Transaction } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, formatNumber, formatDuration, tagColor, typeLabel } from '@mass/shared';

// 轻量迷你趋势条（避免引入重图表库）
function MiniBar({ data }: { data: { date: string; tokens: number }[] }) {
  const max = Math.max(...data.map((d) => d.tokens), 1);
  return (
    <div style={{ display: 'flex', alignItems: 'flex-end', gap: 2, height: 80 }}>
      {data.map((d, i) => (
        <div
          key={i}
          title={`${d.date}: ${d.tokens.toLocaleString()} tokens`}
          style={{
            flex: 1,
            background: 'linear-gradient(180deg,#60a5fa,#2563eb)',
            borderRadius: '2px 2px 0 0',
            height: `${Math.max((d.tokens / max) * 100, 2)}%`,
            opacity: 0.5 + (i / data.length) * 0.5,
          }}
        />
      ))}
    </div>
  );
}

export default function Usage() {
  const { message } = App.useApp();
  const [range, setRange] = useState<[Dayjs, Dayjs]>([dayjs().subtract(29, 'day'), dayjs()]);
  const [daily, setDaily] = useState<{ date: string; tokens: number; cost: string }[]>([]);
  const [summary, setSummary] = useState({ total_tokens: 0, total_cost: '0' });
  const [recordLoading, setRecordLoading] = useState(false);
  const [txLoading, setTxLoading] = useState(false);
  const [records, setRecords] = useState<BillingRecord[]>([]);
  const [txs, setTxs] = useState<Transaction[]>([]);
  const [recordTotal, setRecordTotal] = useState(0);
  const [txTotal, setTxTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [txPage, setTxPage] = useState(1);
  const [detail, setDetail] = useState<BillingRecord | null>(null);

  const loadUsage = (start: string, end: string) => {
    request
      .get('/user/usage', { params: { start, end } })
      .then((r) => {
        const d = r.data.data;
        setDaily(d.daily || []);
        setSummary({ total_tokens: d.total_tokens, total_cost: d.total_cost });
      })
      .catch(() => {});
  };

  const loadRecords = (p: number) => {
    setRecordLoading(true);
    request
      .get('/user/billing-records', { params: { page: p, size } })
      .then((r) => {
        setRecords(r.data.data?.items || []);
        setRecordTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setRecordLoading(false));
  };

  const loadTxs = (p: number) => {
    setTxLoading(true);
    request
      .get('/user/transactions', { params: { page: p, size } })
      .then((r) => {
        setTxs(r.data.data?.items || []);
        setTxTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setTxLoading(false));
  };

  useEffect(() => {
    loadUsage(range[0].toISOString(), range[1].toISOString());
  }, [range]);

  useEffect(() => {
    loadRecords(1);
    loadTxs(1);
  }, []);

  const recordColumns = [
    {
      title: '模型',
      dataIndex: 'model',
      render: (m: string) => <Typography.Text code>{m}</Typography.Text>,
    },
    { title: 'Provider', dataIndex: 'provider', width: 100 },
    { title: '输入 tokens', dataIndex: 'tokens_in', render: (n: number) => formatNumber(n) },
    { title: '输出 tokens', dataIndex: 'tokens_out', render: (n: number) => formatNumber(n) },
    {
      title: '缓存 tokens',
      dataIndex: 'cached_tokens',
      render: (n: number) => (n ? formatNumber(n) : '-'),
    },
    {
      title: '计费类型',
      dataIndex: 'billing_type',
      render: (t: string) => <Tag color={t === 'subscription' ? 'geekblue' : 'gold'}>{t === 'subscription' ? '订阅额度' : '按量计费'}</Tag>,
    },
    { title: '耗时', dataIndex: 'duration_ms', render: (v: number) => formatDuration(v || 0) },
    {
      title: '费用',
      dataIndex: 'cost',
      render: (c: string) => <span className="money" style={{ fontWeight: 600 }}>{formatMoney(c)}</span>,
    },
    { title: '时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm:ss') },
    {
      title: '操作',
      render: (_: unknown, r: BillingRecord) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setDetail(r)}>
          详情
        </Button>
      ),
    },
  ];

  const txColumns = [
    { title: '交易号', dataIndex: 'transaction_no', render: (t: string) => <Typography.Text code style={{ fontSize: 12 }}>{t}</Typography.Text> },
    { title: '类型', dataIndex: 'type', render: (t: string) => <Tag color={tagColor(t)}>{typeLabel[t] || t}</Tag> },
    { title: '金额', dataIndex: 'amount', render: (a: string) => <span className="money" style={{ fontWeight: 600 }}>{formatMoney(a)}</span> },
    { title: '余额变化', render: (_: unknown, r: Transaction) => (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {formatMoney(r.balance_before)} → {formatMoney(r.balance_after)}
        </Typography.Text>
      ) },
    { title: '说明', dataIndex: 'description', ellipsis: true },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
  ];

  const exportJsonl = async () => {
    try {
      const res = await request.get('/user/conversations/export.jsonl', { responseType: 'blob' });
      const url = URL.createObjectURL(res.data);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'conversations.jsonl';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      message.error('导出失败');
    }
  };

  const tabItems = [
    {
      key: 'usage',
      label: '用量趋势',
      children: (
        <div>
          <Space style={{ marginBottom: 16 }}>
            <DatePicker.RangePicker
              value={range}
              allowClear={false}
              onChange={(v) => v && setRange(v as [Dayjs, Dayjs])}
            />
            <Space split>
              <Typography.Text>总 Tokens：<b>{formatNumber(summary.total_tokens)}</b></Typography.Text>
              <Typography.Text>总费用：<b className="money">{formatMoney(summary.total_cost)}</b></Typography.Text>
            </Space>
          </Space>
          <Card size="small" title="每日 Tokens 用量">
            <MiniBar data={daily} />
          </Card>
        </div>
      ),
    },
    {
      key: 'records',
      label: `计费明细`,
      children: (
        <Table
          rowKey="id"
          loading={recordLoading}
          columns={recordColumns}
          dataSource={records}
          scroll={{ x: 1100 }}
          pagination={{ current: page, total: recordTotal, pageSize: size, onChange: (p) => { setPage(p); loadRecords(p); } }}
        />
      ),
    },
    {
      key: 'transactions',
      label: '交易流水',
      children: (
        <Table
          rowKey="id"
          loading={txLoading}
          columns={txColumns}
          dataSource={txs}
          scroll={{ x: 900 }}
          pagination={{ current: txPage, total: txTotal, pageSize: size, onChange: (p) => { setTxPage(p); loadTxs(p); } }}
        />
      ),
    },
    {
      key: 'export',
      label: '对话数据导出',
      children: (
        <Card>
          <Typography.Paragraph>
            可将你的对话记录导出为 <Typography.Text code>JSONL</Typography.Text> 格式（OpenAI fine-tune 风格），用于数据备份或模型微调。
          </Typography.Paragraph>
          <Button type="primary" icon={<DownloadOutlined />} onClick={exportJsonl}>
            导出 conversations.jsonl
          </Button>
        </Card>
      ),
    },
  ];

  return (
    <div>
      <div className="page-header">
        <h2>用量与账单</h2>
        <p>查看模型调用用量、计费明细与交易流水</p>
      </div>
      <Card>
        <Tabs items={tabItems} />
      </Card>

      <Drawer title="计费详情" open={!!detail} onClose={() => setDetail(null)} width={480}>
        {detail && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="请求 ID"><Typography.Text code style={{ fontSize: 12 }}>{detail.request_id}</Typography.Text></Descriptions.Item>
            <Descriptions.Item label="模型">{detail.model}</Descriptions.Item>
            <Descriptions.Item label="提供商">{detail.provider}</Descriptions.Item>
            <Descriptions.Item label="输入 tokens">{formatNumber(detail.tokens_in)}</Descriptions.Item>
            <Descriptions.Item label="输出 tokens">{formatNumber(detail.tokens_out)}</Descriptions.Item>
            <Descriptions.Item label="缓存 tokens">{formatNumber(detail.cached_tokens)}</Descriptions.Item>
            <Descriptions.Item label="TTFT">{formatDuration(detail.ttft_ms)}</Descriptions.Item>
            <Descriptions.Item label="总耗时">{formatDuration(detail.duration_ms)}</Descriptions.Item>
            <Descriptions.Item label="计费类型">{typeLabel[detail.billing_type] || detail.billing_type}</Descriptions.Item>
            <Descriptions.Item label="费用">{formatMoney(detail.cost)}</Descriptions.Item>
            {detail.detail && <Descriptions.Item label="计费详情"><Typography.Text style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>{detail.detail}</Typography.Text></Descriptions.Item>}
            <Descriptions.Item label="时间">{dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>
    </div>
  );
}
