import { useEffect, useState } from 'react';
import { Card, List, Tag, Button, Typography, Empty, Spin, Badge, Space, App } from 'antd';
import { CheckOutlined, BellOutlined } from '@ant-design/icons';
import type { Notification } from '@mass/shared';
import { request } from '../api/request';
import { tagColor } from '@mass/shared';

export default function Notifications() {
  const { message } = App.useApp();
  const [items, setItems] = useState<Notification[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [loading, setLoading] = useState(true);

  const load = (p: number) => {
    setLoading(true);
    request
      .get('/user/notifications', { params: { page: p, size } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(1), []);

  const markRead = async (id: number) => {
    await request.put(`/user/notifications/${id}/read`);
    setItems((list) => list.map((n) => (n.id === id ? { ...n, is_read: true } : n)));
  };

  const markAll = async () => {
    await request.put('/user/notifications/read-all');
    setItems((list) => list.map((n) => ({ ...n, is_read: true })));
    message.success('已全部标记为已读');
  };

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>站内通知</h2>
          <p>平台公告与重要信息</p>
        </div>
        <Button icon={<CheckOutlined />} onClick={markAll} disabled={!items.some((n) => !n.is_read)}>
          全部已读
        </Button>
      </div>
      <Card>
        <List
          loading={loading}
          itemLayout="horizontal"
          dataSource={items}
          locale={{ emptyText: <Empty description="暂无通知" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
          pagination={{
            current: page,
            total,
            pageSize: size,
            onChange: (p) => {
              setPage(p);
              load(p);
            },
          }}
          renderItem={(n) => (
            <List.Item
              actions={[
                !n.is_read && (
                  <Button key="read" type="link" size="small" onClick={() => markRead(n.id)}>
                    标记已读
                  </Button>
                ),
              ]}
              style={{ cursor: 'pointer' }}
              onClick={() => !n.is_read && markRead(n.id)}
            >
              <List.Item.Meta
                avatar={
                  <Badge dot={!n.is_read} offset={[-2, 2]}>
                    <BellOutlined style={{ fontSize: 20, color: '#2563eb' }} />
                  </Badge>
                }
                title={
                  <Space>
                    <Typography.Text strong={!n.is_read}>{n.title}</Typography.Text>
                    <Tag color={tagColor(n.type)} bordered={false}>{n.type === 'system' ? '系统' : n.type}</Tag>
                  </Space>
                }
                description={
                  <div>
                    <Typography.Paragraph style={{ marginBottom: 4, whiteSpace: 'pre-wrap' }} type="secondary">
                      {n.content}
                    </Typography.Paragraph>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {new Date(n.created_at).toLocaleString('zh-CN')}
                    </Typography.Text>
                  </div>
                }
              />
            </List.Item>
          )}
        />
      </Card>
    </div>
  );
}
