import { useEffect, useState } from 'react';
import { Card, Table, Tag, Input, Button, Space, Typography, Avatar, Select } from 'antd';
import { SearchOutlined, EyeOutlined, UserOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import dayjs from 'dayjs';
import type { UserInfo } from '@mass/shared';
import { request } from '../api/request';
import { formatMoney, tagColor, typeLabel } from '@mass/shared';

export default function Users() {
  const navigate = useNavigate();
  const [items, setItems] = useState<UserInfo[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size] = useState(15);
  const [keyword, setKeyword] = useState('');
  const [search, setSearch] = useState('');
  const [loading, setLoading] = useState(false);

  const load = (p: number, kw: string) => {
    setLoading(true);
    request
      .get('/admin/users', { params: { page: p, size, keyword: kw } })
      .then((r) => {
        setItems(r.data.data?.items || []);
        setTotal(r.data.data?.total || 0);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  };

  useEffect(() => load(1, ''), []);

  const columns = [
    {
      title: '用户',
      render: (_: unknown, r: UserInfo) => (
        <Space>
          <Avatar size={32} src={r.avatar} icon={<UserOutlined />} />
          <div>
            <Typography.Text strong>{r.nickname}</Typography.Text>
            <div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>{r.email}</Typography.Text>
            </div>
          </div>
        </Space>
      ),
    },
    { title: '角色', dataIndex: 'role', render: (r: string) => <Tag color={r === 'admin' ? 'gold' : 'default'}>{r === 'admin' ? '管理员' : '用户'}</Tag> },
    { title: '状态', dataIndex: 'status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '实名', dataIndex: 'real_name_status', render: (s: string) => <Tag color={tagColor(s)}>{typeLabel[s] || s}</Tag> },
    { title: '余额', dataIndex: 'balance', render: (b: string) => <span className="money">{formatMoney(b)}</span> },
    { title: '手机号', dataIndex: 'phone', render: (p?: string) => p || '-' },
    { title: '注册时间', dataIndex: 'created_at', render: (t: string) => dayjs(t).format('YYYY-MM-DD HH:mm') },
    {
      title: '操作',
      render: (_: unknown, r: UserInfo) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => navigate(`/users/${r.id}`)}>
          详情
        </Button>
      ),
    },
  ];

  return (
    <div>
      <div className="page-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end' }}>
        <div>
          <h2>用户列表</h2>
          <p>管理平台注册用户</p>
        </div>
        <Space>
          <Input.Search
            placeholder="搜索邮箱 / 昵称 / 手机号"
            style={{ width: 260 }}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onSearch={(v) => {
              setKeyword(v);
              setPage(1);
              load(1, v);
            }}
            allowClear
          />
        </Space>
      </div>
      <Card>
        <Table
          rowKey="id"
          loading={loading}
          columns={columns}
          dataSource={items}
          scroll={{ x: 1000 }}
          pagination={{
            current: page,
            total,
            pageSize: size,
            onChange: (p) => {
              setPage(p);
              load(p, keyword);
            },
          }}
        />
      </Card>
    </div>
  );
}
