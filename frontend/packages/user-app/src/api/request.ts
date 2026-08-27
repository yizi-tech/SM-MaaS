import { createRequest, setTokenKey } from '@mass/shared';

// 用户端请求实例（token 命名空间 user）
setTokenKey('user', 'user_token');

export const request = createRequest({
  app: 'user',
  onUnauthorized: () => {
    localStorage.removeItem('user_token');
    if (!window.location.pathname.startsWith('/user/login')) {
      window.location.href = '/user/login';
    }
  },
});
