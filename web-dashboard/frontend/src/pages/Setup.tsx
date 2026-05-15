import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/client';

export default function Setup() {
  const [mode, setMode] = useState<'register' | 'connect'>('register');
  const [companyName, setCompanyName] = useState('');
  const [email, setEmail] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [dispatcherID, setDispatcherID] = useState('');
  const [error, setError] = useState('');
  const [result, setResult] = useState<any>(null);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setResult(null);

    try {
      const body = mode === 'register'
        ? { action: 'register', company_name: companyName, email }
        : { action: 'connect', api_key: apiKey, dispatcher_id: dispatcherID };

      const data = await api('/setup', { method: 'POST', body: JSON.stringify(body) });
      setResult(data);
    } catch (err: any) {
      setError(err.message || 'Setup failed');
    }
  };

  if (result) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-100">
        <div className="bg-white p-8 rounded shadow max-w-lg w-full text-center">
          <h1 className="text-2xl font-bold text-green-600 mb-4">✅ Успешно!</h1>
          {result.admin_username && (
            <div className="bg-gray-50 p-4 rounded mb-4 text-left">
              <p><strong>Логин:</strong> {result.admin_username}</p>
              <p><strong>Пароль:</strong> {result.admin_password}</p>
              <p><strong>API Key:</strong> {result.api_key}</p>
              <p className="text-red-500 text-sm mt-2">Сохраните эти данные! Они не будут показаны снова. После этого перезапустите приложение.</p>
            </div>
          )}
          <button onClick={() => navigate('/login')} className="bg-blue-600 text-white px-6 py-2 rounded hover:bg-blue-700">
            Перейти ко входу
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-100">
      <div className="bg-white p-8 rounded shadow max-w-lg w-full">
        <h1 className="text-2xl font-bold mb-6">Настройка диспетчерской</h1>

        <div className="flex gap-4 mb-6">
          <button onClick={() => setMode('register')} className={`flex-1 p-2 rounded ${mode === 'register' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}>
            Регистрация
          </button>
          <button onClick={() => setMode('connect')} className={`flex-1 p-2 rounded ${mode === 'connect' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}>
            У меня есть ключ
          </button>
        </div>

        {error && <p className="text-red-500 mb-4">{error}</p>}

        <form onSubmit={handleSubmit} className="space-y-4">
          {mode === 'register' ? (
            <>
              <input type="text" placeholder="Название компании" value={companyName}
                onChange={e => setCompanyName(e.target.value)} className="w-full border p-2 rounded" required />
              <input type="email" placeholder="Email" value={email}
                onChange={e => setEmail(e.target.value)} className="w-full border p-2 rounded" required />
            </>
          ) : (
            <>
              <input type="text" placeholder="API Key" value={apiKey}
                onChange={e => setApiKey(e.target.value)} className="w-full border p-2 rounded" required />
              <input type="text" placeholder="Dispatcher ID" value={dispatcherID}
                onChange={e => setDispatcherID(e.target.value)} className="w-full border p-2 rounded" required />
            </>
          )}
          <button type="submit" className="w-full bg-blue-600 text-white p-2 rounded hover:bg-blue-700">
            {mode === 'register' ? 'Зарегистрировать' : 'Подключиться'}
          </button>
        </form>
      </div>
    </div>
  );
}