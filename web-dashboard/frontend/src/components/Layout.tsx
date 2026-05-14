import { Outlet, Link, useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';

export default function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  if (!user) return <Outlet />;

  return (
    <div className="min-h-screen flex">
      <aside className="w-64 bg-white border-r p-4 flex flex-col gap-2">
        <h1 className="text-lg font-bold mb-4">Поддержка</h1>
        <span className="text-sm text-gray-500">{user.username} ({user.role})</span>
        <hr />
        <Link to="/" className="hover:text-blue-600">Дашборд</Link>
        <Link to="/queue" className="hover:text-blue-600">Очередь тикетов</Link>
        {user.role === 'admin' && (
          <>
            <hr />
            <span className="text-xs text-gray-400 uppercase">Администрирование</span>
            <Link to="/admin/dashboard" className="hover:text-blue-600">Метрики</Link>
            <Link to="/admin/settings" className="hover:text-blue-600">Настройки</Link>
            <Link to="/admin/agents" className="hover:text-blue-600">Агенты</Link>
          </>
        )}
        <div className="mt-auto">
          <button onClick={() => { logout(); navigate('/login'); }} className="text-red-500 hover:underline">
            Выйти
          </button>
        </div>
      </aside>
      <main className="flex-1 p-6">
        <Outlet />
      </main>
    </div>
  );
}