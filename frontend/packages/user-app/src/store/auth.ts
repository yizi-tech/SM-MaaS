import { create } from 'zustand';
import type { ApiResponse, GuiQuota, GuiSession, SiteConfig, UserInfo } from '@mass/shared';
import { request } from '../api/request';

interface AuthState {
  token: string;
  user: UserInfo | null;
  quota: GuiQuota | null;
  siteName: string;
  siteLogo: string;
  loadSession: () => Promise<void>;
  refreshQuota: () => Promise<void>;
  loadSiteConfig: () => Promise<void>;
  setToken: (token: string, user: UserInfo) => void;
  /** OAuth 回调落地：仅写入 token，用户信息由控制台 loadSession 拉取 */
  adoptToken: (token: string) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: localStorage.getItem('user_token') || '',
  user: null,
  quota: null,
  siteName: 'MASS 平台',
  siteLogo: '',
  setToken: (token, user) => {
    localStorage.setItem('user_token', token);
    set({ token, user });
  },
  adoptToken: (token) => {
    localStorage.setItem('user_token', token);
    set({ token });
  },
  logout: () => {
    localStorage.removeItem('user_token');
    set({ token: '', user: null, quota: null });
  },
  loadSession: async () => {
    try {
      const data = await request.get<ApiResponse<GuiSession>>('/gui/session').then((r) => r.data.data);
      if (!data) return;
      set({ user: data.user, quota: data.quota });
    } catch {
      /* token 无效时交由拦截器处理 */
    }
  },
  refreshQuota: async () => {
    try {
      const quota = await request.get<ApiResponse<GuiQuota>>('/gui/sync').then((r) => r.data.data);
      if (quota) set({ quota });
    } catch {
      /* 静默失败 */
    }
  },
  loadSiteConfig: async () => {
    try {
      const data = await request.get<ApiResponse<SiteConfig>>('/site-config').then((r) => r.data.data);
      if (data) {
        set({
          siteName: data.site_name || get().siteName,
          siteLogo: data.site_logo || '',
        });
        if (data.site_name) document.title = data.site_name;
      }
    } catch {
      /* 使用默认品牌 */
    }
  },
}));
