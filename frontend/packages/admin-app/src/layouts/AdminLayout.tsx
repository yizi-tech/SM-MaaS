import { useEffect, useState } from 'react';
import { Layout, Menu, Dropdown, Avatar, Tooltip, Button, Drawer } from 'antd';
import {
  DashboardOutlined,
  UserOutlined,
  TeamOutlined,
  IdcardOutlined,
  FileTextOutlined,
  ApiOutlined,
  TagsOutlined,
  DollarOutlined,
  ProfileOutlined,
  AccountBookOutlined,
  ExclamationCircleOutlined,
  RedoOutlined,
  BellOutlined,
  CommentOutlined,
  MessageOutlined,
  SettingOutlined,
  LogoutOutlined,
  SunOutlined,
  MoonOutlined,
  MenuOutlined,
} from '@ant-design/icons';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useAdminAuthStore } from '../store/auth';
import { useThemeStore } from '../store/theme';
import { useIsMobile } from '../hooks/useIsMobile';

const { Sider, Header, Content } = Layout;

const menuGroups = [
  {
    key: 'operations',
    label: '运营工作台',
    items: [
      { key: '/', icon: <DashboardOutlined />, label: '数据概览' },
      { key: '/users', icon: <TeamOutlined />, label: '用户列表' },
      { key: '/identity-verifications', icon: <IdcardOutlined />, label: '实名认证审核' },
    ],
  },
  {
    key: 'finance',
    label: '财务与风控',
    items: [
      { key: '/orders', icon: <FileTextOutlined />, label: '订单管理' },
      { key: '/invoices', icon: <DollarOutlined />, label: '发票审核' },
      { key: '/credit-applications', icon: <AccountBookOutlined />, label: '授信申请' },
      { key: '/credit-collections', icon: <ExclamationCircleOutlined />, label: '催账管理' },
      { key: '/reset-coupons', icon: <RedoOutlined />, label: '重置券发放' },
    ],
  },
  {
    key: 'gateway',
    label: '网关与商业化',
    items: [
      { key: '/channels', icon: <ApiOutlined />, label: 'LLM 渠道' },
      { key: '/pricing-groups', icon: <TagsOutlined />, label: '定价分组' },
      { key: '/model-prices', icon: <DollarOutlined />, label: '模型价格表' },
      { key: '/plans', icon: <ProfileOutlined />, label: '套餐管理' },
    ],
  },
  {
    key: 'service',
    label: '服务运营',
    items: [
      { key: '/notifications', icon: <BellOutlined />, label: '通知管理' },
      { key: '/feedback', icon: <CommentOutlined />, label: '反馈处理' },
      { key: '/conversations', icon: <MessageOutlined />, label: '对话记录' },
      { key: '/system-config', icon: <SettingOutlined />, label: '系统配置' },
    ],
  },
];

export default function AdminLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, logout, loadSiteConfig, siteName, siteLogo } = useAdminAuthStore();
  const { mode, toggle } = useThemeStore();
  const isMobile = useIsMobile();
  const [drawerOpen, setDrawerOpen] = useState(false);

  useEffect(() => {
    loadSiteConfig();
  }, [loadSiteConfig]);

  const path = location.pathname.replace(/\/+$/, '') || '/';
  const selectedKey = menuGroups.flatMap((group) => group.items).some((item) => item.key === path)
    ? path
    : path.split('/').slice(0, 2).join('/') || '/';

  const userMenu = {
    items: [{ key: 'logout', icon: <LogoutOutlined />, label: '退出登录' }],
    onClick: ({ key }: { key: string }) => {
      if (key === 'logout') {
        logout();
        navigate('/login');
      }
    },
  };

  const brand = (
    <>
      {siteLogo ? (
        <img src={siteLogo} alt="logo" className="sider-logo" />
      ) : (
        <div className="sider-logo logo-fallback">M</div>
      )}
      <div style={{ minWidth: 0 }}>
        <div className="sider-title">{siteName}</div>
        <div className="sider-subtitle">运营控制台</div>
      </div>
    </>
  );

  const themeSwitch = (
    <button type="button" className="theme-switch" onClick={toggle}>
      <span className="theme-switch-track" data-mode={mode}>
        <SunOutlined className="theme-icon sun" />
        <MoonOutlined className="theme-icon moon" />
        <span className="theme-switch-thumb" />
      </span>
      <span className="theme-switch-label">{mode === 'dark' ? '深色模式' : '浅色模式'}</span>
    </button>
  );

  const renderGroups = (onClick: (key: string) => void) =>
    menuGroups.map((group) => (
      <div key={group.key} style={{ marginBottom: 10 }}>
        <div className="sider-group-label">{group.label}</div>
        <Menu
          mode="inline"
          selectedKeys={[selectedKey]}
          items={group.items}
          onClick={({ key }) => onClick(key)}
          style={{ border: 'none', background: 'transparent' }}
        />
      </div>
    ));

  return (
    <Layout style={{ minHeight: '100vh' }}>
      {!isMobile && (
        <Sider
          width={240}
          collapsedWidth={0}
          trigger={null}
          className="console-sider"
          style={{ position: 'fixed', left: 0, top: 0, bottom: 0, zIndex: 20 }}
        >
          <div className="sider-brand">{brand}</div>
          <div className="sider-divider" />
          <div className="sider-menu-scroll">{renderGroups((key) => navigate(key))}</div>
          <div className="sider-footer">{themeSwitch}</div>
        </Sider>
      )}

      {isMobile && (
        <Drawer
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          placement="left"
          width={280}
          className="mobile-drawer"
          styles={{ body: { padding: 0, display: 'flex', flexDirection: 'column' } }}
        >
          <div className="sider-brand">{brand}</div>
          <div className="sider-divider" />
          <div className="sider-menu-scroll">{renderGroups((key) => { navigate(key); setDrawerOpen(false); })}</div>
          <div className="sider-footer">{themeSwitch}</div>
        </Drawer>
      )}

      <Layout style={{ marginLeft: isMobile ? 0 : 240 }}>
        <Header className="console-header">
          {isMobile && (
            <Button
              type="text"
              aria-label="打开菜单"
              icon={<MenuOutlined />}
              onClick={() => setDrawerOpen(true)}
              style={{ fontSize: 18, marginRight: 4 }}
            />
          )}
          <Tooltip title={mode === 'dark' ? '切换为浅色' : '切换为深色'}>
            <Button
              type="text"
              aria-label="切换主题"
              icon={mode === 'dark' ? <SunOutlined /> : <MoonOutlined />}
              onClick={toggle}
              style={{ fontSize: 17, marginLeft: isMobile ? 'auto' : undefined }}
            />
          </Tooltip>
          <Dropdown menu={userMenu} placement="bottomRight">
            <div className="header-user">
              <Avatar size={30} src={user?.avatar} icon={<UserOutlined />} />
              <span className="header-user-name">{user?.nickname}</span>
            </div>
          </Dropdown>
        </Header>
        <Content className="page-content" style={{ padding: 24, maxWidth: 1400, margin: '0 auto', width: '100%' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
