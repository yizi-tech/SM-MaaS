import { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, Modal, Input, Descriptions, Typography, App } from 'antd';
import { NotificationOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { request } from '../api/request';
import { formatNumber, tagColor, typeLabel } from '@mass/shared';

interface CreditCollection {
  id: number;
  user_id: number;
  user_email?: string;
  tokens_due: number;
  note: string;
  created_at: string;
}

export default function CreditCollections() {
  const { message } = App.useApp();
  const [items, setItems] = useState<CreditCollection[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [loading, setLoading] = useState(false);
  const [collecting, setCollecting] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [userId, setUserId] = useState<number | undefined>();
  const [note, setNote] = useState('');

  const load = (p: number) => {
    setLoading(true);
    request
      .get('/admin/credit-collections', { params: { page: p, size } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(1), []);

  const submit = async () => {
    if (!userId || userId <= 0) {
      message.warning('请输入用户 ID');
      return;
    }
    setSubmitting(true);
    try {
      await request.post('/admin/credit-collect', { user_id: userId, note: note.trim() });
      message.success('已发送催款通知');
      setCollecting(false);
      setUserId(undefined);
      setNote('');
      load(page);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSubmitting(false);
    }
  };

  const columns = [
    {
      title: '用户',
      render: (_: unknown, r: CreditCollection) => (
        <div>
          <Typography.Text strong>#{r.user_id}</Typography.Text>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{r.user_email || '-'}</Typography.Text>
          </div>
        </div>
      ),
    },
    {
      title: '待还 Tokens',
      dataIndex: 'tokens_due',
      render: (v: number) => <b className="money">{formatNumber(v)}</b>,
    },
    { title: '催款备注', dataIndex: 'note', ellipsis: true },
    { title: '时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm:ss') },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>催账管理</h2>
          <p>向有授信待还的用户发送催款通知</p>
        </div>
        <Button type="primary" icon={<NotificationOutlined />} onClick={() => setCollecting(true)}>
          发起催收
        </Button>
      </div>
      <Card>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 800 }}
          pagination={{
            current: page,
            total,
            pageSize: size,
            onChange: (p) => { setPage(p); load(p); },
          }}
        />
      </Card>

      <Modal
        title="发起催收"
        open={collecting}
        onCancel={() => { setCollecting(false); setUserId(undefined); setNote(''); }}
        onOk={submit}
        okText="发送催款通知"
        confirmLoading={submitting}
        destroyOnClose
      >
        <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginBottom: 16 }}>
          系统将向该用户发送一条站内催款通知（额度按用户当前已用授信自动计算）。若用户暂无待还额度，操作会被拒绝。
        </Typography.Paragraph>
        <Descriptions column={1} size="small" bordered style={{ marginBottom: 16 }}>
          <Descriptions.Item label="用户 ID">
            <Input
              type="number"
              min={1}
              placeholder="请输入用户 ID"
              value={userId}
              onChange={(e) => setUserId(Number(e.target.value))}
            />
          </Descriptions.Item>
          <Descriptions.Item label="催收备注">
            <Input.TextArea
              rows={3}
              placeholder="选填，默认：请尽快购买加油包并在充值页归还授信额度"
              value={note}
              onChange={(e) => setNote(e.target.value)}
            />
          </Descriptions.Item>
        </Descriptions>
      </Modal>
    </div>
  );
}
