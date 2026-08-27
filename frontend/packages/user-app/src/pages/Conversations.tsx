import { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, Drawer, Descriptions, Typography, Select, Space, App, Alert } from 'antd';
import { DownloadOutlined, EyeOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { ConversationLog } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, formatNumber, tagColor, typeLabel } from '@mass/shared';

export default function Conversations() {
  const { message } = App.useApp();
  const [items, setItems] = useState<ConversationLog[]>([]);
  const [models, setModels] = useState<string[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [model, setModel] = useState<string | undefined>();
  const [loading, setLoading] = useState(true);
  const [detail, setDetail] = useState<ConversationLog | null>(null);

  const load = (p: number, m?: string) => {
    setLoading(true);
    request
      .get('/user/conversations', { params: { page: p, size, model: m } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
        setModels(r.data.data?.models || []);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(1), []);

  const parseMessages = (raw: string): { role: string; content: string }[] => {
    try {
      return JSON.parse(raw);
    } catch {
      return [];
    }
  };

  const renderContent = (c: string) => {
    if (typeof c !== 'string') return String(c || '');
    try {
      const parsed = JSON.parse(c);
      if (Array.isArray(parsed)) {
        return parsed
          .map((p: { text?: string }) => p.text || '')
          .join('\n');
      }
      if (typeof parsed === 'object' && parsed !== null) return JSON.stringify(parsed, null, 2);
      return c;
    } catch {
      return c;
    }
  };

  const exportJsonl = async () => {
    try {
      const res = await request.get('/user/conversations/export.jsonl', { responseType: 'blob' });
      const url = URL.createObjectURL(res.data);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'conversations.jsonl';
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      message.error('导出失败');
    }
  };

  const columns = [
    {
      title: '模型',
      dataIndex: 'model',
      render: (m: string) => <Typography.Text code>{m}</Typography.Text>,
    },
    {
      title: '输入 / 输出 tokens',
      render: (_: unknown, r: ConversationLog) => `${formatNumber(r.tokens_in)} / ${formatNumber(r.tokens_out)}`,
    },
    { title: '费用', dataIndex: 'cost', render: (c: string) => <span className="money">{formatMoney(c)}</span> },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm:ss') },
    {
      title: '操作',
      render: (_: unknown, r: ConversationLog) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setDetail(r)}>
          查看
        </Button>
      ),
    },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>对话记录</h2>
          <p>平台留存的调用对话数据，可导出为 JSONL 用于微调或备份</p>
        </div>
        <Space>
          <Select
            placeholder="按模型筛选"
            allowClear
            style={{ width: 180 }}
            value={model}
            onChange={(v) => {
              setModel(v);
              setPage(1);
              load(1, v);
            }}
            options={models.map((m) => ({ label: m, value: m }))}
          />
          <Button icon={<DownloadOutlined />} onClick={exportJsonl}>
            导出 JSONL
          </Button>
        </Space>
      </div>

      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message="对话记录包含你的请求与回复内容，用于数据留存与质量审查；如需删除请联系管理员。"
      />

      <Card>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 800 }}
          pagination={{ current: page, total, pageSize: size, onChange: (p) => { setPage(p); load(p, model); } }}
        />
      </Card>

      <Drawer title="对话详情" open={!!detail} onClose={() => setDetail(null)} width={640}>
        {detail && (
          <div>
            <Descriptions column={2} size="small" bordered style={{ marginBottom: 16 }}>
              <Descriptions.Item label="请求 ID" span={2}><Typography.Text code style={{ fontSize: 12 }}>{detail.request_id}</Typography.Text></Descriptions.Item>
              <Descriptions.Item label="模型">{detail.model}</Descriptions.Item>
              <Descriptions.Item label="流式">{detail.stream ? '是' : '否'}</Descriptions.Item>
              <Descriptions.Item label="tokens 入/出">{formatNumber(detail.tokens_in)} / {formatNumber(detail.tokens_out)}</Descriptions.Item>
              <Descriptions.Item label="费用">{formatMoney(detail.cost)}</Descriptions.Item>
              <Descriptions.Item label="时间" span={2}>{dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
            </Descriptions>
            <Typography.Title level={5}>请求消息</Typography.Title>
            {parseMessages(detail.messages).map((m, i) => (
              <div key={i} style={{ marginBottom: 12 }}>
                <Tag color={m.role === 'assistant' ? 'blue' : m.role === 'system' ? 'gold' : 'green'}>{m.role}</Tag>
                <pre style={{ background: '#f8fafc', padding: 12, borderRadius: 8, whiteSpace: 'pre-wrap', fontSize: 13 }}>
                  {renderContent(m.content)}
                </pre>
              </div>
            ))}
            <Typography.Title level={5}>回复内容</Typography.Title>
            <pre style={{ background: '#eff6ff', padding: 12, borderRadius: 8, whiteSpace: 'pre-wrap', fontSize: 13 }}>
              {renderContent(detail.response)}
            </pre>
          </div>
        )}
      </Drawer>
    </div>
  );
}
