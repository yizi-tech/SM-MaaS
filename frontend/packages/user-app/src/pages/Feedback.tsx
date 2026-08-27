import { useEffect, useState } from 'react';
import { Card, Form, Input, Radio, Button, App, Table, Tag, Typography, Tabs, Empty } from 'antd';
import type { Feedback } from '@mass/shared';
import { request } from '../api/request';
import { tagColor, typeLabel } from '@mass/shared';

export default function Feedback() {
  const { message } = App.useApp();
  const [items, setItems] = useState<Feedback[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm();

  const load = (p: number) => {
    setLoading(true);
    request
      .get('/user/feedback', { params: { page: p, size } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(1), []);

  const onSubmit = async (values: { type: string; title: string; content: string; contact?: string }) => {
    setSubmitting(true);
    try {
      await request.post('/user/feedback', values);
      message.success('反馈已提交，感谢你的建议');
      form.resetFields();
      load(1);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSubmitting(false);
    }
  };

  const columns = [
    { title: '类型', dataIndex: 'type', render: (t: string) => <Tag color={tagColor(t)}>{typeLabel[t] || t}</Tag> },
    { title: '标题', dataIndex: 'title', render: (t: string) => <Typography.Text strong>{t}</Typography.Text> },
    { title: '内容', dataIndex: 'content', ellipsis: true },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '处理备注', dataIndex: 'admin_note', render: (n?: string) => n || '-' },
    { title: '提交时间', dataIndex: 'created_at', render: (t: string) => new Date(t).toLocaleString('zh-CN') },
  ];

  const tabItems = [
    {
      key: 'submit',
      label: '提交反馈',
      children: (
        <div style={{ maxWidth: 560 }}>
          <Form form={form} layout="vertical" onFinish={onSubmit}>
            <Form.Item name="type" label="反馈类型" initialValue="suggestion" rules={[{ required: true }]}>
              <Radio.Group>
                <Radio.Button value="bug">问题反馈</Radio.Button>
                <Radio.Button value="suggestion">功能建议</Radio.Button>
                <Radio.Button value="other">其他</Radio.Button>
              </Radio.Group>
            </Form.Item>
            <Form.Item name="title" label="标题" rules={[{ required: true, max: 200, message: '请输入标题' }]}>
              <Input placeholder="简要描述问题或建议" maxLength={200} />
            </Form.Item>
            <Form.Item name="content" label="详细描述" rules={[{ required: true, max: 10000, message: '请输入详细描述' }]}>
              <Input.TextArea rows={5} placeholder="请描述遇到的问题 / 复现步骤 / 期望功能" maxLength={10000} showCount />
            </Form.Item>
            <Form.Item name="contact" label="联系方式（选填）">
              <Input placeholder="邮箱 / QQ / 微信" maxLength={100} />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={submitting}>
              提交反馈
            </Button>
          </Form>
        </div>
      ),
    },
    {
      key: 'history',
      label: '我的反馈',
      children: (
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          locale={{ emptyText: <Empty description="暂无反馈记录" /> }}
          scroll={{ x: 800 }}
          pagination={{ current: page, total, pageSize: size, onChange: (p) => { setPage(p); load(p); } }}
        />
      ),
    },
  ];

  return (
    <div>
      <div className="page-header">
        <h2>问题反馈</h2>
        <p>遇到问题或有好的想法？告诉我们</p>
      </div>
      <Card>
        <Tabs items={tabItems} />
      </Card>
    </div>
  );
}
