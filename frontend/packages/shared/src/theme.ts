import { theme as antdTheme } from 'antd';

const fontFamily =
  "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif";

/** 共享基础 token：双主题一致的部分 */
function baseToken(mode: 'light' | 'dark') {
  return {
    colorPrimary: '#2563EB',
    colorInfo: '#2563EB',
    colorSuccess: '#16A34A',
    colorWarning: '#D97706',
    colorError: '#DC2626',
    borderRadius: 8,
    borderRadiusLG: 12,
    controlHeight: 40,
    controlHeightSM: 32,
    fontFamily,
    ...(mode === 'dark'
      ? {
          colorText: '#E6EAF2',
          colorTextSecondary: '#8B98AD',
          colorBorder: 'rgba(148, 163, 184, 0.22)',
          colorBgLayout: '#0B1220',
          colorBgContainer: '#111A2C',
        }
      : {
          colorText: '#172033',
          colorTextSecondary: '#667085',
          colorBorder: '#E4E9F1',
          colorBgLayout: '#F6F8FB',
          colorBgContainer: '#FFFFFF',
        }),
  };
}

/** 浅色主题（默认） */
export const lightTheme = {
  algorithm: antdTheme.defaultAlgorithm,
  token: baseToken('light'),
  components: {
    Layout: {
      headerBg: '#FFFFFF',
      siderBg: '#FFFFFF',
      bodyBg: '#F6F8FB',
    },
    Menu: {
      itemBg: 'transparent',
      subMenuItemBg: 'transparent',
      itemColor: '#5B6B84',
      itemHoverColor: '#2563EB',
      itemHoverBg: 'rgba(37, 99, 235, 0.06)',
      itemSelectedColor: '#2563EB',
      itemSelectedBg: 'rgba(37, 99, 235, 0.10)',
      activeBarBorderWidth: 0,
      itemMarginInline: 12,
      itemBorderRadius: 8,
    },
    Card: { borderRadiusLG: 12 },
    Table: {
      headerBg: '#F8FAFC',
      headerColor: '#475467',
      rowHoverBg: '#F8FBFF',
    },
    Button: { borderRadius: 8, controlHeight: 40, primaryShadow: 'rgba(37,99,235,.30)' },
    Input: { controlHeight: 40 },
    Select: { controlHeight: 40 },
    Modal: { borderRadiusLG: 16 },
  },
};

/** 深色主题 */
export const darkTheme = {
  algorithm: antdTheme.darkAlgorithm,
  token: baseToken('dark'),
  components: {
    Layout: {
      headerBg: '#111A2C',
      siderBg: '#070D19',
      bodyBg: '#0B1220',
    },
    Menu: {
      itemBg: 'transparent',
      subMenuItemBg: 'transparent',
      itemColor: '#8B98AD',
      itemHoverColor: '#93C5FD',
      itemHoverBg: 'rgba(147, 197, 253, 0.08)',
      itemSelectedColor: '#FFFFFF',
      itemSelectedBg: 'rgba(37, 99, 235, 0.35)',
      activeBarBorderWidth: 0,
      itemMarginInline: 12,
      itemBorderRadius: 8,
    },
    Card: { borderRadiusLG: 12 },
    Table: {
      headerBg: '#16213A',
      headerColor: '#A9B5C7',
      rowHoverBg: 'rgba(37, 99, 235, 0.08)',
    },
    Button: { borderRadius: 8, controlHeight: 40 },
    Input: { controlHeight: 40 },
    Select: { controlHeight: 40 },
    Modal: { borderRadiusLG: 16 },
  },
};

export type MassTheme = typeof lightTheme;

/** @deprecated 兼容旧引用，等价于 lightTheme */
export const enterpriseTheme = lightTheme;
