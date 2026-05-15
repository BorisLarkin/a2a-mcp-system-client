import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from '@/hooks/useAuth';
import Layout from '@/components/Layout';
import Login from '@/pages/Login';
import Setup from '@/pages/Setup';
import OperatorDashboard from '@/pages/operator/Dashboard';
import Queue from '@/pages/operator/Queue';
import TicketDetail from '@/pages/operator/TicketDetail';
import AdminDashboard from '@/pages/admin/Dashboard';
import Settings from '@/pages/admin/Settings';
import Agents from '@/pages/admin/Agents';

function ProtectedRoute({ children, roles }: { children: React.ReactNode; roles?: string[] }) {
  const { user, isAuthenticated, isLoading, setupRequired } = useAuth();

  if (isLoading) {
    return <div className="flex items-center justify-center min-h-screen">Загрузка...</div>;
  }

  if (setupRequired) {
    return <Navigate to="/setup" />;
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" />;
  }

  if (roles && user && !roles.includes(user.role)) {
    return <Navigate to="/" />;
  }

  return <>{children}</>;
}

function PublicRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, setupRequired } = useAuth();

  if (isLoading) return null;

  if (setupRequired) {
    return <>{children}</>;
  }

  if (isAuthenticated) {
    return <Navigate to="/" />;
  }

  return <>{children}</>;
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/setup" element={<PublicRoute><Setup /></PublicRoute>} />
          <Route path="/login" element={<PublicRoute><Login /></PublicRoute>} />
          <Route path="/" element={<Layout />}>
            <Route index element={<ProtectedRoute roles={['operator', 'admin']}><OperatorDashboard /></ProtectedRoute>} />
            <Route path="queue" element={<ProtectedRoute roles={['operator', 'admin']}><Queue /></ProtectedRoute>} />
            <Route path="tickets/:id" element={<ProtectedRoute roles={['operator', 'admin']}><TicketDetail /></ProtectedRoute>} />
            <Route path="admin/dashboard" element={<ProtectedRoute roles={['admin']}><AdminDashboard /></ProtectedRoute>} />
            <Route path="admin/settings" element={<ProtectedRoute roles={['admin']}><Settings /></ProtectedRoute>} />
            <Route path="admin/agents" element={<ProtectedRoute roles={['admin']}><Agents /></ProtectedRoute>} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}