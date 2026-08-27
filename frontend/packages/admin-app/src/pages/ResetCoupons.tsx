import { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, Modal, Form, Input, InputNumber, Typography, App } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { request } from '../api/request';
import { formatNumber, tagColor, typeLabel } from '@mass/shared';

interface ResetCouponAdmin {
  id: number;
  code: string;
  user_id: number;
  user_email?: string;
  status: string;
  note?: string;
  issued_by?: number;
  used_at?: string;
  created_at: string;
}

export default function ResetCoupons() {
  const { message } = App.useApp();
  const [items, setItems] = useState<ResetCouponAdmin[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm();

  const load = (p: number) => {
    setLoading(true);
    request
      .get('/admin/reset-coupons', { params: { page: p, size } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(1), []);

  const submit = async () => {
    const values = await form.validateFields();
    setSubmitting(true);
    try {
      const res = await request.post('/admin/reset-coupons', {
        user_id: values.user_id ?? 0,
        count: values.count ?? 1,
        note: values.note ?? '',
      });
      message.success(`已生成 ${res.data.data?.issued || 0} 张重置券`);
      setOpen(false);
      form.resetFields();
      load(page);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSubmitting(false);
    }
  };

  const columns = [
    { title: '券码', dataIndex: 'code', render: (c: string) => <Typography.Text code style={{ fontSize: 12 }}>{c}</Typography.Text> },
    {
      title: '归属用户',
      render: (_: unknown, r: ResetCouponAdmin) => (
        <div>
          <Typography.Text strong>#{r.user_id}</Typography.Text>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{r.user_email || '-'}</Typography.Text>
          </div>
        </div>
      ),
    },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '备注', dataIndex: 'note', ellipsis: true },
    { title: '使用时间', dataIndex: 'used_at', render: (t?: string) => (t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '-') },
    { title: '发放时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>重置券发放</h2>
          <p>为用户发放用量重置券（用户可兑换重置订阅用量）</p>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          发放重置券
        </Button>
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
            onChange: (p) => { setPage(p); load(p); },
          }}
        />
      </Card>

      <Modal
        title="发放重置券"
        open={open}
        onCancel={() => setOpen(false)}
        onOk={submit}
        okText="生成"
        confirmLoading={submitting}
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ user_id: 0, count: 1 }}>
          <Form.Item name="user_id" label="发放对象" extra="留空表示发放给全部活跃用户">
            <InputNumber style={{ width: '100%' }} min={0} placeholder="用户 ID（0=全部）" addonAfter="0=全部用户" />
          </Form.Item>
          <Form.Item name="count" label="每人张数">
            <InputNumber style={{ width: '100%' }} min={1} max={10} addonAfter="1-10 张" />
          </Form.Item>
          <Form.Item name="note" label="备注">
            <Input.TextArea rows={2} placeholder="选填" />
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ fontSize: 12, margin: 0 }}>
            生成的券码形如 <Typography.Text code>RC+时间戳+随机串</Typography.Text>，用户可在「重置券」页面兑换。
          </Typography.Paragraph>
        </Form>
      </Modal>
    </div>
  );
}
