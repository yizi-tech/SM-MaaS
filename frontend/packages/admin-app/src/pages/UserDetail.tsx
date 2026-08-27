import { useEffect, useState } from 'react';
import {
  Card, Descriptions, Tag, Button, Space, Typography, Table, Modal, Form, Input, Select, InputNumber, App, Spin, Avatar, Popconfirm,
} from 'antd';
import { ArrowLeftOutlined, UserOutlined, EditOutlined, IdcardOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import dayjs from 'dayjs';
import type { UserDetail, Subscription, ApiKey } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, tagColor, typeLabel } from '@mass/shared';

export default function UserDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { message } = App.useApp();
  const [user, setUser] = useState<UserDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [editOpen, setEditOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = () => {
    setLoading(true);
    request
      .get(`/admin/users/${id}`)
      .then((r) => setUser(r.data.data))
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(load, [id]);

  const openEdit = () => {
    if (!user) return;
    form.setFieldsValue({
      nickname: user.nickname,
      phone: user.phone,
      qq: user.qq,
      role: user.role,
      status: user.status,
      real_name_status: user.real_name_status,
      balance: user.balance,
    });
    setEditOpen(true);
  };

  const onSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await request.put(`/admin/users/${id}`, {
        nickname: values.nickname,
        phone: values.phone,
        qq: values.qq,
        role: values.role,
        status: values.status,
        real_name_status: values.real_name_status,
        balance_adjust: values.balance_adjust || '',
        balance_note: values.balance_note || '',
      });
      message.success('已更新');
      setEditOpen(false);
      load();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSaving(false);
    }
  };

  const subColumns = [
    { title: '套餐', dataIndex: 'plan_name', render: (n: string) => <Typography.Text strong>{n}</Typography.Text> },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '生效期', render: (_: unknown, r: Subscription) => `${dayjs(r.start_at).format('YYYY-MM-DD')} ~ ${dayjs(r.end_at).format('YYYY-MM-DD')}` },
    { title: '额度', render: (_: unknown, r: Subscription) => `${r.used_tokens.toLocaleString()} / ${r.included_tokens.toLocaleString()}` },
    { title: '自动续费', dataIndex: 'auto_renew', render: (a: boolean) => (a ? '是' : '否') },
  ];

  const keyColumns = [
    { title: '名称', dataIndex: 'name' },
    { title: 'Key 前缀', dataIndex: 'key_prefix', render: (p: string) => <Typography.Text code>sk-{p}••••</Typography.Text> },
    { title: '模型访问', dataIndex: 'model_access', render: (ma: string[]) => (ma?.length ? ma.join(', ') : '全部') },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '创建时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
  ];

  if (loading || !user) return <Spin size="large" style={{ display: 'block', margin: '80px auto' }} />;

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <Button type="link" icon={<ArrowLeftOutlined />} style={{ padding: 0, marginBottom: 8 }} onClick={() => navigate('/users')}>
            返回用户列表
          </Button>
          <h2 style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Avatar size={40} src={user.avatar} icon={<UserOutlined />} />
            {user.nickname}
          </h2>
          <p>{user.email}</p>
        </div>
        <Button type="primary" icon={<EditOutlined />} onClick={openEdit}>
          编辑用户
        </Button>
      </div>

      <Card title="基本信息" style={{ marginBottom: 16 }}>
        <Descriptions column={3} size="small" bordered>
          <Descriptions.Item label="ID">{user.id}</Descriptions.Item>
          <Descriptions.Item label="邮箱">{user.email}</Descriptions.Item>
          <Descriptions.Item label="角色">
            <Tag color={user.role === 'admin' ? 'gold' : 'default'}>{user.role === 'admin' ? '管理员' : '用户'}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={tagColor(user.status)}>{typeLabel[user.status] || user.status}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="实名状态">
            <Tag color={tagColor(user.real_name_status)}>{typeLabel[user.real_name_status] || user.real_name_status}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="余额">
            <b className="money">{formatMoney(user.balance)}</b>
          </Descriptions.Item>
          <Descriptions.Item label="手机号">{user.phone || '-'}</Descriptions.Item>
          <Descriptions.Item label="最后登录 IP">{user.last_login_ip || '-'}</Descriptions.Item>
          <Descriptions.Item label="最后登录时间">
            {user.last_login_at ? dayjs(user.last_login_at).format('YYYY-MM-DD HH:mm:ss') : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="注册时间">{dayjs(user.created_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title="订阅记录" style={{ marginBottom: 16 }}>
        <Table rowKey="id" columns={subColumns} dataSource={user.subscriptions || []} pagination={false} scroll={{ x: 700 }} />
      </Card>

      <Card title="API Keys">
        <Table rowKey="id" columns={keyColumns} dataSource={user.api_keys || []} pagination={false} scroll={{ x: 700 }} />
      </Card>

      <Modal title={`编辑用户 #${user.id}`} open={editOpen} onOk={onSave} confirmLoading={saving} onCancel={() => setEditOpen(false)} width={520} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="nickname" label="昵称" rules={[{ required: true }]}>
            <Input maxLength={50} />
          </Form.Item>
          <Form.Item name="phone" label="手机号">
            <Input maxLength={11} />
          </Form.Item>
          <Form.Item name="qq" label="QQ">
            <Input maxLength={20} />
          </Form.Item>
          <Space size="large" style={{ display: 'flex' }}>
            <Form.Item name="role" label="角色" style={{ flex: 1 }}>
              <Select
                options={[
                  { label: '用户', value: 'user' },
                  { label: '管理员', value: 'admin' },
                ]}
              />
            </Form.Item>
            <Form.Item name="status" label="状态" style={{ flex: 1 }}>
              <Select
                options={[
                  { label: '正常', value: 'active' },
                  { label: '禁用', value: 'disabled' },
                  { label: '停用', value: 'suspended' },
                ]}
              />
            </Form.Item>
          </Space>
          <Form.Item name="real_name_status" label="实名状态">
            <Select
              options={[
                { label: '未认证', value: 'unverified' },
                { label: '审核中', value: 'pending' },
                { label: '已认证', value: 'verified' },
                { label: '已驳回', value: 'rejected' },
              ]}
            />
          </Form.Item>
          <Typography.Title level={5}>余额调整</Typography.Title>
          <Space size="large" style={{ display: 'flex' }}>
            <Form.Item name="balance_adjust" label="调整金额（正加负减）" style={{ flex: 1 }}>
              <InputNumber style={{ width: '100%' }} placeholder="例如 100 或 -50" />
            </Form.Item>
            <Form.Item name="balance_note" label="调整说明" style={{ flex: 2 }}>
              <Input placeholder="记录调整原因" maxLength={200} />
            </Form.Item>
          </Space>
        </Form>
      </Modal>
    </div>
  );
}
