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

  // ЭКРАН УСПЕШНОГО РЕЗУЛЬТАТА
  if (result) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-50 p-4">
        <div className="bg-white p-8 rounded-2xl shadow-xl border border-slate-100 max-w-lg w-full text-center">
          <div className="w-16 h-16 bg-emerald-50 text-emerald-600 rounded-full flex items-center justify-center text-3xl mx-auto mb-4 border border-emerald-100 animate-bounce">
            ✓
          </div>
          <h1 className="text-2xl font-bold text-slate-900 mb-2">Настройка завершена!</h1>
          <p className="text-sm text-slate-500 mb-6">Параметры диспетчерской успешно применены</p>
          
          {result.admin_username && (
            <div className="bg-slate-900 text-slate-100 p-5 rounded-xl mb-6 text-left border border-slate-800 relative overflow-hidden font-mono text-xs sm:text-sm space-y-3 shadow-inner">
              <div className="absolute top-0 right-0 bg-violet-600/20 text-violet-400 px-3 py-1 text-[10px] uppercase font-bold rounded-bl-lg font-sans">
                Секретные ключи
              </div>
              
              <div className="space-y-1.5 pt-2">
                <p className="flex justify-between border-b border-slate-800/60 pb-1.5">
                  <span className="text-slate-500 font-sans">Логин:</span> 
                  <span className="text-emerald-400 font-bold">{result.admin_username}</span>
                </p>
                <p className="flex justify-between border-b border-slate-800/60 pb-1.5">
                  <span className="text-slate-500 font-sans">Пароль:</span> 
                  <span className="text-emerald-400 font-bold">{result.admin_password}</span>
                </p>
                <p className="flex flex-col gap-0.5 border-b border-slate-800/60 pb-1.5">
                  <span className="text-slate-500 font-sans">API Key:</span> 
                  <span className="text-slate-300 break-all bg-slate-950 p-1.5 rounded text-xs mt-1 border border-slate-800">{result.api_key}</span>
                </p>
                <p className="flex flex-col gap-0.5">
                  <span className="text-slate-500 font-sans">Dispatcher ID:</span> 
                  <span className="text-slate-300 break-all bg-slate-950 p-1.5 rounded text-xs mt-1 border border-slate-800">{result.dispatcher_id}</span>
                </p>
              </div>
              
              <p className="text-rose-400 font-sans text-xs mt-4 leading-relaxed font-medium bg-rose-950/30 p-3 rounded-lg border border-rose-900/30">
                ⚠️ Важно: Обязательно сохраните эти данные прямо сейчас! Они показываются только один раз. Приложение автоматически перезапускается.
              </p>
            </div>
          )}
          
          <button 
            onClick={() => navigate('/login')} 
            className="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium px-6 py-3 rounded-xl transition-all shadow-sm"
          >
            Перейти к авторизации
          </button>
        </div>
      </div>
    );
  }

  // ОСНОВНОЙ ЭКРАН ВВОДА ДАННЫХ
  return (
    <div className="min-h-screen flex items-center justify-center bg-radial from-slate-100 to-slate-200/60 p-4">
      <div className="bg-white p-8 rounded-2xl shadow-xl border border-slate-200/60 max-w-md w-full">
        <div className="text-center mb-6">
          <h1 className="text-2xl font-black text-slate-900 tracking-tight">Setup Wizard</h1>
          <p className="text-sm text-slate-400 mt-1">Первичная конфигурация вашей ИИ-диспетчерской</p>
        </div>

        {/* Переключатель режимов (Tabs) */}
        <div className="flex bg-slate-100 p-1 rounded-xl gap-1 mb-6 border border-slate-200/40">
          <button 
            type="button"
            onClick={() => setMode('register')} 
            className={`flex-1 text-center py-2 text-sm font-semibold rounded-lg transition-all ${
              mode === 'register' 
                ? 'bg-white text-blue-600 shadow-sm' 
                : 'text-slate-500 hover:text-slate-800'
            }`}
          >
            Регистрация
          </button>
          <button 
            type="button"
            onClick={() => setMode('connect')} 
            className={`flex-1 text-center py-2 text-sm font-semibold rounded-lg transition-all ${
              mode === 'connect' 
                ? 'bg-white text-blue-600 shadow-sm' 
                : 'text-slate-500 hover:text-slate-800'
            }`}
          >
            У меня есть ключ
          </button>
        </div>

        {/* Вывод ошибки */}
        {error && (
          <div className="bg-rose-50 border border-rose-200 text-rose-700 p-3 rounded-xl text-sm mb-4 flex items-center gap-2 font-medium">
            <span>❌ Ошибка: {error}</span>
          </div>
        )}

        {/* Форма */}
        <form onSubmit={handleSubmit} className="space-y-4">
          {mode === 'register' ? (
            <>
              <div>
                <label className="block text-xs font-bold text-slate-400 uppercase tracking-wider mb-1.5">Название компании</label>
                <input 
                  type="text" 
                  placeholder="Например, ОАО ТехноПром" 
                  value={companyName}
                  onChange={e => setCompanyName(e.target.value)} 
                  className="w-full bg-slate-50/50 border border-slate-200 text-slate-900 placeholder-slate-400 px-4 py-2.5 rounded-xl text-sm focus:outline-hidden focus:ring-2 focus:ring-blue-500/20 focus:border-blue-600 transition-all" 
                  required 
                />
              </div>
              <div>
                <label className="block text-xs font-bold text-slate-400 uppercase tracking-wider mb-1.5">Адрес электронной почты</label>
                <input 
                  type="email" 
                  placeholder="admin@company.com" 
                  value={email}
                  onChange={e => setEmail(e.target.value)} 
                  className="w-full bg-slate-50/50 border border-slate-200 text-slate-900 placeholder-slate-400 px-4 py-2.5 rounded-xl text-sm focus:outline-hidden focus:ring-2 focus:ring-blue-500/20 focus:border-blue-600 transition-all" 
                  required 
                />
              </div>
            </>
          ) : (
            <>
              <div>
                <label className="block text-xs font-bold text-slate-400 uppercase tracking-wider mb-1.5">API Секретный Ключ</label>
                <input 
                  type="text" 
                  placeholder="Вставьте ваш существующий токен" 
                  value={apiKey}
                  onChange={e => setApiKey(e.target.value)} 
                  className="w-full bg-slate-50/50 border border-slate-200 text-slate-900 font-mono placeholder-slate-400 px-4 py-2.5 rounded-xl text-sm focus:outline-hidden focus:ring-2 focus:ring-blue-500/20 focus:border-blue-600 transition-all" 
                  required 
                />
              </div>
              <div>
                <label className="block text-xs font-bold text-slate-400 uppercase tracking-wider mb-1.5">Идентификатор диспетчера (ID)</label>
                <input 
                  type="text" 
                  placeholder="Укажите уникальный Dispatcher ID" 
                  value={dispatcherID}
                  onChange={e => setDispatcherID(e.target.value)} 
                  className="w-full bg-slate-50/50 border border-slate-200 text-slate-900 font-mono placeholder-slate-400 px-4 py-2.5 rounded-xl text-sm focus:outline-hidden focus:ring-2 focus:ring-blue-500/20 focus:border-blue-600 transition-all" 
                  required 
                />
              </div>
            </>
          )}

          <button 
            type="submit" 
            className="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium px-4 py-3 rounded-xl shadow-xs transition-all mt-2 cursor-pointer"
          >
            {mode === 'register' ? 'Зарегистрировать инстанс' : 'Подключить инфраструктуру'}
          </button>
        </form>
      </div>
    </div>
  );
}