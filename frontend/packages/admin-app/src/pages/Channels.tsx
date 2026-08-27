import { useEffect, useState } from 'react';
import {
  Card, Table, Tag, Button, Modal, Form, Input, Select, Switch, InputNumber, App, Space, Typography, Popconfirm, Alert,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PlusOutlined, DeleteOutlined, EditOutlined, AppstoreOutlined, ExperimentOutlined, CloudDownloadOutlined } from '@ant-design/icons';
import type { LLMChannel } from '@mass/shared';
import { request } from '../api/request';
import ModelListModal from '../components/ModelListModal';
import { useIsMobile } from '../hooks/useIsMobile';

export default function Channels() {
  const { message } = App.useApp();
  const isMobile = useIsMobile();
  const [items, setItems] = useState<LLMChannel[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<LLMChannel | null>(null);
  const [saving, setSaving] = useState(false);
  const [testOpen, setTestOpen] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ ok: boolean; latency_ms: number; count: number; models: string[]; error?: string } | null>(null);
  const [modelsView, setModelsView] = useState<LLMChannel | null>(null);
  const [fetching, setFetching] = useState(false);
  const [form] = Form.useForm();
  const [testForm] = Form.useForm();

  const load = () => {
    setLoading(true);
    request
      .get('/admin/channels')
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

  const openEdit = (c: LLMChannel) => {
    setEditing(c);
    form.setFieldsValue({
      name: c.name,
      type: c.type,
      base_url: c.base_url,
      api_key: c.api_key || '',
      models: c.models || [],
      priority: c.priority,
      enabled: c.enabled,
      remark: c.remark,
    });
    setOpen(true);
  };

  const onSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await request.put(`/admin/channels/${editing.id}`, values);
        message.success('渠道已更新');
      } else {
        await request.post('/admin/channels', values);
        message.success('渠道已创建');
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
    await request.delete(`/admin/channels/${id}`);
    message.success('已删除');
    load();
  };

  const onTest = async () => {
    const values = await testForm.validateFields();
    setTesting(true);
    setTestResult(null);
    try {
      const data = await request.post('/admin/channels/test', values).then((r) => r.data.data);
      setTestResult(data);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setTesting(false);
    }
  };

  // Fetch models from the upstream using the form's credentials and fill the
  // channel's model list field.
  const onFetchModels = async () => {
    const { type, base_url, api_key } = form.getFieldsValue(['type', 'base_url', 'api_key']);
    if (!base_url) {
      message.warning('请先填写 Base URL');
      return;
    }
    setFetching(true);
    try {
      const data = await request
        .post('/admin/channels/test', { type: type || 'openai', base_url, api_key: api_key || '' })
        .then((r) => r.data.data);
      if (!data.ok) {
        message.error(data.error || '获取模型失败');
        return;
      }
      form.setFieldValue('models', data.models);
      message.success(`已获取 ${data.count} 个模型并填入列表`);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setFetching(false);
    }
  };

  const columns: ColumnsType<LLMChannel> = [
    { title: '名称', dataIndex: 'name', render: (n: string) => <Typography.Text strong>{n}</Typography.Text> },
    { title: '类型', dataIndex: 'type', render: (t: string) => <Tag color={t === 'anthropic' ? 'orange' : 'blue'}>{t === 'anthropic' ? 'Anthropic' : 'OpenAI'}</Tag> },
    { title: 'Base URL', dataIndex: 'base_url', render: (u: string) => <Typography.Text code style={{ fontSize: 12 }}>{u}</Typography.Text>, ellipsis: true, responsive: ['md'] },
    { title: 'API Key', dataIndex: 'api_key', render: (k?: string) => (k ? `${k.slice(0, 6)}••••${k.slice(-4)}` : '-'), responsive: ['lg'] },
    {
      title: '模型',
      dataIndex: 'models',
      render: (m: string[], r: LLMChannel) =>
        m?.length ? (
          <Button type="link" size="small" style={{ padding: 0 }} icon={<AppstoreOutlined />} onClick={() => setModelsView(r)}>
            {m.length} 个模型
          </Button>
        ) : (
          <Tag>全部</Tag>
        ),
    },
    { title: '优先级', dataIndex: 'priority', width: 80, responsive: ['sm'] },
    { title: '状态', dataIndex: 'enabled', render: (e: boolean) => <Tag color={e ? 'success' : 'default'}>{e ? '启用' : '停用'}</Tag> },
    {
      title: '操作',
      fixed: 'right' as const,
      width: 150,
      render: (_: unknown, r: LLMChannel) => (
        <Space size={0} wrap>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>编辑</Button>
          <Popconfirm title="删除该渠道？" onConfirm={() => onDelete(r.id)}>
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
          <h2>LLM 渠道管理</h2>
          <p>配置上游模型提供商渠道，网关按优先级路由到匹配渠道</p>
        </div>
        <Space wrap style={{ justifyContent: 'flex-end' }}>
          <Button icon={<ExperimentOutlined />} onClick={() => { setTestOpen(true); testForm.resetFields(); }}>
            渠道测试
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增渠道
          </Button>
        </Space>
      </div>

      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message="模型列表为空表示接受全部模型；支持通配符（如 gpt-4o*）。优先级高的启用渠道优先被路由。"
      />

      <Card>
        <Table rowKey="id" loading={loading} columns={columns} dataSource={items} pagination={false} scroll={{ x: 800 }} size={isMobile ? 'small' : 'middle'} />
      </Card>

      <ModelListModal
        open={!!modelsView}
        title={`渠道模型 · ${modelsView?.name ?? ''}`}
        subtitle={
          modelsView && (
            <span>
              类型 <b>{modelsView.type === 'anthropic' ? 'Anthropic 兼容' : 'OpenAI 兼容'}</b> · 优先级 {modelsView.priority}
            </span>
          )
        }
        models={modelsView?.models}
        onClose={() => setModelsView(null)}
      />

      <Modal
        title={editing ? '编辑渠道' : '新增渠道'}
        open={open}
        onOk={onSave}
        confirmLoading={saving}
        onCancel={() => setOpen(false)}
        width={isMobile ? 'calc(100vw - 16px)' : 560}
        centered={isMobile}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="渠道名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input maxLength={100} placeholder="例如：OpenAI 官方 / Anthropic 中转" />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select
              options={[
                { label: 'OpenAI 兼容', value: 'openai' },
                { label: 'Anthropic 兼容', value: 'anthropic' },
              ]}
            />
          </Form.Item>
          <Form.Item name="base_url" label="Base URL" rules={[{ required: true, message: '请输入 Base URL' }]}>
            <Input placeholder="https://api.openai.com" />
          </Form.Item>
          <Form.Item name="api_key" label="API Key">
            <Input.Password placeholder="上游密钥" />
          </Form.Item>
          <Form.Item
            name="models"
            label="模型列表（空 = 全部）"
            extra={
              <Button
                size="small"
                type="link"
                style={{ paddingLeft: 0, marginTop: 4 }}
                loading={fetching}
                icon={<CloudDownloadOutlined />}
                onClick={onFetchModels}
              >
                自动获取模型
              </Button>
            }
          >
            <Select
              mode="tags"
              placeholder="输入模型名后回车，支持通配符 gpt-4o*"
              tokenSeparators={[',', '，']}
            />
          </Form.Item>
          <Form.Item name="priority" label="优先级" initialValue={0}>
            <InputNumber min={0} style={{ width: '100%' }} addonAfter="值越大越优先" />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} maxLength={500} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="渠道连通性测试"
        open={testOpen}
        onOk={onTest}
        confirmLoading={testing}
        onCancel={() => setTestOpen(false)}
        width={isMobile ? 'calc(100vw - 16px)' : 560}
        centered={isMobile}
        destroyOnClose
      >
        <Form form={testForm} layout="vertical">
          <Form.Item name="type" label="类型" initialValue="openai" rules={[{ required: true }]}>
            <Select
              options={[
                { label: 'OpenAI 兼容', value: 'openai' },
                { label: 'Anthropic 兼容', value: 'anthropic' },
              ]}
            />
          </Form.Item>
          <Form.Item name="base_url" label="Base URL" rules={[{ required: true, message: '请输入 Base URL' }]}>
            <Input placeholder="https://api.openai.com" />
          </Form.Item>
          <Form.Item name="api_key" label="API Key">
            <Input.Password placeholder="上游密钥" />
          </Form.Item>
        </Form>
        {testResult && (
          <Alert
            type={testResult.ok ? 'success' : 'error'}
            showIcon
            message={testResult.ok ? `连接成功（${testResult.latency_ms}ms），获取到 ${testResult.count} 个模型` : '连接失败'}
            description={
              testResult.ok ? (
                <Typography.Text style={{ fontSize: 12 }}>{testResult.models.join(', ')}</Typography.Text>
              ) : (
                testResult.error
              )
            }
          />
        )}
      </Modal>
    </div>
  );
}
