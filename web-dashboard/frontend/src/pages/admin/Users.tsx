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
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Пользователи</h1>
        <button onClick={() => setShowAdd(true)} className="bg-blue-600 text-white px-4 py-2 rounded-lg hover:bg-blue-700">
          + Добавить оператора
        </button>
      </div>

      {showAdd && (
        <div className="bg-white p-4 rounded-lg shadow mb-4">
          <h2 className="font-bold mb-3">Новый пользователь</h2>
          <div className="grid grid-cols-2 gap-3">
            <input type="text" placeholder="Логин" value={newUser.username}
              onChange={e => setNewUser({ ...newUser, username: e.target.value })}
              className="border p-2 rounded" />
            <input type="password" placeholder="Пароль" value={newUser.password}
              onChange={e => setNewUser({ ...newUser, password: e.target.value })}
              className="border p-2 rounded" />
            <input type="text" placeholder="Полное имя" value={newUser.full_name}
              onChange={e => setNewUser({ ...newUser, full_name: e.target.value })}
              className="border p-2 rounded" />
            <select value={newUser.role} onChange={e => setNewUser({ ...newUser, role: e.target.value })}
              className="border p-2 rounded">
              <option value="operator">Оператор</option>
              <option value="admin">Администратор</option>
            </select>
          </div>
          <div className="flex gap-2 mt-3">
            <button onClick={() => addMutation.mutate(newUser)} disabled={addMutation.isPending}
              className="bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700">
              {addMutation.isPending ? 'Создание...' : 'Создать'}
            </button>
            <button onClick={() => setShowAdd(false)} className="text-gray-500 hover:underline">Отмена</button>
          </div>
        </div>
      )}

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Логин</th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Имя</th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Роль</th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Активен</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u: any) => (
              <tr key={u.ID} className="border-t">
                <td className="p-3 text-sm">{u.Username}</td>
                <td className="p-3 text-sm">{u.FullName || '—'}</td>
                <td className="p-3 text-sm">{u.Role === 'admin' ? 'Админ' : 'Оператор'}</td>
                <td className="p-3 text-sm">{u.IsActive ? '✅' : '❌'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}