import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '@/api/client';

const STATUS_LABELS: Record<string, string> = {
  new: 'Новый',
  in_progress: 'В работе',
  waiting: 'Ожидает',
  waiting_for_feedback: 'Ждёт ответа',
  resolved: 'Решён',
  closed: 'Закрыт',
};

const STATUS_COLORS: Record<string, string> = {
  new: 'bg-blue-100 text-blue-700',
  in_progress: 'bg-purple-100 text-purple-700',
  waiting: 'bg-yellow-100 text-yellow-700',
  waiting_for_feedback: 'bg-green-100 text-green-700',
  resolved: 'bg-gray-100 text-gray-700',
  closed: 'bg-red-100 text-red-700',
};

export default function Queue() {
  const [searchParams] = useSearchParams();
  const statusFilter = searchParams.get('status') || 'new,waiting,in_progress,waiting_for_feedback';

  const { data, isLoading } = useQuery({
    queryKey: ['tickets', 'active', statusFilter],
    queryFn: () => api(`/tickets?status=${statusFilter}&limit=50`),
    refetchInterval: 10000,
  });

  if (isLoading) return <p className="p-6">Загрузка...</p>;

  const tickets = data?.tickets || [];

  return (
    <div>
      <h1 className="text-2xl font-bold mb-4">Очередь тикетов</h1>
      
      <div className="flex gap-2 mb-4">
        {['new,waiting,in_progress,waiting_for_feedback', 'waiting', 'waiting_for_feedback', 'resolved'].map(filter => (
          <Link
            key={filter}
            to={`/queue?status=${filter}`}
            className={`px-3 py-1 rounded text-sm ${
              statusFilter === filter ? 'bg-blue-600 text-white' : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
            }`}
          >
            {filter === 'new,waiting,in_progress,waiting_for_feedback' ? 'Все активные' :
             filter === 'waiting' ? 'Ожидают' :
             filter === 'waiting_for_feedback' ? 'Ждут ответа' : 'Решённые'}
          </Link>
        ))}
      </div>

      {tickets.length === 0 ? (
        <p className="text-gray-500 p-6 text-center bg-white rounded-lg shadow">Нет тикетов</p>
      ) : (
        <div className="bg-white rounded-lg shadow overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="p-3 text-left text-sm font-medium text-gray-500">ID</th>
                <th className="p-3 text-left text-sm font-medium text-gray-500">Клиент</th>
                <th className="p-3 text-left text-sm font-medium text-gray-500">Текст</th>
                <th className="p-3 text-left text-sm font-medium text-gray-500">Категория</th>
                <th className="p-3 text-left text-sm font-medium text-gray-500">Статус</th>
                <th className="p-3 text-left text-sm font-medium text-gray-500">Приоритет</th>
              </tr>
            </thead>
            <tbody>
              {tickets.map((t: any) => (
                <tr key={t.ID} className="border-t hover:bg-gray-50">
                  <td className="p-3">
                    <Link to={`/tickets/${t.ID}`} className="text-blue-600 hover:underline font-mono text-sm">
                      {t.ID?.slice(0, 8)}
                    </Link>
                  </td>
                  <td className="p-3 text-sm">{t.Client?.Name || '—'}</td>
                  <td className="p-3 max-w-xs truncate text-sm">{t.OriginalText}</td>
                  <td className="p-3 text-sm">{t.Category || '—'}</td>
                  <td className="p-3">
                    <span className={`px-2 py-1 rounded text-xs ${STATUS_COLORS[t.Status] || 'bg-gray-100'}`}>
                      {STATUS_LABELS[t.Status] || t.Status}
                    </span>
                  </td>
                  <td className="p-3 text-sm">{t.Priority}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}