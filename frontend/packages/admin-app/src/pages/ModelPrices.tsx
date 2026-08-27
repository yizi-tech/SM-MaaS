import { useEffect, useState } from 'react';
import {
  Card, Table, Tag, Button, Modal, Form, Input, InputNumber, Switch, App, Space, Popconfirm, Typography, Alert, Select, AutoComplete,
} from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, ImportOutlined } from '@ant-design/icons';
import type { ModelPrice, LLMChannel } from '@mass/shared';
import { request } from '../api/request';

export default function ModelPrices() {
  const { message } = App.useApp();
  const [items, setItems] = useState<ModelPrice[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<ModelPrice | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();
  const [syncOpen, setSyncOpen] = useState(false);
  const [channels, setChannels] = useState<LLMChannel[]>([]);
  const [channelModels, setChannelModels] = useState<string[]>([]);
  const [syncing, setSyncing] = useState(false);
  const [syncForm] = Form.useForm();

  const loadChannelModels = () => {
    request
      .get('/admin/channels')
      .then((r) => {
        const list: LLMChannel[] = r.data.data || [];
        setChannels(list);
        const set = new Set<string>();
        list.forEach((c) => (c.models || []).forEach((m) => m && set.add(m)));
        setChannelModels(Array.from(set).sort());
      })
      .catch(() => {});
  };

  const openSync = () => {
    loadChannelModels();
    setSyncOpen(true);
  };

  const onSync = async () => {
    const v = await syncForm.validateFields();
    setSyncing(true);
    try {
      const data = await request
        .post('/admin/model-prices/sync', {
          channel_id: v.channel_id,
          default_input: String(v.default_input ?? 0),
          default_output: String(v.default_output ?? 0),
          enabled: !!v.enabled,
        })
        .then((r) => r.data.data);
      message.success(`同步完成：新增 ${data.created} 条，跳过 ${data.skipped} 条（共 ${data.total}）`);
      setSyncOpen(false);
      load();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSyncing(false);
    }
  };

  const load = () => {
    setLoading(true);
    request
      .get('/admin/model-prices')
      .then((r) => setItems(r.data.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
    loadChannelModels();
  }, []);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setOpen(true);
  };

  const openEdit = (p: ModelPrice) => {
    setEditing(p);
    form.setFieldsValue({
      model: p.model,
      input_price: p.input_price,
      output_price: p.output_price,
      cache_read_price: p.cache_read_price,
      cache_write_price: p.cache_write_price,
      enabled: p.enabled,
      remark: p.remark,
      support_unlimited: p.support_unlimited,
    });
    setOpen(true);
  };

  // Toggle the unlimited-firepower switch for a model price.
  const onToggleUnlimited = async (p: ModelPrice, enabled: boolean) => {
    try {
      await request.post(`/admin/model-prices/${p.id}/unlimited`, { enabled });
      message.success(enabled ? '已开启无限火力' : '已关闭无限火力');
      load();
    } catch {
      /* 拦截器已提示 */
    }
  };

  const onSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await request.put(`/admin/model-prices/${editing.id}`, values);
        message.success('已更新');
      } else {
        await request.post('/admin/model-prices', values);
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
    await request.delete(`/admin/model-prices/${id}`);
    message.success('已删除');
    load();
  };

  const columns = [
    { title: '模型', dataIndex: 'model', render: (m: string) => <Typography.Text code>{m}</Typography.Text> },
    { title: '输入价（¥/1M）', dataIndex: 'input_price', render: (v: string) => <b className="money">¥{v}</b> },
    { title: '输出价（¥/1M）', dataIndex: 'output_price', render: (v: string) => <b className="money">¥{v}</b> },
    {
      title: '缓存读 / 写',
      render: (_: unknown, r: ModelPrice) => (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {r.cache_read_price ? `¥${r.cache_read_price}` : '默认(输入×10%)'}
          {' / '}
          {r.cache_write_price ? `¥${r.cache_write_price}` : '默认(输入×125%)'}
        </Typography.Text>
      ),
    },
    { title: '状态', dataIndex: 'enabled', render: (e: boolean) => <Tag color={e ? 'success' : 'default'}>{e ? '启用' : '停用'}</Tag> },
    {
      title: '无限火力',
      render: (_: unknown, r: ModelPrice) =>
        r.support_unlimited ? (
          <Switch
            checked={r.unlimited_enabled}
            checkedChildren="开"
            unCheckedChildren="关"
            onChange={(v) => onToggleUnlimited(r, v)}
          />
        ) : (
          <Typography.Text type="secondary">未支持</Typography.Text>
        ),
    },
    { title: '备注', dataIndex: 'remark', render: (r?: string) => r || '-' },
    {
      title: '操作',
      render: (_: unknown, r: ModelPrice) => (
        <Space>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>编辑</Button>
          <Popconfirm title="删除该模型价格？" onConfirm={() => onDelete(r.id)}>
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
          <h2>模型价格表</h2>
          <p>唯一价格源：未配置价格的模型不可被调用</p>
        </div>
        <Space>
          <Button icon={<ImportOutlined />} onClick={openSync}>
            从渠道同步
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增模型价格
          </Button>
        </Space>
      </div>

      <Alert
        style={{ marginBottom: 16 }}
        type="warning"
        showIcon
        message="价格单位为 ¥ / 每百万 tokens。缓存读留空默认输入价×10%，缓存写留空默认输入价×125%。"
      />

      <Card>
        <Table rowKey="id" loading={loading} columns={columns} dataSource={items} pagination={false} scroll={{ x: 1000 }} />
      </Card>

      <Modal
        title={editing ? '编辑模型价格' : '新增模型价格'}
        open={open}
        onOk={onSave}
        confirmLoading={saving}
        onCancel={() => setOpen(false)}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="model" label="模型 ID" rules={[{ required: true, message: '请选择或输入模型 ID' }]}>
            <AutoComplete
              placeholder="选择已拉取的模型或手动输入，如 gpt-4o"
              options={channelModels.map((m) => ({ value: m, label: m }))}
              showSearch
              filterOption={(input, opt) =>
                (opt?.label as string)?.toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Space size="large" style={{ display: 'flex' }}>
            <Form.Item name="input_price" label="输入价（¥/1M）" rules={[{ required: true, message: '必填' }]}>
              <InputNumber min={0} style={{ width: '100%' }} placeholder="0.00" />
            </Form.Item>
            <Form.Item name="output_price" label="输出价（¥/1M）" rules={[{ required: true, message: '必填' }]}>
              <InputNumber min={0} style={{ width: '100%' }} placeholder="0.00" />
            </Form.Item>
          </Space>
          <Space size="large" style={{ display: 'flex' }}>
            <Form.Item name="cache_read_price" label="缓存读价（¥/1M，选填）">
              <InputNumber min={0} style={{ width: '100%' }} placeholder="默认=输入×10%" />
            </Form.Item>
            <Form.Item name="cache_write_price" label="缓存写价（¥/1M，选填）">
              <InputNumber min={0} style={{ width: '100%' }} placeholder="默认=输入×125%" />
            </Form.Item>
          </Space>
          <Form.Item name="enabled" label="启用" valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
          <Form.Item
            name="support_unlimited"
            label="支持无限火力活动"
            valuePropName="checked"
            initialValue={false}
            tooltip="开启后，管理员可在列表中动态拨动「无限火力」开关；付费订阅用户调用该模型时将免 Token 扣费"
          >
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input maxLength={200} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="从渠道同步模型"
        open={syncOpen}
        onOk={onSync}
        confirmLoading={syncing}
        onCancel={() => setSyncOpen(false)}
        okText="同步"
        destroyOnClose
      >
        <Form form={syncForm} layout="vertical">
          <Form.Item
            name="channel_id"
            label="选择渠道"
            rules={[{ required: true, message: '请选择渠道' }]}
          >
            <Select
              placeholder="渠道需已拉取模型列表"
              options={channels.map((c) => ({ label: `${c.name}（${c.models?.length || 0} 个模型）`, value: c.id }))}
            />
          </Form.Item>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="将把该渠道已拉取的模型导入价格表，已存在的模型自动跳过（价格默认 ¥0，可后续编辑）。"
          />
          <Space size="large" style={{ display: 'flex' }}>
            <Form.Item name="default_input" label="默认输入价（¥/1M）" initialValue={0}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="default_output" label="默认输出价（¥/1M）" initialValue={0}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
          </Space>
          <Form.Item
            name="enabled"
            label="创建后启用"
            valuePropName="checked"
            initialValue={false}
            tooltip="关闭则同步进价格表但保持停用，不会立即进入模型广场（避免 ¥0 污染）"
          >
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
