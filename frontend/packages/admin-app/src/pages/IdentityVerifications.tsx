import { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, Modal, Descriptions, Image, App, Select, Typography, Space } from 'antd';
import { CheckOutlined, CloseOutlined, EyeOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { IdentityVerificationAdmin, UserInfo } from '@mass/shared';
import { request } from '../api/request';
import { tagColor, typeLabel } from '@mass/shared';

interface VerificationItem extends IdentityVerificationAdmin {
  user?: UserInfo;
}

export default function IdentityVerifications() {
  const { message } = App.useApp();
  const [items, setItems] = useState<VerificationItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [status, setStatus] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [detail, setDetail] = useState<VerificationItem | null>(null);
  const [reviewing, setReviewing] = useState(false);

  const load = (p: number, s?: string) => {
    setLoading(true);
    request
      .get('/admin/identity-verifications', { params: { page: p, size, status: s } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(1), []);

  const review = async (action: 'approve' | 'reject') => {
    if (!detail) return;
    setReviewing(true);
    try {
      await request.post(`/admin/identity-verifications/${detail.id}/review`, { action });
      message.success(action === 'approve' ? '已通过' : '已驳回');
      setDetail(null);
      load(page, status);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setReviewing(false);
    }
  };

  const columns = [
    {
      title: '用户',
      render: (_: unknown, r: VerificationItem) => (
        <div>
          <Typography.Text strong>{r.user?.nickname || `#${r.user_id}`}</Typography.Text>
          <div>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{r.user?.email || ''}</Typography.Text>
          </div>
        </div>
      ),
    },
    { title: '真实姓名', dataIndex: 'real_name' },
    { title: '身份证号', dataIndex: 'id_number', render: (n: string) => <Typography.Text code style={{ fontSize: 12 }}>{n}</Typography.Text> },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '提交时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
    {
      title: '操作',
      render: (_: unknown, r: VerificationItem) =>
        r.status === 'pending' ? (
          <Button type="primary" size="small" icon={<EyeOutlined />} onClick={() => setDetail(r)}>
            审核
          </Button>
        ) : (
          '-'
        ),
    },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>实名认证审核</h2>
          <p>审核用户提交的真实身份认证申请</p>
        </div>
        <Select
          placeholder="按状态筛选"
          allowClear
          style={{ width: 150 }}
          value={status}
          onChange={(v) => {
            setStatus(v);
            setPage(1);
            load(1, v);
          }}
          options={[
            { label: '待审核', value: 'pending' },
            { label: '已通过', value: 'approved' },
            { label: '已驳回', value: 'rejected' },
          ]}
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
            onChange: (p) => {
              setPage(p);
              load(p, status);
            },
          }}
        />
      </Card>

      <Modal
        title="实名审核"
        open={!!detail}
        onCancel={() => setDetail(null)}
        width={600}
        footer={
          detail?.status === 'pending'
            ? [
                <Button key="reject" danger icon={<CloseOutlined />} loading={reviewing} onClick={() => review('reject')}>
                  驳回
                </Button>,
                <Button key="approve" type="primary" icon={<CheckOutlined />} loading={reviewing} onClick={() => review('approve')}>
                  通过
                </Button>,
              ]
            : [
                <Button key="close" onClick={() => setDetail(null)}>
                  关闭
                </Button>,
              ]
        }
      >
        {detail && (
          <div>
            <Descriptions column={1} size="small" bordered style={{ marginBottom: 16 }}>
              <Descriptions.Item label="用户">
                {detail.user?.nickname}（{detail.user?.email}）
              </Descriptions.Item>
              <Descriptions.Item label="真实姓名">{detail.real_name}</Descriptions.Item>
              <Descriptions.Item label="身份证号">{detail.id_number}</Descriptions.Item>
              <Descriptions.Item label="提交时间">{dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
              {detail.reject_reason && (
                <Descriptions.Item label="驳回原因">
                  <Typography.Text type="danger">{detail.reject_reason}</Typography.Text>
                </Descriptions.Item>
              )}
            </Descriptions>
            <Space size="large">
              <div>
                <Typography.Text type="secondary">人像面</Typography.Text>
                <Image src={detail.id_card_front} width={200} height={130} style={{ objectFit: 'cover', borderRadius: 8 }} fallback="data:image/svg+xml;base64,PHN2ZyB3aWR0aD0nMjAwJyBoZWlnaHQ9JzEzMCcgeG1sbnM9J2h0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnJz48cmVjdCB3aWR0aD0nMjAwJyBoZWlnaHQ9JzEzMCcgZmlsbD0nI2YxZjVmOScvPjx0ZXh0IHg9JzUwJScgeT0nNTAlJyBkb21pbmFudC1iYXNlbGluZT0nbWlkZGxlJyB0ZXh0LWFuY2hvcj0nbWlkZGxlJyBmaWxsPScjOTRhM2I4Jz7or6bnu5nkuI3lj6/liLDlm77niYc8L3RleHQ+PC9zdmc+" />
              </div>
              <div>
                <Typography.Text type="secondary">国徽面</Typography.Text>
                <Image src={detail.id_card_back} width={200} height={130} style={{ objectFit: 'cover', borderRadius: 8 }} fallback="data:image/svg+xml;base64,PHN2ZyB3aWR0aD0nMjAwJyBoZWlnaHQ9JzEzMCcgeG1sbnM9J2h0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnJz48cmVjdCB3aWR0aD0nMjAwJyBoZWlnaHQ9JzEzMCcgZmlsbD0nI2YxZjVmOScvPjx0ZXh0IHg9JzUwJScgeT0nNTAlJyBkb21pbmFudC1iYXNlbGluZT0nbWlkZGxlJyB0ZXh0LWFuY2hvcj0nbWlkZGxlJyBmaWxsPScjOTRhM2I4Jz7or6bnu5nkuI3lj6/liLDlm77niYc8L3RleHQ+PC9zdmc+" />
              </div>
            </Space>
          </div>
        )}
      </Modal>
    </div>
  );
}
