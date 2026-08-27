import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuthStore } from './store/auth';
import ConsoleLayout from './layouts/ConsoleLayout';
import Login from './pages/auth/Login';
import Register from './pages/auth/Register';
import Dashboard from './pages/Dashboard';
import Models from './pages/Models';
import ApiKeys from './pages/ApiKeys';
import Usage from './pages/Usage';
import Billing from './pages/Billing';
import Plans from './pages/Plans';
import TokenPackages from './pages/TokenPackages';
import ResetCoupons from './pages/ResetCoupons';
import Notifications from './pages/Notifications';
import Invoices from './pages/Invoices';
import Credit from './pages/Credit';
import Identity from './pages/Identity';
import Conversations from './pages/Conversations';
import Feedback from './pages/Feedback';
import Settings from './pages/Settings';

function RequireAuth({ children }: { children: React.ReactElement }) {
  const token = useAuthStore((s) => s.token);
  if (!token) return <Navigate to="/login" replace />;
  return children;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />
      <Route
        path="/"
        element={
          <RequireAuth>
            <ConsoleLayout />
          </RequireAuth>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="models" element={<Models />} />
        <Route path="api-keys" element={<ApiKeys />} />
        <Route path="usage" element={<Usage />} />
        <Route path="billing" element={<Billing />} />
        <Route path="plans" element={<Plans />} />
        <Route path="token-packages" element={<TokenPackages />} />
        <Route path="reset-coupons" element={<ResetCoupons />} />
        <Route path="notifications" element={<Notifications />} />
        <Route path="invoices" element={<Invoices />} />
        <Route path="credit" element={<Credit />} />
        <Route path="identity" element={<Identity />} />
        <Route path="conversations" element={<Conversations />} />
        <Route path="feedback" element={<Feedback />} />
        <Route path="settings" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
