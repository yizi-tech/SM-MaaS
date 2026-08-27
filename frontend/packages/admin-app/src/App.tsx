import { Routes, Route, Navigate } from 'react-router-dom';
import { useAdminAuthStore } from './store/auth';
import AdminLayout from './layouts/AdminLayout';
import Login from './pages/Login';
import Overview from './pages/Overview';
import Users from './pages/Users';
import UserDetail from './pages/UserDetail';
import IdentityVerifications from './pages/IdentityVerifications';
import Orders from './pages/Orders';
import Channels from './pages/Channels';
import PricingGroups from './pages/PricingGroups';
import ModelPrices from './pages/ModelPrices';
import Plans from './pages/Plans';
import Invoices from './pages/Invoices';
import CreditApplications from './pages/CreditApplications';
import CreditCollections from './pages/CreditCollections';
import ResetCoupons from './pages/ResetCoupons';
import Notifications from './pages/Notifications';
import Feedback from './pages/Feedback';
import Conversations from './pages/Conversations';
import SystemConfig from './pages/SystemConfig';

function RequireAdmin({ children }: { children: React.ReactElement }) {
  const { token, user } = useAdminAuthStore();
  if (!token || user?.role !== 'admin') return <Navigate to="/login" replace />;
  return children;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={
          <RequireAdmin>
            <AdminLayout />
          </RequireAdmin>
        }
      >
        <Route index element={<Overview />} />
        <Route path="users" element={<Users />} />
        <Route path="users/:id" element={<UserDetail />} />
        <Route path="identity-verifications" element={<IdentityVerifications />} />
        <Route path="orders" element={<Orders />} />
        <Route path="channels" element={<Channels />} />
        <Route path="pricing-groups" element={<PricingGroups />} />
        <Route path="model-prices" element={<ModelPrices />} />
        <Route path="plans" element={<Plans />} />
        <Route path="invoices" element={<Invoices />} />
        <Route path="credit-applications" element={<CreditApplications />} />
        <Route path="credit-collections" element={<CreditCollections />} />
        <Route path="reset-coupons" element={<ResetCoupons />} />
        <Route path="notifications" element={<Notifications />} />
        <Route path="feedback" element={<Feedback />} />
        <Route path="conversations" element={<Conversations />} />
        <Route path="system-config" element={<SystemConfig />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
