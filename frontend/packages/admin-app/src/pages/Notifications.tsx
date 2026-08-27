import { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, Modal, Form, Input, InputNumber, Select, Typography, App } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { request } from '../api/request';
import { tagColor, typeLabel } from '@mass/shared';

interface NotificationAdmin {
  id: number;
  user_id: number;
  user_email?: string;
  title: string;
  content: string;
  type: string;
  is_read: boolean;
  created_at: string;
}

export default function Notifications() {
  const { message } = App.useApp();
  const [items, setItems] = useState<NotificationAdmin[]>([]);
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
      .get('/admin/notifications', { params: { page: p, size } })
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
      const res = await request.post('/admin/notifications', {
        user_id: values.user_id ?? 0,
        title: values.title,
        content: values.content,
        type: values.type ?? 'system',
      });
      message.success(`已发送 ${res.data.data?.issued || 0} 条通知（${res.data.data?.target === 'all' ? '全员' : '指定用户'}）`);
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
    { title: '标题', dataIndex: 'title', render: (t: string, r: NotificationAdmin) => (
      <div>
        <Typography.Text strong={!r.is_read}>{t}</Typography.Text>
        {!r.is_read && <Tag color="red" style={{ marginLeft: 8 }}>未读</Tag>}
      </div>
    ) },
    { title: '类型', dataIndex: 'type', render: (t: string) => <Tag color={tagColor(t)}>{typeLabel[t] || t}</Tag> },
    {
      title: '接收用户',
      render: (_: unknown, r: NotificationAdmin) => (
        <div>
          <Typography.Text strong>#{r.user_id}</Typography.Text>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{r.user_email || '-'}</Typography.Text>
          </div>
        </div>
      ),
    },
    { title: '内容', dataIndex: 'content', ellipsis: true },
    { title: '发送时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>通知管理</h2>
          <p>向指定用户或全部用户发送站内通知</p>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          发送通知
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
        title="发送通知"
        open={open}
        onCancel={() => setOpen(false)}
        onOk={submit}
        okText="发送"
        confirmLoading={submitting}
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ user_id: 0, type: 'system' }}>
          <Form.Item name="user_id" label="接收用户" extra="留空表示发送给全部活跃用户">
            <InputNumber style={{ width: '100%' }} min={0} placeholder="用户 ID（0=全部）" addonAfter="0=全部用户" />
          </Form.Item>
          <Form.Item name="type" label="通知类型">
            <Select
              options={[
                { label: '系统通知', value: 'system' },
                { label: '订单通知', value: 'order' },
                { label: '额度通知', value: 'credit' },
                { label: '安全通知', value: 'security' },
              ]}
            />
          </Form.Item>
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input placeholder="例如：系统维护公告" maxLength={100} />
          </Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true, message: '请输入内容' }]}>
            <Input.TextArea rows={4} placeholder="请输入通知内容" maxLength={2000} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
