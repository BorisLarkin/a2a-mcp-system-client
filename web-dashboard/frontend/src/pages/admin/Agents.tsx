import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';

export default function Agents() {
  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'channels'],
    queryFn: () => api('/admin/channels'),
  });

  // Показываем каналы как «агентов» — в будущем заменим на реальный список
  const channels = data?.channels || [];

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Управление агентами и каналами</h1>

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50">
            <tr>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Название</th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Тип</th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Активен</th>
              <th className="p-3 text-left text-sm font-medium text-gray-500">Создан</th>
            </tr>
          </thead>
          <tbody>
            {channels.map((ch: any) => (
              <tr key={ch.ID} className="border-t hover:bg-gray-50">
                <td className="p-3 text-sm">{ch.Name}</td>
                <td className="p-3 text-sm">{ch.Type}</td>
                <td className="p-3 text-sm">{ch.IsActive ? '✅' : '❌'}</td>
                <td className="p-3 text-sm">{new Date(ch.CreatedAt).toLocaleDateString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <p className="text-gray-500 text-sm mt-4">
        Регистрация новых AI-агентов будет доступна в следующей версии.
      </p>
    </div>
  );
}