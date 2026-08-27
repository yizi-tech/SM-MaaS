import axios, { AxiosInstance, AxiosRequestConfig, InternalAxiosRequestConfig } from 'axios';
import type { ApiResponse, PageResult } from './types';

// 请求封装：统一 baseURL、Bearer Token、错误处理与分页结果提取。
// token 的存储 key 由各应用注入（user_token / admin_token），避免共享包耦合业务。

const TOKEN_KEYS: Record<string, string> = {};

export function setTokenKey(app: string, key: string) {
  TOKEN_KEYS[app] = key;
}

export function getToken(app: string): string {
  return localStorage.getItem(TOKEN_KEYS[app] || `${app}_token`) || '';
}

export function setToken(app: string, token: string) {
  localStorage.setItem(TOKEN_KEYS[app] || `${app}_token`, token);
}

export function clearToken(app: string) {
  localStorage.removeItem(TOKEN_KEYS[app] || `${app}_token`);
}

interface RequestInstanceOptions {
  app: string; // token 命名空间，如 'user' | 'admin'
  onUnauthorized?: () => void;
  onError?: (message: string) => void;
}

export function createRequest(opts: RequestInstanceOptions): AxiosInstance {
  const instance = axios.create({
    baseURL: '/api/v1',
    timeout: 30000,
  });

  instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    const token = getToken(opts.app);
    if (token) {
      config.headers = config.headers || {};
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  });

  instance.interceptors.response.use(
    (response) => {
      // 文件下载 / 流式响应直接返回
      if (response.config.responseType === 'blob' || response.config.responseType === 'stream') {
        return response;
      }
      const body = response.data as ApiResponse;
      if (body && typeof body.code === 'number' && body.code !== 0) {
        const msg = body.message || '请求失败';
        opts.onError?.(msg);
        if (body.code === 401) {
          opts.onUnauthorized?.();
        }
        return Promise.reject(new Error(msg));
      }
      return response;
    },
    (error) => {
      const status = error?.response?.status;
      const msg =
        error?.response?.data?.message ||
        (status === 401 ? '登录已过期，请重新登录' : error?.message || '网络错误');
      opts.onError?.(msg);
      if (status === 401) {
        opts.onUnauthorized?.();
      }
      return Promise.reject(error);
    },
  );

  return instance;
}

// 提取 {code,message,data} 中的 data；分页响应统一为 PageResult<T>
export function unwrap<T>(res: { data: ApiResponse<T> }): T {
  return res.data.data as T;
}

export function unwrapPage<T>(res: { data: ApiResponse<PageResult<T>> }): PageResult<T> {
  const d = res.data.data;
  if (!d) return { total: 0, page: 1, size: 20, items: [] };
  return d;
}

export async function requestData<T>(instance: AxiosInstance, config: AxiosRequestConfig): Promise<T> {
  const res = await instance.request<ApiResponse<T>>(config);
  return unwrap(res);
}
