import { useEffect, useState } from 'react';
import {
  Card, Table, Button, Modal, Form, Input, Select, Space, Tag, Typography, App, Popconfirm, Alert,
} from 'antd';
import { PlusOutlined, CopyOutlined, KeyOutlined, DeleteOutlined, EyeOutlined, EyeInvisibleOutlined } from '@ant-design/icons';
import type { ApiKey, ModelCatalogEntry } from '@mass/shared';
import { request } from '../api/request';
import { tagColor, typeLabel, copyText } from '@mass/shared';

export default function ApiKeys() {
  const { message } = App.useApp();
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [models, setModels] = useState<ModelCatalogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newKey, setNewKey] = useState<{ full_key: string; name: string } | null>(null);
  const [form] = Form.useForm();

  const load = () => {
    setLoading(true);
    request
      .get('/user/api-keys')
      .then((r) => setKeys(r.data.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load();
    request.get('/models').then((r) => setModels(r.data.data || [])).catch(() => {});
  }, []);

  const onCreate = async () => {
    const values = await form.validateFields();
    setCreating(true);
    try {
      const data = await request.post('/user/api-keys', values).then((r) => r.data.data);
      setNewKey({ full_key: data.full_key, name: data.name });
      setOpen(false);
      form.resetFields();
      load();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setCreating(false);
    }
  };

  const onDelete = async (id: number) => {
    await request.delete(`/user/api-keys/${id}`);
    message.success('已删除');
    load();
  };

  const modelOptions = models.map((m) => ({ label: m.name, value: m.id }));

  const KeyCell = ({ prefix }: { prefix: string }) => {
    const [revealed, setRevealed] = useState(false);
    const value = `sk-${prefix}`;
    const display = revealed ? value : `${value}••••••`;
    return (
      <Space size={4} align="center" wrap>
        <Button
          type="text"
          size="small"
          aria-label={revealed ? '隐藏 Key' : '显示 Key'}
          icon={revealed ? <EyeInvisibleOutlined /> : <EyeOutlined />}
          onClick={() => setRevealed((v) => !v)}
        />
        <span className="key-text" title={value}>{display}</span>
        <Button
          type="text"
          size="small"
          aria-label="复制 Key"
          icon={<CopyOutlined />}
          onClick={() => {
            copyText(value).then((ok) =>
              ok ? message.success('已复制 Key 前缀') : message.error('复制失败'),
            );
          }}
        />
      </Space>
    );
  };

  const columns = [
    { title: '名称', dataIndex: 'name', render: (n: string) => <Typography.Text strong>{n}</Typography.Text> },
    {
      title: 'Key',
      dataIndex: 'key_prefix',
      render: (p: string) => <KeyCell prefix={p} />,
    },
    {
      title: '模型访问',
      dataIndex: 'model_access',
      responsive: ['md'] as ('md')[],
      render: (ma: string[]) =>
        ma?.length ? (
          <Space size={4} wrap>
            {ma.map((m) => <Tag key={m} bordered={false}>{m}</Tag>)}
          </Space>
        ) : (
          <Tag color="green">全部模型</Tag>
        ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag>,
    },
    {
      title: '最后使用',
      dataIndex: 'last_used_at',
      responsive: ['lg'] as ('lg')[],
      render: (t?: string) => (t ? new Date(t).toLocaleString('zh-CN') : '从未使用'),
    },
      { title: '创建时间', dataIndex: 'created_at', responsive: ['lg'] as ('lg')[], render: (t: string) => new Date(t).toLocaleString('zh-CN') },
    {
      title: '操作',
      render: (_: unknown, r: ApiKey) => (
        <Popconfirm title="删除后不可恢复，确定删除？" onConfirm={() => onDelete(r.id)}>
          <Button danger type="link" icon={<DeleteOutlined />} size="small">删除</Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <div>
      <div className="apikey-head page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', flexWrap: 'wrap', gap: 12 }}>
        <div>
          <h2>API Keys</h2>
          <p>用于调用 LLM 网关的密钥，请妥善保管</p>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)} style={{ flexShrink: 0 }}>创建 API Key</Button>
      </div>

      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message="如何调用"
        description={
          <Typography.Text style={{ fontSize: 13 }}>
            将 Base URL 指向 <Typography.Text code>https://&lt;站点域名&gt;/v1</Typography.Text>，在请求头携带{' '}
            <Typography.Text code>Authorization: Bearer sk-xxx</Typography.Text>（或 <Typography.Text code>X-API-Key</Typography.Text>）即可，
            兼容 OpenAI / Anthropic SDK。
          </Typography.Text>
        }
      />

      <Card>
        <Table rowKey="id" loading={loading} columns={columns} dataSource={keys} pagination={false} scroll={{ x: 'max-content' }} />
      </Card>

      <Modal title="创建 API Key" open={open} onOk={onCreate} confirmLoading={creating} onCancel={() => setOpen(false)} destroyOnClose>
        <Form form={form} layout="vertical" initialValues={{ model_access: [] }}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="例如：生产环境 / 本地开发" maxLength={100} />
          </Form.Item>
          <Form.Item name="model_access" label="模型访问权限（不选则为全部模型）">
            <Select mode="multiple" placeholder="选择允许访问的模型" options={modelOptions} allowClear />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="Key 创建成功"
        open={!!newKey}
        footer={null}
        onCancel={() => setNewKey(null)}
        destroyOnClose
      >
        <Alert
          type="warning"
          showIcon
          message="请立即复制保存，完整 Key 仅显示这一次！"
          description={
            <div>
              <Typography.Text code style={{ fontSize: 13, wordBreak: 'break-all', display: 'block', marginBottom: 12 }}>
                <KeyOutlined /> {newKey?.full_key}
              </Typography.Text>
              <Button
                type="primary"
                icon={<CopyOutlined />}
                block
                onClick={() => {
                  copyText(newKey?.full_key || '').then((ok) =>
                    ok ? message.success('已复制完整 Key') : message.error('复制失败'),
                  );
                }}
              >
                复制完整 Key
              </Button>
            </div>
          }
        />
        <Button type="primary" block style={{ marginTop: 12 }} onClick={() => setNewKey(null)}>
          我已保存
        </Button>
      </Modal>
    </div>
  );
}
