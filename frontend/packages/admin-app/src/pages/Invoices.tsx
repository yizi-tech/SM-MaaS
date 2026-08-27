import { useEffect, useState } from 'react';
import {
  Card, Table, Tag, Button, Modal, Descriptions, App, Select, Input, Typography, Space,
} from 'antd';
import { CheckOutlined, CloseOutlined, EyeOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { Invoice, UserInfo } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, tagColor, typeLabel } from '@mass/shared';

interface InvoiceItem extends Invoice {
  user_id?: number;
  user?: UserInfo;
}

export default function Invoices() {
  const { message } = App.useApp();
  const [items, setItems] = useState<InvoiceItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [status, setStatus] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState<InvoiceItem | null>(null);
  const [action, setAction] = useState<'issue' | 'reject' | null>(null);
  const [actionValue, setActionValue] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = (p: number, s?: string) => {
    setLoading(true);
    request
      .get('/admin/invoices', { params: { page: p, size, status: s } })
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
      if (action === 'issue') {
        if (!actionValue.trim()) {
          message.warning('请输入发票号码');
          return;
        }
        await request.post(`/admin/invoices/${detail.id}/issue`, { invoice_no: actionValue.trim() });
        message.success('已开具');
      } else if (action === 'reject') {
        if (!actionValue.trim()) {
          message.warning('请输入驳回原因');
          return;
        }
        await request.post(`/admin/invoices/${detail.id}/reject`, { reason: actionValue.trim() });
        message.success('已驳回');
      }
      setAction(null);
      setActionValue('');
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
      render: (_: unknown, r: InvoiceItem) => (
        <div>
          <Typography.Text strong>{r.user?.nickname || `#${r.user_id}`}</Typography.Text>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{r.user?.email || ''}</Typography.Text>
          </div>
        </div>
      ),
    },
    { title: '金额', dataIndex: 'amount', render: (a: string) => <b className="money">{formatMoney(a)}</b> },
    { title: '抬头', dataIndex: 'title', ellipsis: true },
    { title: '类型', render: (_: unknown, r: InvoiceItem) => `${typeLabel[r.title_type] || r.title_type} · ${typeLabel[r.invoice_type] || r.invoice_type}` },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '申请时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
    {
      title: '操作',
      render: (_: unknown, r: InvoiceItem) => (
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
          <h2>发票审核</h2>
          <p>审核用户提交的发票申请</p>
        </div>
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 140 }}
          value={status}
          onChange={(v) => { setStatus(v); setPage(1); load(1, v); }}
          options={[
            { label: '待处理', value: 'pending' },
            { label: '已开具', value: 'issued' },
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
          scroll={{ x: 900 }}
          pagination={{
            current: page,
            total,
            pageSize: size,
            onChange: (p) => { setPage(p); load(p, status); },
          }}
        />
      </Card>

      <Modal
        title="发票详情"
        open={!!detail}
        onCancel={() => { setDetail(null); setAction(null); setActionValue(''); }}
        width={560}
        footer={
          detail?.status === 'pending'
            ? [
                <Button key="reject" danger icon={<CloseOutlined />} onClick={() => setAction('reject')}>
                  驳回
                </Button>,
                <Button key="issue" type="primary" icon={<CheckOutlined />} onClick={() => setAction('issue')}>
                  开具
                </Button>,
              ]
            : [<Button key="close" onClick={() => { setDetail(null); setAction(null); setActionValue(''); }}>关闭</Button>]
        }
      >
        {detail && (
          <div>
            <Descriptions column={1} size="small" bordered style={{ marginBottom: 16 }}>
              <Descriptions.Item label="用户">{detail.user?.nickname}（{detail.user?.email}）</Descriptions.Item>
              <Descriptions.Item label="金额">{formatMoney(detail.amount)}</Descriptions.Item>
              <Descriptions.Item label="抬头类型">{typeLabel[detail.title_type] || detail.title_type}</Descriptions.Item>
              <Descriptions.Item label="发票类型">{typeLabel[detail.invoice_type] || detail.invoice_type}</Descriptions.Item>
              <Descriptions.Item label="抬头">{detail.title}</Descriptions.Item>
              <Descriptions.Item label="税号">{detail.tax_no || '-'}</Descriptions.Item>
              <Descriptions.Item label="开户行">{detail.bank_name || '-'}</Descriptions.Item>
              <Descriptions.Item label="银行账号">{detail.bank_account || '-'}</Descriptions.Item>
              <Descriptions.Item label="接收邮箱">{detail.email || '-'}</Descriptions.Item>
              <Descriptions.Item label="备注">{detail.remark || '-'}</Descriptions.Item>
              <Descriptions.Item label="申请时间">{dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
              {detail.invoice_no && <Descriptions.Item label="发票号">{detail.invoice_no}</Descriptions.Item>}
              {detail.reject_reason && (
                <Descriptions.Item label="驳回原因"><Typography.Text type="danger">{detail.reject_reason}</Typography.Text></Descriptions.Item>
              )}
            </Descriptions>

            {action && (
              <Space.Compact style={{ width: '100%' }}>
                <Input
                  placeholder={action === 'issue' ? '请输入发票号码' : '请输入驳回原因'}
                  value={actionValue}
                  onChange={(e) => setActionValue(e.target.value)}
                />
                <Button type="primary" loading={submitting} onClick={submit}>
                  {action === 'issue' ? '确认开具' : '确认驳回'}
                </Button>
              </Space.Compact>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
}
