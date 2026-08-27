import { createRequest, setTokenKey } from '@mass/shared';

setTokenKey('admin', 'admin_token');

export const request = createRequest({
  app: 'admin',
  onUnauthorized: () => {
    localStorage.removeItem('admin_token');
    localStorage.removeItem('admin_user');
    if (!window.location.pathname.startsWith('/admin/login')) {
      window.location.href = '/admin/login';
    }
  },
});
