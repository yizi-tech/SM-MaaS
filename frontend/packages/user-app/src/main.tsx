import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { App as AntApp, ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import 'antd/dist/reset.css';
import { darkTheme, lightTheme } from '@mass/shared';
import { useThemeStore } from './store/theme';
import App from './App';
import './styles/global.css';
import './styles/auth.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeBridge>
      <BrowserRouter basename="/user">
        <App />
      </BrowserRouter>
    </ThemeBridge>
  </React.StrictMode>,
);

function ThemeBridge({ children }: { children: React.ReactNode }) {
  const mode = useThemeStore((s) => s.mode);
  return (
    <ConfigProvider locale={zhCN} theme={mode === 'dark' ? darkTheme : lightTheme}>
      <AntApp>{children}</AntApp>
    </ConfigProvider>
  );
}
