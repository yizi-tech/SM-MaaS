import { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, Modal, Descriptions, Typography, Select, Space, App, Input, Radio } from 'antd';
import { EyeOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { Feedback } from '@mass/shared';
import { request } from '../api/request';
import { tagColor, typeLabel } from '@mass/shared';

const statusOptions = [
  { label: '待处理', value: 'pending' },
  { label: '处理中', value: 'processing' },
  { label: '已解决', value: 'resolved' },
  { label: '已关闭', value: 'closed' },
];

export default function Feedback() {
  const { message } = App.useApp();
  const [items, setItems] = useState<Feedback[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [status, setStatus] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState<Feedback | null>(null);
  const [newStatus, setNewStatus] = useState<string>('processing');
  const [adminNote, setAdminNote] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const load = (p: number, s?: string) => {
    setLoading(true);
    request
      .get('/admin/feedback', { params: { page: p, size, status: s } })
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
    if (!adminNote.trim() && newStatus === detail.status) {
      message.warning('请修改状态或填写处理备注');
      return;
    }
    setSubmitting(true);
    try {
      await request.put(`/admin/feedback/${detail.id}/status`, {
        status: newStatus,
        admin_note: adminNote.trim(),
      });
      message.success('已更新');
      setDetail(null);
      setAdminNote('');
      load(page, status);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSubmitting(false);
    }
  };

  const columns = [
    {
      title: '类型',
      dataIndex: 'type',
      render: (t: string) => <Tag color={tagColor(t)}>{typeLabel[t] || t}</Tag>,
    },
    { title: '标题', dataIndex: 'title', ellipsis: true },
    { title: '内容', dataIndex: 'content', ellipsis: true },
    {
      title: '状态',
      dataIndex: 'status',
      render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag>,
    },
    { title: '提交时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
    {
      title: '操作',
      render: (_: unknown, r: Feedback) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => { setDetail(r); setNewStatus(r.status); setAdminNote(''); }}>
          处理
        </Button>
      ),
    },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>反馈处理</h2>
          <p>处理用户提交的程序问题反馈与建议</p>
        </div>
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 140 }}
          value={status}
          onChange={(v) => { setStatus(v); setPage(1); load(1, v); }}
          options={statusOptions}
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
        title="反馈详情"
        open={!!detail}
        onCancel={() => { setDetail(null); setAdminNote(''); }}
        width={560}
        footer={[
          <Button key="close" onClick={() => { setDetail(null); setAdminNote(''); }}>关闭</Button>,
          <Button key="save" type="primary" loading={submitting} onClick={submit}>保存处理结果</Button>,
        ]}
      >
        {detail && (
          <div>
            <Descriptions column={1} size="small" bordered style={{ marginBottom: 16 }}>
              <Descriptions.Item label="类型">{typeLabel[detail.type] || detail.type}</Descriptions.Item>
              <Descriptions.Item label="标题">{detail.title}</Descriptions.Item>
              <Descriptions.Item label="内容">
                <Typography.Paragraph style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{detail.content}</Typography.Paragraph>
              </Descriptions.Item>
              {detail.contact && <Descriptions.Item label="联系方式">{detail.contact}</Descriptions.Item>}
              <Descriptions.Item label="提交时间">{dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
              {detail.resolved_at && (
                <Descriptions.Item label="解决时间">{dayjs(detail.resolved_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
              )}
              {detail.admin_note && (
                <Descriptions.Item label="处理备注">
                  <Typography.Paragraph style={{ margin: 0, whiteSpace: 'pre-wrap' }} type="success">{detail.admin_note}</Typography.Paragraph>
                </Descriptions.Item>
              )}
            </Descriptions>

            <Typography.Title level={5}>更新处理状态</Typography.Title>
            <Space direction="vertical" style={{ width: '100%' }} size={12}>
              <Radio.Group
                optionType="button"
                buttonStyle="solid"
                value={newStatus}
                onChange={(e) => setNewStatus(e.target.value)}
                options={statusOptions}
              />
              <Input.TextArea
                rows={3}
                placeholder="填写处理备注（可选，会展示给用户）"
                value={adminNote}
                onChange={(e) => setAdminNote(e.target.value)}
                maxLength={2000}
              />
            </Space>
          </div>
        )}
      </Modal>
    </div>
  );
}
