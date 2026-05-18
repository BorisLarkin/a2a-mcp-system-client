import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    try {
      await login(username, password);
      navigate('/');
    } catch {
      setError('Неверный логин или пароль');
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-slate-50">
      <div className="bg-white p-8 rounded-2xl border border-slate-200/60 shadow-xl shadow-slate-200/30 w-full max-w-md">
        <div className="text-center mb-6">
          <div className="bg-blue-600 text-white p-2.5 rounded-xl inline-block shadow-md shadow-blue-500/20 mb-3">
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </div>
          <h1 className="text-xl font-bold text-slate-900">Вход в систему управления</h1>
          <p className="text-sm text-slate-400 mt-1">Многоагентная платформа A2A/MCP</p>
        </div>
    
        {error && <p className="bg-red-50 text-red-600 text-xs p-3 rounded-lg border border-red-100 mb-4 font-medium">{error}</p>}
        
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <input
              type="text" placeholder="Имя пользователя" value={username}
              onChange={e => setUsername(e.target.value)}
              className="w-full" required
            />
          </div>
          <div>
            <input
              type="password" placeholder="Пароль" value={password}
              onChange={e => setPassword(e.target.value)}
              className="w-full" required
            />
          </div>
          <button type="submit" className="w-full bg-blue-600 text-white font-medium p-2.5 rounded-lg hover:bg-blue-700 shadow-md shadow-blue-500/10 mt-2">
            Войти в панель
          </button>
        </form>
      </div>
    </div>
  );
}