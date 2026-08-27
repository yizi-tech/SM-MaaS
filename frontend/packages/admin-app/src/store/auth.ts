import { create } from 'zustand';
import type { ApiResponse, SiteConfig, UserInfo } from '@mass/shared';
import { request } from '../api/request';

interface AdminAuthState {
  token: string;
  user: UserInfo | null;
  siteName: string;
  siteLogo: string;
  setLogin: (token: string, user: UserInfo) => void;
  logout: () => void;
  loadSiteConfig: () => Promise<void>;
}

export const useAdminAuthStore = create<AdminAuthState>((set) => ({
  token: localStorage.getItem('admin_token') || '',
  user: (() => {
    try {
      return JSON.parse(localStorage.getItem('admin_user') || 'null');
    } catch {
      return null;
    }
  })(),
  siteName: 'MASS 管理后台',
  siteLogo: '',
  setLogin: (token, user) => {
    localStorage.setItem('admin_token', token);
    localStorage.setItem('admin_user', JSON.stringify(user));
    set({ token, user });
  },
  logout: () => {
    localStorage.removeItem('admin_token');
    localStorage.removeItem('admin_user');
    set({ token: '', user: null });
  },
  loadSiteConfig: async () => {
    try {
      const data = await request.get<ApiResponse<SiteConfig>>('/site-config').then((r) => r.data.data);
      if (data?.site_name) {
        set({
          siteName: `${data.site_name} · 管理后台`,
          siteLogo: data.site_logo || '',
        });
        document.title = `${data.site_name} · 管理后台`;
      }
    } catch {
      /* 默认品牌 */
    }
  },
}));
