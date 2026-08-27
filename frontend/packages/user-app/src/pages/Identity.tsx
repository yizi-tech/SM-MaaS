import { useEffect, useState } from 'react';
import {
  Card, Form, Input, Button, Upload, App, Tag, Result, Alert, Typography, Space, Row, Col,
} from 'antd';
import { PlusOutlined, IdcardOutlined } from '@ant-design/icons';
import type { UploadFile } from 'antd';
import type { IdentityVerification } from '@mass/shared';
import { request } from '../api/request';
import { tagColor, typeLabel } from '@mass/shared';
import { useAuthStore } from '../store/auth';

const statusText: Record<string, { title: string; sub: string; color: string }> = {
  verified: { title: '实名认证已通过', sub: '你的实名信息已通过审核', color: 'success' },
  pending: { title: '实名认证审核中', sub: '我们将在 1-3 个工作日内完成审核，请耐心等待', color: 'processing' },
  rejected: { title: '实名认证未通过', sub: '审核被驳回，请根据原因重新提交', color: 'error' },
  unverified: { title: '尚未进行实名认证', sub: '提交真实姓名与身份证信息完成认证', color: 'default' },
};

export default function Identity() {
  const { message } = App.useApp();
  const refreshQuota = useAuthStore((s) => s.refreshQuota);
  const [info, setInfo] = useState<IdentityVerification | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [front, setFront] = useState<UploadFile[]>([]);
  const [back, setBack] = useState<UploadFile[]>([]);
  const [form] = Form.useForm();

  useEffect(() => {
    request
      .get('/user/identity-verification')
      .then((r) => {
        setInfo(r.data.data);
        if (r.data.data?.status === 'rejected') {
          form.setFieldsValue({ real_name: r.data.data.real_name });
        }
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const uploadProps = (onChange: (files: UploadFile[]) => void) => ({
    listType: 'picture-card' as const,
    maxCount: 1,
    accept: 'image/*',
    beforeUpload: (file: UploadFile) => {
      const isImage = file.type?.startsWith('image/');
      if (!isImage) message.error('只能上传图片');
      return isImage || Upload.LIST_IGNORE;
    },
    customRequest: async (options: {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      [key: string]: any;
    }) => {
      const { file, onSuccess, onError } = options;
      const fd = new FormData();
      fd.append('file', file as File);
      try {
        const data = await request.post('/user/upload', fd).then((r) => r.data.data);
        onSuccess?.(data);
      } catch (e) {
        onError?.(e as Error);
      }
    },
    onChange: ({ fileList }: { fileList: UploadFile[] }) => onChange(fileList),
  });

  const onSubmit = async (values: { real_name: string; id_number: string }) => {
    const frontFile = front[0] as UploadFile & { response?: { url: string } };
    const backFile = back[0] as UploadFile & { response?: { url: string } };
    if (!frontFile?.response?.url || !backFile?.response?.url) {
      message.warning('请上传身份证正反面照片');
      return;
    }
    setSubmitting(true);
    try {
      await request.post('/user/identity-verification', {
        real_name: values.real_name,
        id_number: values.id_number,
        id_card_front: frontFile.response.url,
        id_card_back: backFile.response.url,
      });
      message.success('提交成功，等待审核');
      refreshQuota();
      const r = await request.get('/user/identity-verification');
      setInfo(r.data.data);
    } catch {
      /* 拦截器已提示 */
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) return null;

  const st = statusText[info?.status || 'unverified'];

  return (
    <div style={{ maxWidth: 680, margin: '0 auto' }}>
      <div className="page-header">
        <h2>实名认证</h2>
        <p>平台要求真实身份认证，用于安全风控与发票开具</p>
      </div>

      {info && info.status !== 'unverified' ? (
        <Card>
          <Result
            status={st.color as 'success' | 'info' | 'warning' | 'error'}
            title={st.title}
            subTitle={
              <div>
                <p>{st.sub}</p>
                {info.status === 'rejected' && info.reject_reason && (
                  <Alert type="error" showIcon message={`驳回原因：${info.reject_reason}`} />
                )}
                {info.real_name && info.status === 'verified' && (
                  <Typography.Text type="secondary">认证姓名：{info.real_name}</Typography.Text>
                )}
              </div>
            }
            extra={
              info.status === 'rejected' ? (
                <Button type="primary" onClick={() => { setInfo({ ...info, status: 'unverified' }); form.resetFields(); setFront([]); setBack([]); }}>
                  重新提交
                </Button>
              ) : null
            }
          />
        </Card>
      ) : (
        <Card title={<span><IdcardOutlined /> 提交实名信息</span>}>
          <Alert
            style={{ marginBottom: 24 }}
            type="info"
            showIcon
            message="个人信息将严格保密，仅用于实名认证与风控审核。实名认证通过后，方可申请开具企业发票。"
          />
          <Form form={form} layout="vertical" onFinish={onSubmit}>
            <Row gutter={16}>
              <Col xs={24} sm={12}>
                <Form.Item name="real_name" label="真实姓名" rules={[{ required: true, message: '请输入真实姓名' }]}>
                  <Input placeholder="与身份证一致" maxLength={50} />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12}>
                <Form.Item name="id_number" label="身份证号" rules={[{ required: true, message: '请输入身份证号' }]}>
                  <Input placeholder="18 位身份证号码" maxLength={18} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12}>
                <Form.Item label="身份证人像面（正面）" required>
                  <Upload {...uploadProps(setFront)}>
                    {front.length ? null : (
                      <div>
                        <PlusOutlined />
                        <div style={{ marginTop: 8 }}>上传照片</div>
                      </div>
                    )}
                  </Upload>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    清晰的证件人像面，文字正向无遮挡
                  </Typography.Text>
                </Form.Item>
              </Col>
              <Col xs={24} sm={12}>
                <Form.Item label="身份证国徽面（反面）" required>
                  <Upload {...uploadProps(setBack)}>
                    {back.length ? null : (
                      <div>
                        <PlusOutlined />
                        <div style={{ marginTop: 8 }}>上传照片</div>
                      </div>
                    )}
                  </Upload>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    清晰的证件国徽面，文字正向无遮挡
                  </Typography.Text>
                </Form.Item>
              </Col>
            </Row>
            <Button type="primary" htmlType="submit" block loading={submitting} style={{ marginTop: 8 }}>
              提交认证
            </Button>
          </Form>
        </Card>
      )}
    </div>
  );
}
