import { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, App, Typography, Popconfirm, Empty, Spin } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { ResetCoupon } from '@mass/shared';
import { request } from '../api/request';
import { tagColor, typeLabel } from '@mass/shared';
import { useAuthStore } from '../store/auth';

export default function ResetCoupons() {
  const { message } = App.useApp();
  const refreshQuota = useAuthStore((s) => s.refreshQuota);
  const [coupons, setCoupons] = useState<ResetCoupon[]>([]);
  const [loading, setLoading] = useState(true);
  const [redeeming, setRedeeming] = useState(false);

  const load = () => {
    setLoading(true);
    request
      .get('/user/reset-coupons')
      .then((r) => setCoupons(r.data.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const onRedeem = async (id: number) => {
    setRedeeming(true);
    try {
      const data = await request.post(`/user/reset-coupons/${id}/redeem`).then((r) => r.data.data);
      message.success(`兑换成功，已重置 ${data.reset_count} 个订阅的已用额度`);
      refreshQuota();
      load();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setRedeeming(false);
    }
  };

  const columns = [
    {
      title: '券码',
      dataIndex: 'code',
      render: (c: string) => (
        <Typography.Text code copyable style={{ fontSize: 13 }}>
          {c}
        </Typography.Text>
      ),
    },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '备注', dataIndex: 'note', render: (n?: string) => n || '-' },
    { title: '发放时间', dataIndex: 'created_at', render: (t: string) => new Date(t).toLocaleString('zh-CN') },
    { title: '使用时间', dataIndex: 'used_at', render: (t?: string) => (t ? new Date(t).toLocaleString('zh-CN') : '-') },
    {
      title: '操作',
      render: (_: unknown, r: ResetCoupon) =>
        r.status === 'unused' ? (
          <Popconfirm title="兑换后将重置当前订阅的已用 Token 额度，确认？" onConfirm={() => onRedeem(r.id)}>
            <Button type="link" size="small" loading={redeeming}>
              兑换
            </Button>
          </Popconfirm>
        ) : (
          '-'
        ),
    },
  ];

  return (
    <div>
      <div className="page-header">
        <h2>重置券</h2>
        <p>使用重置券可将当前订阅的已用 Token 额度重置为 0（管理员发放）</p>
      </div>
      <Card>
        {coupons.length ? (
          <Table rowKey="id" loading={loading} columns={columns} dataSource={coupons} pagination={false} scroll={{ x: 700 }} />
        ) : (
          <Empty description="暂无重置券，如有需要请联系管理员发放" image={Empty.PRESENTED_IMAGE_SIMPLE}>
            <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
          </Empty>
        )}
      </Card>
    </div>
  );
}
