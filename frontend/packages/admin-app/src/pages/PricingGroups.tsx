import { useEffect, useState } from 'react';
import {
  Card, Table, Tag, Button, Modal, Form, Input, Switch, InputNumber, App, Space, Popconfirm, Typography, Select,
} from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons';
import type { PricingGroup } from '@mass/shared';
import { request } from '../api/request';

export default function PricingGroups() {
  const { message } = App.useApp();
  const [items, setItems] = useState<PricingGroup[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<PricingGroup | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = () => {
    setLoading(true);
    request
      .get('/admin/pricing-groups')
      .then((r) => setItems(r.data.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setOpen(true);
  };

  const openEdit = (g: PricingGroup) => {
    setEditing(g);
    form.setFieldsValue({
      name: g.name,
      multiplier: g.multiplier,
      models: g.models || [],
      enabled: g.enabled,
      remark: g.remark,
    });
    setOpen(true);
  };

  const onSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await request.put(`/admin/pricing-groups/${editing.id}`, values);
        message.success('已更新');
      } else {
        await request.post('/admin/pricing-groups', values);
        message.success('已创建');
      }
      setOpen(false);
      load();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSaving(false);
    }
  };

  const onDelete = async (id: number) => {
    await request.delete(`/admin/pricing-groups/${id}`);
    message.success('已删除');
    load();
  };

  const columns = [
    { title: '名称', dataIndex: 'name', render: (n: string) => <Typography.Text strong>{n}</Typography.Text> },
    {
      title: '倍率',
      dataIndex: 'multiplier',
      render: (m: string) => <Tag color={parseFloat(m) > 1 ? 'red' : 'green'}>{m}×</Tag>,
    },
    { title: '模型', dataIndex: 'models', render: (m: string[]) => (m?.length ? m.join(', ') : '全部') },
    { title: '状态', dataIndex: 'enabled', render: (e: boolean) => <Tag color={e ? 'success' : 'default'}>{e ? '启用' : '停用'}</Tag> },
    { title: '备注', dataIndex: 'remark', render: (r?: string) => r || '-' },
    {
      title: '操作',
      render: (_: unknown, r: PricingGroup) => (
        <Space>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>编辑</Button>
          <Popconfirm title="删除该定价分组？" onConfirm={() => onDelete(r.id)}>
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>定价分组</h2>
          <p>对模型价格表做倍率加成（按量计费 = 价格 × 倍率；订阅扣减 = tokens × 倍率）</p>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
          新增分组
        </Button>
      </div>
      <Card>
        <Table rowKey="id" loading={loading} columns={columns} dataSource={items} pagination={false} scroll={{ x: 800 }} />
      </Card>

      <Modal
        title={editing ? '编辑定价分组' : '新增定价分组'}
        open={open}
        onOk={onSave}
        confirmLoading={saving}
        onCancel={() => setOpen(false)}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="分组名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input maxLength={50} placeholder="例如：热门模型 / 推理模型" />
          </Form.Item>
          <Form.Item name="multiplier" label="倍率" rules={[{ required: true, message: '请输入倍率' }]}>
            <InputNumber min={0.0001} max={1000} step={0.1} style={{ width: '100%' }} addonAfter="×" />
          </Form.Item>
          <Form.Item name="models" label="适用模型（空 = 全部）">
            <Select mode="tags" placeholder="模型名，支持通配符" tokenSeparators={[',', '，']} />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input maxLength={200} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
