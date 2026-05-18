import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { api } from '@/api/client';
import { useWebSocket } from '@/hooks/useWebSocket';

export default function OperatorDashboard() {
  const queryClient = useQueryClient();
    
  useWebSocket((data) => {
    if (data.type === 'ticket_created' || data.type === 'ticket_updated' || data.type === 'new_escalated') {
      queryClient.invalidateQueries({ queryKey: ['tickets'] });
    }
  });

  const { data: allTickets } = useQuery({
    queryKey: ['tickets', 'all'],
    queryFn: () => api('/tickets?limit=100'),
  });

  const tickets = allTickets?.tickets || [];

  const stats = {
    waiting: tickets.filter((t: any) => t.Status === 'waiting' || t.Status === 'new').length,
    in_progress: tickets.filter((t: any) => t.Status === 'in_progress').length,
    waiting_for_feedback: tickets.filter((t: any) => t.Status === 'waiting_for_feedback').length,
    resolved_today: tickets.filter((t: any) => t.Status === 'resolved').length,
  };

  return (
    <div className="max-w-7xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900 tracking-tight">Рабочее место оператора</h1>
        <p className="text-sm text-slate-500 mt-1">Контроль инцидентов и валидация ответов LLM в реальном времени</p>
      </div>
      
      {/* Метрики */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <div className="metric-card border-l-4 border-l-amber-500">
          <p className="text-slate-400 text-xs font-bold uppercase tracking-wider">Ожидают реакции</p>
          <p className="text-3xl font-black text-amber-600 mt-1">{stats.waiting}</p>
        </div>
        <div className="metric-card border-l-4 border-l-violet-500">
          <p className="text-slate-400 text-xs font-bold uppercase tracking-wider">В обработке человеком</p>
          <p className="text-3xl font-black text-violet-600 mt-1">{stats.in_progress}</p>
        </div>
        <div className="metric-card border-l-4 border-l-emerald-500">
          <p className="text-slate-400 text-xs font-bold uppercase tracking-wider">Ожидание фидбека</p>
          <p className="text-3xl font-black text-emerald-600 mt-1">{stats.waiting_for_feedback}</p>
        </div>
        <div className="metric-card border-l-4 border-l-slate-400">
          <p className="text-slate-400 text-xs font-bold uppercase tracking-wider">Автозакрыто (Успешно)</p>
          <p className="text-3xl font-black text-slate-700 mt-1">{stats.resolved_today}</p>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Последние тикеты */}
        <div className="bg-white p-5 rounded-xl border border-slate-200/80 shadow-sm lg:col-span-2">
          <h2 className="font-bold text-base text-slate-900 mb-4">Журнал последних поступлений</h2>
          <div className="divide-y divide-slate-100">
            {tickets.slice(0, 6).map((t: any) => (
              <div key={t.ID} className="py-3 flex justify-between items-center first:pt-0 last:pb-0">
                <div className="flex items-center gap-3 truncate mr-4">
                  <Link to={`/tickets/${t.ID}`} className="text-blue-600 hover:text-blue-700 font-mono text-xs font-bold shrink-0">
                    #{t.ID?.slice(0, 8)}
                  </Link>
                  <span className="text-slate-600 text-sm truncate font-medium">{t.OriginalText}</span>
                </div>
                <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wide shrink-0 ${
                  t.Status === 'waiting' || t.Status === 'new' ? 'bg-amber-50 text-amber-700 border border-amber-100' :
                  t.Status === 'waiting_for_feedback' ? 'bg-emerald-50 text-emerald-700 border border-emerald-100' :
                  'bg-slate-100 text-slate-600'
                }`}>{t.Status}</span>
              </div>
            ))}
          </div>
          <Link to="/queue" className="text-blue-600 hover:text-blue-700 text-xs font-bold mt-4 inline-block">
            Открыть полную очередь тикетов &rarr;
          </Link>
        </div>

        {/* Быстрые действия */}
        <div className="bg-white p-5 rounded-xl border border-slate-200/80 shadow-sm h-max">
          <h2 className="font-bold text-base text-slate-900 mb-4">Умные фильтры очереди</h2>
          <div className="space-y-2.5">
            <Link to="/queue?status=waiting" className="flex justify-between items-center p-3 bg-amber-50/50 hover:bg-amber-50 rounded-xl text-sm font-medium text-amber-900 border border-amber-100/60 transition-all">
              <span>🔴 Критическая эскалация</span>
              <span className="bg-amber-600 text-white font-mono text-xs px-2 py-0.5 rounded-md font-bold">{stats.waiting}</span>
            </Link>
            <Link to="/queue?status=waiting_for_feedback" className="flex justify-between items-center p-3 bg-emerald-50/50 hover:bg-emerald-50 rounded-xl text-sm font-medium text-emerald-900 border border-emerald-100/60 transition-all">
              <span>🟢 Ждут ответа абонента</span>
              <span className="bg-emerald-600 text-white font-mono text-xs px-2 py-0.5 rounded-md font-bold">{stats.waiting_for_feedback}</span>
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}