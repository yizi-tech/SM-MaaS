import { useEffect, useState } from 'react';
import {
  Card, Table, Tag, Button, Modal, Form, Input, InputNumber, App, Space, Popconfirm, Typography, Select,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PlusOutlined, DeleteOutlined, EditOutlined, AppstoreOutlined } from '@ant-design/icons';
import type { Plan } from '@mass/shared';
import { request } from '../api/request';
import ModelListModal from '../components/ModelListModal';
import { useIsMobile } from '../hooks/useIsMobile';

export default function Plans() {
  const { message } = App.useApp();
  const isMobile = useIsMobile();
  const [items, setItems] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Plan | null>(null);
  const [saving, setSaving] = useState(false);
  const [modelsView, setModelsView] = useState<Plan | null>(null);
  const [form] = Form.useForm();

  const load = () => {
    setLoading(true);
    request
      .get('/admin/plans')
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

  const openEdit = (p: Plan) => {
    setEditing(p);
    form.setFieldsValue({
      name: p.name,
      description: p.description,
      price: p.price,
      duration_days: p.duration_days,
      rpm: p.rpm,
      tpm: p.tpm,
      included_tokens: p.included_tokens,
      concurrent_limit: p.concurrent_limit,
      max_purchase: p.max_purchase || 0,
      model_access: p.model_access || [],
    });
    setOpen(true);
  };

  const onSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await request.put(`/admin/plans/${editing.id}`, values);
        message.success('已更新');
      } else {
        await request.post('/admin/plans', values);
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
    await request.delete(`/admin/plans/${id}`);
    message.success('已删除');
    load();
  };

  const columns: ColumnsType<Plan> = [
    { title: '套餐', dataIndex: 'name', render: (n: string) => <Typography.Text strong>{n}</Typography.Text> },
    { title: '描述', dataIndex: 'description', ellipsis: true, responsive: ['md'] },
    { title: '价格', dataIndex: 'price', render: (p: string) => <b className="money">¥{p}</b> },
    { title: '时长', dataIndex: 'duration_days', render: (d: number) => `${d} 天`, responsive: ['sm'] },
    { title: '额度', dataIndex: 'included_tokens', render: (t: number) => t.toLocaleString(), responsive: ['md'] },
    { title: 'RPM/TPM', render: (_: unknown, r: Plan) => `${r.rpm} / ${r.tpm.toLocaleString()}`, responsive: ['lg'] },
    { title: '并发', dataIndex: 'concurrent_limit', responsive: ['lg'] },
    { title: '限购', dataIndex: 'max_purchase', render: (n: number) => (n > 0 ? `${n} 次` : '不限'), responsive: ['lg'] },
    {
      title: '模型白名单',
      dataIndex: 'model_access',
      render: (m: string[], r: Plan) =>
        m?.length ? (
          <Button type="link" size="small" style={{ padding: 0 }} icon={<AppstoreOutlined />} onClick={() => setModelsView(r)}>
            {m.length} 个模型
          </Button>
        ) : (
          <Tag>全部</Tag>
        ),
    },
    {
      title: '操作',
      fixed: 'right' as const,
      width: 150,
      render: (_: unknown, r: Plan) => (
        <Space size={0} wrap>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>编辑</Button>
          <Popconfirm title="删除该套餐？" onConfirm={() => onDelete(r.id)}>
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>套餐管理</h2>
          <p>管理订阅套餐配置（余额支付，套餐额度随用随减）</p>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate} block={isMobile}>
          新增套餐
        </Button>
      </div>
      <Card>
        <Table rowKey="id" loading={loading} columns={columns} dataSource={items} pagination={false} scroll={{ x: 900 }} size={isMobile ? 'small' : 'middle'} />
      </Card>

      <ModelListModal
        open={!!modelsView}
        title={`模型白名单 · ${modelsView?.name ?? ''}`}
        subtitle={
          modelsView && (
            <span>
              价格 <b className="money">¥{modelsView.price}</b> / {modelsView.duration_days} 天 · 额度{' '}
              {modelsView.included_tokens.toLocaleString()} tokens
            </span>
          )
        }
        models={modelsView?.model_access}
        onClose={() => setModelsView(null)}
      />

      <Modal
        title={editing ? '编辑套餐' : '新增套餐'}
        open={open}
        onOk={onSave}
        confirmLoading={saving}
        onCancel={() => setOpen(false)}
        width={isMobile ? 'calc(100vw - 16px)' : 600}
        centered={isMobile}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="套餐名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} maxLength={500} />
          </Form.Item>
          <div className="form-row-3">
            <Form.Item name="price" label="价格（¥）" rules={[{ required: true, message: '请输入价格' }]}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="duration_days" label="时长（天）" rules={[{ required: true, message: '请输入时长' }]}>
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="included_tokens" label="额度（tokens）" initialValue={0}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
          </div>
          <div className="form-row-3">
            <Form.Item name="rpm" label="RPM" initialValue={60}>
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="tpm" label="TPM" initialValue={100000}>
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="concurrent_limit" label="并发上限" initialValue={10}>
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="max_purchase" label="限购次数（0 = 不限）" initialValue={0}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
          </div>
          <Form.Item name="model_access" label="模型白名单（空 = 全部模型）">
            <Select mode="tags" placeholder="模型名，支持通配符" tokenSeparators={[',', '，']} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
