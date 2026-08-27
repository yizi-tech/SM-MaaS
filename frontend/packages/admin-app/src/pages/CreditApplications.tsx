import { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, Modal, InputNumber, Descriptions, Typography, Select, Space, App, Input } from 'antd';
import { CheckOutlined, CloseOutlined, EyeOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { request } from '../api/request';
import { formatNumber, tagColor, typeLabel } from '@mass/shared';

interface CreditApplication {
  id: number;
  user_id: number;
  user_email?: string;
  status: string;
  granted_tokens: number;
  reject_reason?: string;
  consumed_total: string;
  credit_limit: number;
  credit_used: number;
  credit_available: number;
  created_at: string;
  reviewed_at?: string;
}

export default function CreditApplications() {
  const { message } = App.useApp();
  const [items, setItems] = useState<CreditApplication[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [status, setStatus] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState<CreditApplication | null>(null);
  const [action, setAction] = useState<'approve' | 'reject' | null>(null);
  const [grantedTokens, setGrantedTokens] = useState<number>(10000);
  const [reason, setReason] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = (p: number, s?: string) => {
    setLoading(true);
    request
      .get('/admin/credit-applications', { params: { page: p, size, status: s } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(1), []);

  const submit = async () => {
    if (!detail) return;
    setSubmitting(true);
    try {
      if (action === 'approve') {
        if (!grantedTokens || grantedTokens <= 0) {
          message.warning('授信额度必须大于 0');
          return;
        }
        await request.post(`/admin/credit-applications/${detail.id}/approve`, { granted_tokens: grantedTokens });
        message.success('已通过并授予额度');
      } else if (action === 'reject') {
        if (!reason.trim()) {
          message.warning('请输入驳回原因');
          return;
        }
        await request.post(`/admin/credit-applications/${detail.id}/reject`, { reason: reason.trim() });
        message.success('已驳回');
      }
      setAction(null);
      setDetail(null);
      load(page, status);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSubmitting(false);
    }
  };

  const columns = [
    {
      title: '用户',
      render: (_: unknown, r: CreditApplication) => (
        <div>
          <Typography.Text strong>#{r.user_id}</Typography.Text>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{r.user_email || '-'}</Typography.Text>
          </div>
        </div>
      ),
    },
    {
      title: '已授额度',
      dataIndex: 'granted_tokens',
      render: (v: number) => <b>{formatNumber(v)} Tokens</b>,
    },
    {
      title: '累计消费',
      dataIndex: 'consumed_total',
      render: (v: string) => `${formatNumber(Number(v))} Tokens`,
    },
    {
      title: '信用状态',
      render: (_: unknown, r: CreditApplication) => (
        <Space direction="vertical" size={0}>
          <Typography.Text style={{ fontSize: 12 }}>额度 {formatNumber(r.credit_limit)} · 已用 {formatNumber(r.credit_used)}</Typography.Text>
          <Typography.Text style={{ fontSize: 12 }}>可用 {formatNumber(r.credit_available)}</Typography.Text>
        </Space>
      ),
    },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '申请时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
    {
      title: '操作',
      render: (_: unknown, r: CreditApplication) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setDetail(r)}>
          详情
        </Button>
      ),
    },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>授信申请</h2>
          <p>审核用户的 Token 授信申请</p>
        </div>
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 140 }}
          value={status}
          onChange={(v) => { setStatus(v); setPage(1); load(1, v); }}
          options={[
            { label: '待审核', value: 'pending' },
            { label: '已通过', value: 'approved' },
            { label: '已驳回', value: 'rejected' },
          ]}
        />
      </div>
      <Card>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 1000 }}
          pagination={{
            current: page,
            total,
            pageSize: size,
            onChange: (p) => { setPage(p); load(p, status); },
          }}
        />
      </Card>

      <Modal
        title="授信申请详情"
        open={!!detail}
        onCancel={() => { setDetail(null); setAction(null); setReason(''); }}
        width={560}
        footer={
          detail?.status === 'pending'
            ? [
                <Button key="reject" danger icon={<CloseOutlined />} onClick={() => setAction('reject')}>
                  驳回
                </Button>,
                <Button key="approve" type="primary" icon={<CheckOutlined />} onClick={() => setAction('approve')}>
                  通过
                </Button>,
              ]
            : [<Button key="close" onClick={() => { setDetail(null); setAction(null); setReason(''); }}>关闭</Button>]
        }
      >
        {detail && (
          <div>
            <Descriptions column={1} size="small" bordered style={{ marginBottom: 16 }}>
              <Descriptions.Item label="用户">#{detail.user_id}（{detail.user_email || '-'}）</Descriptions.Item>
              <Descriptions.Item label="信用额度">{formatNumber(detail.credit_limit)} Tokens</Descriptions.Item>
              <Descriptions.Item label="已用">{formatNumber(detail.credit_used)} Tokens</Descriptions.Item>
              <Descriptions.Item label="可用">{formatNumber(detail.credit_available)} Tokens</Descriptions.Item>
              <Descriptions.Item label="累计消费">{formatNumber(Number(detail.consumed_total))} Tokens</Descriptions.Item>
              <Descriptions.Item label="申请时间">{dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
              {detail.reviewed_at && <Descriptions.Item label="审核时间">{dayjs(detail.reviewed_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>}
              {detail.reject_reason && (
                <Descriptions.Item label="驳回原因"><Typography.Text type="danger">{detail.reject_reason}</Typography.Text></Descriptions.Item>
              )}
            </Descriptions>

            {action === 'approve' && (
              <div style={{ marginBottom: 8 }}>
                <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 8 }}>
                  授予该用户新的授信 Token 额度（将会覆盖其现有授信额度）
                </Typography.Paragraph>
                <Space.Compact style={{ width: '100%' }}>
                  <InputNumber
                    style={{ width: '100%' }}
                    min={1}
                    step={1000}
                    value={grantedTokens}
                    onChange={(v) => setGrantedTokens(v ?? 0)}
                    addonAfter="Tokens"
                  />
                  <Button type="primary" loading={submitting} onClick={submit}>
                    确认通过
                  </Button>
                </Space.Compact>
              </div>
            )}
            {action === 'reject' && (
              <Space.Compact style={{ width: '100%' }}>
                <Input
                  placeholder="请输入驳回原因"
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                />
                <Button danger loading={submitting} onClick={submit}>
                  确认驳回
                </Button>
              </Space.Compact>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
}
