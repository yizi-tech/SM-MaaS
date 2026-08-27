import { useEffect, useState } from 'react';
import {
  Card, Table, Tag, Button, Typography, Modal, Form, Input, Radio, InputNumber, App, Space, Statistic, Descriptions,
} from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import type { Invoice, InvoiceQuota } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, tagColor, typeLabel } from '@mass/shared';

export default function Invoices() {
  const { message } = App.useApp();
  const [quota, setQuota] = useState<InvoiceQuota | null>(null);
  const [items, setItems] = useState<Invoice[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm();
  const [titleType, setTitleType] = useState<'company' | 'personal'>('company');
  const [invoiceType, setInvoiceType] = useState<'normal' | 'vat'>('normal');

  const loadQuota = () => {
    request.get('/user/invoice-quota').then((r) => setQuota(r.data.data)).catch(() => {});
  };

  const load = (p: number) => {
    setLoading(true);
    request
      .get('/user/invoices', { params: { page: p, size } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    loadQuota();
    load(1);
  }, []);

  const onCreate = async () => {
    const values = await form.validateFields();
    setCreating(true);
    try {
      await request.post('/user/invoices', { ...values, amount: String(values.amount) });
      message.success('发票申请已提交');
      setOpen(false);
      form.resetFields();
      loadQuota();
      load(1);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setCreating(false);
    }
  };

  const columns = [
    { title: '金额', dataIndex: 'amount', render: (a: string) => <b className="money">{formatMoney(a)}</b> },
    { title: '抬头', dataIndex: 'title', render: (t: string) => <Typography.Text strong>{t}</Typography.Text> },
    { title: '抬头类型', dataIndex: 'title_type', render: (t: string) => typeLabel[t] || t },
    { title: '发票类型', dataIndex: 'invoice_type', render: (t: string) => <Tag color={t === 'vat' ? 'gold' : 'default'}>{typeLabel[t] || t}</Tag> },
    { title: '发票号', dataIndex: 'invoice_no', render: (n?: string) => n || '-' },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '驳回原因', dataIndex: 'reject_reason', render: (r?: string) => (r ? <Typography.Text type="danger" style={{ fontSize: 12 }}>{r}</Typography.Text> : '-') },
    { title: '申请时间', dataIndex: 'created_at', render: (t: string) => new Date(t).toLocaleString('zh-CN') },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>发票管理</h2>
          <p>对已充值金额申请开具发票</p>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>
          申请发票
        </Button>
      </div>

      <Card style={{ marginBottom: 16 }}>
        <Space size="large" wrap>
          <Statistic title="累计充值" value={parseFloat(quota?.recharged || '0')} precision={2} prefix="¥" />
          <Statistic title="已开/申请占用" value={parseFloat(quota?.occupied || '0')} precision={2} prefix="¥" />
          <Statistic title="可开票额度" value={parseFloat(quota?.quota || '0')} precision={2} prefix="¥" valueStyle={{ color: '#2563eb' }} />
        </Space>
      </Card>

      <Card>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 900 }}
          pagination={{ current: page, total, pageSize: size, onChange: (p) => { setPage(p); load(p); } }}
        />
      </Card>

      <Modal
        title="申请发票"
        open={open}
        onOk={onCreate}
        confirmLoading={creating}
        onCancel={() => setOpen(false)}
        width={560}
        destroyOnClose
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ title_type: 'company', invoice_type: 'normal' }}
          onValuesChange={(v) => {
            if (v.title_type) setTitleType(v.title_type);
            if (v.invoice_type) setInvoiceType(v.invoice_type);
          }}
        >
          <Space size="large" style={{ display: 'flex' }}>
            <Form.Item name="title_type" label="抬头类型" rules={[{ required: true }]}>
              <Radio.Group>
                <Radio value="company">企业</Radio>
                <Radio value="personal">个人</Radio>
              </Radio.Group>
            </Form.Item>
            <Form.Item name="invoice_type" label="发票类型" rules={[{ required: true }]}>
              <Radio.Group>
                <Radio value="normal">普票</Radio>
                <Radio value="vat">专票</Radio>
              </Radio.Group>
            </Form.Item>
          </Space>
          <Form.Item name="amount" label="开票金额" rules={[{ required: true, message: '请输入开票金额' }]} extra={`可开票额度：${quota ? formatMoney(quota.quota) : '—'}`}>
            <InputNumber min={0.01} style={{ width: '100%' }} addonBefore="¥" placeholder="0.00" />
          </Form.Item>
          <Form.Item name="title" label="发票抬头" rules={[{ required: true, message: '请输入发票抬头' }]}>
            <Input placeholder="公司名称 / 个人姓名" maxLength={200} />
          </Form.Item>
          {titleType === 'company' && (
            <Form.Item name="tax_no" label="税号" rules={[{ required: true, message: '企业抬头必须填写税号' }]}>
              <Input placeholder="纳税人识别号" maxLength={50} />
            </Form.Item>
          )}
          {invoiceType === 'vat' && (
            <>
              <Form.Item name="bank_name" label="开户行" rules={[{ required: true, message: '专票必须填写开户行' }]}>
                <Input maxLength={100} />
              </Form.Item>
              <Form.Item name="bank_account" label="银行账号" rules={[{ required: true, message: '专票必须填写银行账号' }]}>
                <Input maxLength={50} />
              </Form.Item>
              <Form.Item name="address" label="注册地址">
                <Input maxLength={200} />
              </Form.Item>
              <Form.Item name="phone" label="注册电话">
                <Input maxLength={30} />
              </Form.Item>
            </>
          )}
          <Form.Item name="email" label="发票接收邮箱" rules={[{ type: 'email', message: '邮箱格式不正确' }]}>
            <Input placeholder="接收电子发票" maxLength={100} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} maxLength={500} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
