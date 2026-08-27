import { useEffect, useState } from 'react';
import { Row, Col, Card, Button, Typography, Tag, Modal, App, Spin, Empty } from 'antd';
import { GiftOutlined } from '@ant-design/icons';
import type { TokenPackage } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, formatNumber } from '@mass/shared';
import { useAuthStore } from '../store/auth';

export default function TokenPackages() {
  const { message } = App.useApp();
  const refreshQuota = useAuthStore((s) => s.refreshQuota);
  const balance = useAuthStore((s) => s.quota?.balance);
  const [packages, setPackages] = useState<TokenPackage[]>([]);
  const [loading, setLoading] = useState(true);
  const [confirmPkg, setConfirmPkg] = useState<TokenPackage | null>(null);
  const [buying, setBuying] = useState(false);

  useEffect(() => {
    request
      .get('/user/token-packages')
      .then((r) => setPackages(r.data.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const onBuy = async () => {
    if (!confirmPkg) return;
    setBuying(true);
    try {
      const data = await request
        .post(`/user/token-packages/${confirmPkg.id}/purchase`)
        .then((r) => r.data.data);
      message.success(`购买成功，到账 ${formatNumber(data.token_credits)} tokens`);
      setConfirmPkg(null);
      refreshQuota();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setBuying(false);
    }
  };

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '80px auto' }} />;

  return (
    <div>
      <div className="page-header">
        <h2>Token 加油包</h2>
        <p>用余额购买永久 Token 额度，用于授信还款与按量抵扣（余额：{balance ? formatMoney(balance) : '—'}）</p>
      </div>

      {!packages.length ? (
        <Empty description="暂无可购买的加油包" />
      ) : (
        <Row gutter={[16, 16]}>
          {packages.map((p) => (
            <Col xs={24} sm={12} lg={8} key={p.id}>
              <Card hoverable style={{ height: '100%' }}>
                <div style={{ textAlign: 'center', marginBottom: 12 }}>
                  <GiftOutlined style={{ fontSize: 36, color: '#7c3aed' }} />
                </div>
                <Typography.Title level={5} style={{ textAlign: 'center', marginTop: 0 }}>
                  {p.name}
                </Typography.Title>
                <Typography.Paragraph type="secondary" style={{ textAlign: 'center', minHeight: 40 }}>
                  {p.description}
                </Typography.Paragraph>
                <div style={{ textAlign: 'center', marginBottom: 12 }}>
                  <Typography.Text style={{ fontSize: 26, fontWeight: 700, color: '#7c3aed' }} className="money">
                    {formatMoney(p.price)}
                  </Typography.Text>
                </div>
                <div style={{ textAlign: 'center', marginBottom: 16 }}>
                  <Tag color="purple">{formatNumber(p.tokens)} tokens</Tag>
                  {p.bonus_tokens > 0 && <Tag color="magenta">赠送 {formatNumber(p.bonus_tokens)}</Tag>}
                </div>
                <Button type="primary" block onClick={() => setConfirmPkg(p)}>
                  余额购买
                </Button>
              </Card>
            </Col>
          ))}
        </Row>
      )}

      <Modal
        title="确认购买"
        open={!!confirmPkg}
        onOk={onBuy}
        confirmLoading={buying}
        onCancel={() => setConfirmPkg(null)}
      >
        {confirmPkg && (
          <div>
            <p>
              购买 <b>{confirmPkg.name}</b>（{formatMoney(confirmPkg.price)}），获得{' '}
              <b>{formatNumber(confirmPkg.tokens + confirmPkg.bonus_tokens)}</b> tokens 额度。
            </p>
            <p>当前余额：{formatMoney(balance)}</p>
          </div>
        )}
      </Modal>
    </div>
  );
}
