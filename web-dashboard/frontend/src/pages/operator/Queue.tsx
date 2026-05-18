import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '@/api/client';
import { useWebSocket } from '@/hooks/useWebSocket';

const STATUS_LABELS: Record<string, string> = {
  new: 'Новый',
  in_progress: 'В работе',
  waiting: 'Ожидает',
  waiting_for_feedback: 'Ждёт ответа',
  resolved: 'Решён',
  closed: 'Закрыт',
};

// используем аккуратные пиллы с границами:
const STATUS_COLORS: Record<string, string> = {
  new: 'bg-blue-50 text-blue-700 border border-blue-100',
  in_progress: 'bg-amber-50 text-amber-700 border border-amber-100',
  waiting: 'bg-rose-50 text-rose-700 border border-rose-100',
  waiting_for_feedback: 'bg-emerald-50 text-emerald-700 border border-emerald-100',
  resolved: 'bg-slate-100 text-slate-700 border border-slate-200',
  closed: 'bg-slate-200 text-slate-800',
};

export default function Queue() {
  const [searchParams, setSearchParams] = useSearchParams();
  const statusFilter = searchParams.get('status') || 'new,waiting,in_progress,waiting_for_feedback';
  const queryClient = useQueryClient();

  // Текущий выбранный статус из URL. Если его нет, по умолчанию показываем активные тикеты
  const currentStatus = searchParams.get('status') || 'active';

  useWebSocket((data) => {
      if (data.type === 'ticket_created' || data.type === 'ticket_updated' || data.type === 'new_escalated') {
          queryClient.invalidateQueries({ queryKey: ['tickets', 'active'] });
      }
  });

  // Преобразуем фильтр для отправки на бэкенд
  let apiStatusParam = currentStatus;
  if (currentStatus === 'active') {
    apiStatusParam = 'new,waiting,in_progress,waiting_for_feedback';
  } else if (currentStatus === 'all') {
    apiStatusParam = ''; // Пустой параметр вернет абсолютно все тикеты с бэкенда
  }

  const { data, isLoading } = useQuery({
    queryKey: ['tickets', 'queue', apiStatusParam],
    queryFn: () => api(`/tickets?status=${apiStatusParam}&limit=100`),
  });

  if (isLoading) return <p className="p-6">Загрузка...</p>;

  const tickets = data?.tickets || [];

  // Список табов для фильтрации
  const tabs = [
    { id: 'active', label: '⚡ Активные' },
    { id: 'all', label: '📁 Все тикеты' },
    { id: 'new', label: '📩 Новые' },
    { id: 'in_progress', label: '⚙️ В работе' },
    { id: 'resolved', label: '✅ Решенные' },
    { id: 'closed', label: '🔒 Закрытые' },
  ];

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <h1 className="text-2xl font-bold text-slate-900">Очередь обращений</h1>
        
        {/* Горизонтальная панель фильтров (Табы) */}
        <div className="flex flex-wrap gap-1.5 bg-slate-100 p-1 rounded-xl border border-slate-200/40">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setSearchParams({ status: tab.id })}
              className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-all duration-150 ${
                currentStatus === tab.id
                  ? 'bg-white text-slate-900 shadow-sm'
                  : 'text-slate-600 hover:text-slate-900 hover:bg-white/50'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

    {/* Таблица */}
    <div className="bg-white rounded-xl shadow-sm border border-slate-200/80 overflow-hidden">
      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-slate-50 border-b border-slate-200 text-slate-600">
            <th className="p-4 text-left text-xs font-semibold uppercase tracking-wider">ID</th>
            <th className="p-4 text-left text-xs font-semibold uppercase tracking-wider">Клиент</th>
            <th className="p-4 text-left text-xs font-semibold uppercase tracking-wider">Текст обращения</th>
            <th className="p-4 text-left text-xs font-semibold uppercase tracking-wider">AI Категория</th>
            <th className="p-4 text-left text-xs font-semibold uppercase tracking-wider">Статус</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {tickets.map((t: any) => (
            <tr key={t.ID} className="hover:bg-slate-50/80">
              <td className="p-4">
                <Link to={`/tickets/${t.ID}`} className="text-blue-600 hover:text-blue-700 font-mono text-sm font-medium">
                  #{t.ID?.slice(0, 8)}
                </Link>
              </td>
              <td className="p-4 text-sm font-medium text-slate-700">{t.Client?.Name || '—'}</td>
              <td className="p-4 text-sm text-slate-600 max-w-xs truncate">{t.OriginalText}</td>
              <td className="p-4 text-sm">
                <span className="bg-violet-50 text-violet-700 text-xs px-2 py-1 rounded-md font-medium border border-violet-100/60">
                  {t.Category || 'Определяется...'}
                </span>
              </td>
              <td className="p-4">
                <span className={`px-2.5 py-1 rounded-full text-xs font-medium ${STATUS_COLORS[t.Status] || 'bg-slate-100'}`}>
                  {STATUS_LABELS[t.Status] || t.Status}
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