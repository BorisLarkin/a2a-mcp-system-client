import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { api } from '@/api/client';

export default function Users() {
  const queryClient = useQueryClient();
  const [showAdd, setShowAdd] = useState(false);
  const [newUser, setNewUser] = useState({ username: '', password: '', full_name: '', role: 'operator' });

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'users'],
    queryFn: () => api('/admin/users'),
  });

  const addMutation = useMutation({
    mutationFn: (user: any) => api('/admin/users', { method: 'POST', body: JSON.stringify(user) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
      setShowAdd(false);
      setNewUser({ username: '', password: '', full_name: '', role: 'operator' });
    },
  });

  const users = data?.users || [];

  return (
    <div className="max-w-6xl mx-auto">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 tracking-tight">Персонал диспетчерской</h1>
          <p className="text-sm text-slate-500 mt-1">Управление правами доступа, ролями и учетными записями операторов</p>
        </div>
        <button onClick={() => setShowAdd(!showAdd)} className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 shadow-sm shadow-blue-500/10">
          {showAdd ? 'Закрыть форму' : '+ Добавить сотрудника'}
        </button>
      </div>

      {showAdd && (
        <div className="bg-white p-5 rounded-xl border border-slate-200 shadow-sm mb-6 max-w-3xl">
          <h2 className="font-bold text-slate-900 mb-4">Регистрация нового пользователя</h2>
          <div className="grid grid-cols-2 gap-4">
            <input type="text" placeholder="Уникальный Логин (Username)" value={newUser.username}
              onChange={e => setNewUser({ ...newUser, username: e.target.value })} />
            <input type="password" placeholder="Сложный Пароль" value={newUser.password}
              onChange={e => setNewUser({ ...newUser, password: e.target.value })} />
            <input type="text" placeholder="ФИО Оператора" value={newUser.full_name}
              onChange={e => setNewUser({ ...newUser, full_name: e.target.value })} />
            <select value={newUser.role} onChange={e => setNewUser({ ...newUser, role: e.target.value })}>
              <option value="operator">Диспетчер / Оператор</option>
              <option value="admin">Системный Администратор</option>
            </select>
          </div>
          <div className="flex gap-3 mt-4 pt-3 border-t border-slate-100">
            <button onClick={() => addMutation.mutate(newUser)} disabled={addMutation.isPending}
              className="bg-slate-900 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-slate-800">
              {addMutation.isPending ? 'Запись в БД...' : 'Создать аккаунт'}
            </button>
            <button onClick={() => setShowAdd(false)} className="text-slate-500 hover:text-slate-700 text-sm font-medium px-2">Отмена</button>
          </div>
        </div>
      )}

      <div className="bg-white rounded-xl border border-slate-200/80 shadow-sm overflow-hidden">
        <table className="w-full border-collapse">
          <thead>
            <tr className="bg-slate-50 border-b border-slate-200 text-slate-500 text-xs font-bold uppercase tracking-wider">
              <th className="p-4 text-left">ID / Системный Логин</th>
              <th className="p-4 text-left">Полное имя (ФИО)</th>
              <th className="p-4 text-left">Уровень привилегий</th>
              <th className="p-4 text-left">Токен активности</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 text-sm">
            {isLoading ? (
              <tr><td colSpan={4} className="text-center p-6 text-slate-400">Чтение таблицы пользователей...</td></tr>
            ) : users.map((u: any) => (
              <tr key={u.ID} className="hover:bg-slate-50/60">
                <td className="p-4 font-mono font-semibold text-slate-700">{u.Username}</td>
                <td className="p-4 font-medium text-slate-800">{u.FullName || 'Не указано'}</td>
                <td className="p-4">
                  <span className={`px-2 py-0.5 rounded text-xs font-semibold ${
                    u.Role === 'admin' ? 'bg-violet-100 text-violet-800' : 'bg-blue-100 text-blue-800'
                  }`}>
                    {u.Role === 'admin' ? 'Администратор' : 'Оператор'}
                  </span>
                </td>
                <td className="p-4">
                  <span className={`inline-flex items-center gap-1.5 text-xs font-medium ${u.IsActive ? 'text-emerald-700' : 'text-slate-400'}`}>
                    <span className={`w-2 h-2 rounded-full ${u.IsActive ? 'bg-emerald-500' : 'bg-slate-300'}`} />
                    {u.IsActive ? 'Активен' : 'Отключен'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}