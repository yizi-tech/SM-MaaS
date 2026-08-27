import { Button, Empty, Modal, Space, Tag, Typography } from 'antd';
import { CopyOutlined } from '@ant-design/icons';
import { useIsMobile } from '../hooks/useIsMobile';

interface ModelListModalProps {
  open: boolean;
  title: string;
  /** 弹窗顶部说明，如套餐价格 / 渠道类型 */
  subtitle?: React.ReactNode;
  models: string[] | undefined;
  onClose: () => void;
}

/** 模型列表弹窗：以 Tag 网格展示模型白名单/渠道模型，支持一键复制 */
export default function ModelListModal({ open, title, subtitle, models, onClose }: ModelListModalProps) {
  const isMobile = useIsMobile();
  const list = models ?? [];

  const copyAll = async () => {
    try {
      await navigator.clipboard.writeText(list.join('\n'));
    } catch {
      /* 剪贴板不可用时静默失败 */
    }
  };

  return (
    <Modal
      title={title}
      open={open}
      footer={null}
      onCancel={onClose}
      width={isMobile ? 'calc(100vw - 16px)' : 560}
      centered={isMobile}
    >
      {subtitle && (
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12, fontSize: 13 }}>
          {subtitle}
        </Typography.Paragraph>
      )}
      {list.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未配置模型（表示全部模型可用）" />
      ) : (
        <>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, maxHeight: '50vh', overflowY: 'auto', alignContent: 'flex-start' }}>
            {list.map((m) => (
              <Tag key={m} color="blue" style={{ marginInlineEnd: 0, fontSize: 12.5, fontFamily: 'SFMono-Regular, Consolas, monospace' }}>
                {m}
              </Tag>
            ))}
          </div>
          <div style={{ marginTop: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12 }}>
            <Typography.Text type="secondary" style={{ fontSize: 12.5 }}>
              共 {list.length} 个模型
            </Typography.Text>
            <Button size="small" icon={<CopyOutlined />} onClick={copyAll}>
              复制全部
            </Button>
          </div>
        </>
      )}
    </Modal>
  );
}
