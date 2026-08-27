import { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Button, Progress, Typography, Modal, InputNumber, App, Alert, Tag, Descriptions, Empty } from 'antd';
import { AccountBookOutlined, SafetyCertificateOutlined, PayCircleOutlined } from '@ant-design/icons';
import type { CreditStatus } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, formatNumber, tagColor, typeLabel } from '@mass/shared';
import { useAuthStore } from '../store/auth';

export default function Credit() {
  const { message } = App.useApp();
  const refreshQuota = useAuthStore((s) => s.refreshQuota);
  const [status, setStatus] = useState<CreditStatus | null>(null);
  const [repayOpen, setRepayOpen] = useState(false);
  const [repayTokens, setRepayTokens] = useState<number>(0);
  const [repaying, setRepaying] = useState(false);
  const [applying, setApplying] = useState(false);

  const load = () => {
    request.get('/user/credit/status').then((r) => setStatus(r.data.data)).catch(() => {});
  };

  useEffect(load, []);

  const apply = async () => {
    setApplying(true);
    try {
      await request.post('/user/credit/apply');
      message.success('授信申请已提交，等待管理员审核');
      load();
    } catch {
      /* 拦截器已提示 */
    } finally {
      setApplying(false);
    }
  };

  const repay = async () => {
    if (repayTokens <= 0) {
      message.warning('请输入还款数量');
      return;
    }
    setRepaying(true);
    try {
      const data = await request.post('/user/credit/repay', { tokens: repayTokens }).then((r) => r.data.data);
      message.success('还款成功');
      setRepayOpen(false);
      refreshQuota();
      load();
      setStatus((s) => s && { ...s, credit_used: data.credit_used, credit_available: data.credit_available });
    } catch {
      /* 拦截器已提示 */
    } finally {
      setRepaying(false);
    }
  };

  if (!status) return null;

  const app = status.application;
  const totalLimit = status.credit_limit || 0;

  return (
    <div>
      <div className="page-header">
        <h2>Token 授信</h2>
        <p>累计消费满 ¥{status.threshold} 可申请 Token 授信额度，先使用后还款</p>
      </div>

      {!status.can_apply && !app && (
        <Alert
          style={{ marginBottom: 16 }}
          type="info"
          showIcon
          message={`授信条件：累计消费达到 ¥${status.threshold.toLocaleString()}（当前累计消费 ${formatMoney(status.consumed_total)}）`}
        />
      )}

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={8}>
          <Card>
            <Statistic
              title="累计消费"
              value={parseFloat(status.consumed_total)}
              precision={2}
              prefix={<PayCircleOutlined style={{ color: '#ea580c' }} />}
              suffix="¥"
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <Card>
            <Statistic
              title="授信总额度"
              value={totalLimit}
              prefix={<AccountBookOutlined style={{ color: '#059669' }} />}
              suffix="tokens"
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <Card>
            <Statistic
              title="可用授信"
              value={status.credit_available}
              prefix={<SafetyCertificateOutlined style={{ color: '#2563eb' }} />}
              suffix="tokens"
            />
          </Card>
        </Col>
      </Row>

      <Card style={{ marginTop: 16 }}>
        {totalLimit > 0 && (
          <div style={{ marginBottom: 20 }}>
            <Progress
              percent={Math.min(100, (status.credit_used / totalLimit) * 100)}
              format={() => `${formatNumber(status.credit_used)} / ${formatNumber(totalLimit)} tokens`}
              strokeColor={{ '0%': '#059669', '100%': '#10b981' }}
            />
            <Typography.Text type="secondary">
              待还 {formatNumber(status.credit_used)} tokens（可使用 Token 加油包额度还款）
            </Typography.Text>
          </div>
        )}

        {app ? (
          <div>
            <Typography.Title level={5}>最新申请</Typography.Title>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="状态">
                <Tag color={tagColor(app.status)}>{typeLabel[app.status] || app.status}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="申请时累计消费">{formatMoney(app.consumed_total)}</Descriptions.Item>
              <Descriptions.Item label="获批额度">{formatNumber(app.granted_tokens)} tokens</Descriptions.Item>
              <Descriptions.Item label="申请时间">{new Date(app.created_at).toLocaleString('zh-CN')}</Descriptions.Item>
              {app.reject_reason && (
                <Descriptions.Item label="驳回原因" span={2}>
                  <Typography.Text type="danger">{app.reject_reason}</Typography.Text>
                </Descriptions.Item>
              )}
            </Descriptions>
          </div>
        ) : (
          <Button type="primary" onClick={apply} loading={applying} disabled={!status.can_apply} size="large">
            申请授信
          </Button>
        )}

        {status.credit_used > 0 && (
          <Button style={{ marginTop: 16, marginLeft: 8 }} size="large" onClick={() => setRepayOpen(true)}>
            额度还款
          </Button>
        )}
      </Card>

      <Modal title="授信还款" open={repayOpen} onOk={repay} confirmLoading={repaying} onCancel={() => setRepayOpen(false)}>
        <p>
          待还额度：<b>{formatNumber(status.credit_used)}</b> tokens
        </p>
        <p>
          还款使用 Token 加油包额度（当前持有 <b>{formatNumber(useAuthStore.getState().quota?.token_credits ?? 0)}</b> tokens），
          数量不足请先购买加油包。
        </p>
        <InputNumber
          value={repayTokens}
          onChange={(v) => setRepayTokens(v ?? 0)}
          min={1}
          max={Math.min(status.credit_used, useAuthStore.getState().quota?.token_credits ?? 0)}
          style={{ width: '100%' }}
          addonAfter="tokens"
          placeholder="请输入还款数量"
        />
      </Modal>
    </div>
  );
}
