import { useEffect, useState } from 'react';
import { Card, Table, Tag, Select, Space, Typography } from 'antd';
import dayjs from 'dayjs';
import type { Transaction } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, tagColor, typeLabel } from '@mass/shared';

export default function Orders() {
  const [items, setItems] = useState<Transaction[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [type, setType] = useState<string | undefined>();
  const [status, setStatus] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);

  const load = (p: number) => {
    setLoading(true);
    request
      .get('/admin/orders', { params: { page: p, size, type, status } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(1), []);

  const columns = [
    { title: '交易号', dataIndex: 'transaction_no', render: (t: string) => <Typography.Text code style={{ fontSize: 12 }}>{t}</Typography.Text> },
    { title: '类型', dataIndex: 'type', render: (t: string) => <Tag color={tagColor(t)}>{typeLabel[t] || t}</Tag> },
    { title: '金额', dataIndex: 'amount', render: (a: string) => <b className="money">{formatMoney(a)}</b> },
    { title: '余额变化', render: (_: unknown, r: Transaction) => <Typography.Text type="secondary" style={{ fontSize: 12 }}>{formatMoney(r.balance_before)} → {formatMoney(r.balance_after)}</Typography.Text> },
    { title: '支付方式', dataIndex: 'payment_method', render: (m: string) => (m === 'epay' ? '易支付' : m === 'balance' ? '余额' : m || '-') },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '说明', dataIndex: 'description', ellipsis: true },
    { title: '时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm:ss') },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>订单管理</h2>
          <p>平台全部交易流水</p>
        </div>
        <Space>
          <Select
            placeholder="类型"
            allowClear
            style={{ width: 130 }}
            value={type}
            onChange={(v) => { setType(v); setPage(1); load(1); }}
            options={['recharge', 'consume', 'refund', 'subscription', 'token_package', 'adjust'].map((t) => ({ label: typeLabel[t] || t, value: t }))}
          />
          <Select
            placeholder="状态"
            allowClear
            style={{ width: 130 }}
            value={status}
            onChange={(v) => { setStatus(v); setPage(1); load(1); }}
            options={['pending', 'success', 'failed', 'refunded', 'cancelled'].map((s) => ({ label: typeLabel[s] || s, value: s }))}
          />
        </Space>
      </div>
      <Card>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 1100 }}
          pagination={{
            current: page,
            total,
            pageSize: size,
            onChange: (p) => {
              setPage(p);
              load(p);
            },
          }}
        />
      </Card>
    </div>
  );
}
