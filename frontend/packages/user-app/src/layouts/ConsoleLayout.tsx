import { useEffect, useState } from 'react';
import { Layout, Menu, Dropdown, Badge, Avatar, Spin, Tooltip, Button, Drawer } from 'antd';
import {
  DashboardOutlined,
  RobotOutlined,
  KeyOutlined,
  LineChartOutlined,
  CreditCardOutlined,
  ProfileOutlined,
  GiftOutlined,
  RedoOutlined,
  BellOutlined,
  FileTextOutlined,
  AccountBookOutlined,
  IdcardOutlined,
  MessageOutlined,
  CommentOutlined,
  SettingOutlined,
  LogoutOutlined,
  UserOutlined,
  SunOutlined,
  MoonOutlined,
  MenuOutlined,
} from '@ant-design/icons';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/auth';
import { useThemeStore } from '../store/theme';
import { useIsMobile } from '../hooks/useIsMobile';

const { Sider, Header, Content } = Layout;

const menuGroups = [
  {
    key: 'workspace',
    label: '开发工作台',
    items: [
      { key: '/', icon: <DashboardOutlined />, label: '控制台总览' },
      { key: '/models', icon: <RobotOutlined />, label: '模型市场' },
      { key: '/api-keys', icon: <KeyOutlined />, label: 'API Keys' },
      { key: '/usage', icon: <LineChartOutlined />, label: '用量与账单' },
      { key: '/conversations', icon: <MessageOutlined />, label: '对话记录' },
    ],
  },
  {
    key: 'billing',
    label: '资源与计费',
    items: [
      { key: '/plans', icon: <ProfileOutlined />, label: '套餐订阅' },
      { key: '/billing', icon: <CreditCardOutlined />, label: '充值中心' },
      { key: '/token-packages', icon: <GiftOutlined />, label: 'Token 加油包' },
      { key: '/reset-coupons', icon: <RedoOutlined />, label: '重置券' },
      { key: '/credit', icon: <AccountBookOutlined />, label: 'Token 授信' },
      { key: '/invoices', icon: <FileTextOutlined />, label: '发票管理' },
    ],
  },
  {
    key: 'account',
    label: '账户与支持',
    items: [
      { key: '/identity', icon: <IdcardOutlined />, label: '实名认证' },
      { key: '/feedback', icon: <CommentOutlined />, label: '问题反馈' },
      { key: '/settings', icon: <SettingOutlined />, label: '个人设置' },
    ],
  },
];

export default function ConsoleLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, quota, logout, loadSession, refreshQuota, loadSiteConfig, siteName, siteLogo } =
    useAuthStore();
  const { mode, toggle } = useThemeStore();
  const isMobile = useIsMobile();
  const [drawerOpen, setDrawerOpen] = useState(false);

  useEffect(() => {
    loadSiteConfig();
    if (!user) loadSession();
    // 额度毫秒级同步（GUI 联动语义）：进入控制台后每 30s 刷新一次
    const timer = setInterval(refreshQuota, 30000);
    return () => clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!user) {
    return (
      <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" />
      </div>
    );
  }

  const selectedKey = '/' + (location.pathname.split('/')[1] || '');
  const unread = 0; // 未读数由通知页拉取

  const userMenu = {
    items: [
      { key: 'settings', icon: <SettingOutlined />, label: '个人设置' },
      { type: 'divider' as const },
      { key: 'logout', icon: <LogoutOutlined />, label: '退出登录' },
    ],
    onClick: ({ key }: { key: string }) => {
      if (key === 'logout') {
        logout();
        navigate('/login');
      } else if (key === 'settings') {
        navigate('/settings');
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
        <div className="sider-subtitle">开发者控制台</div>
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
          <Badge count={unread} size="small">
            <BellOutlined
              className="header-icon"
              onClick={() => navigate('/notifications')}
            />
          </Badge>
          <Dropdown menu={userMenu} placement="bottomRight">
            <div className="header-user">
              <Avatar size={32} src={user.avatar} icon={<UserOutlined />} />
              <div style={{ lineHeight: 1.2 }}>
                <div className="header-user-name">{user.nickname}</div>
                <div className="header-user-quota">余额 {quota?.balance ?? '¥0.00'}</div>
              </div>
            </div>
          </Dropdown>
        </Header>
        <Content className="page-content" style={{ padding: 24, maxWidth: 1200, margin: '0 auto', width: '100%' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
