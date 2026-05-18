import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';

const COLORS = ['#2563eb', '#f59e0b', '#7c3aed', '#10b981', '#ef4444'];

export default function AdminDashboard() {
  const { data: ticketsData } = useQuery({
    queryKey: ['tickets', 'all'],
    queryFn: () => api('/tickets?limit=1000'),
  });

  const { data: channelsData } = useQuery({
    queryKey: ['admin', 'channels'],
    queryFn: () => api('/admin/channels'),
  });

  const tickets = ticketsData?.tickets || [];
  const channels = channelsData?.channels || [];

  // Подсчёт по категориям
  const categoryCounts: Record<string, number> = {};
  tickets.forEach((t: any) => {
    const cat = t.Category || 'без категории';
    categoryCounts[cat] = (categoryCounts[cat] || 0) + 1;
  });
  const categoryData = Object.entries(categoryCounts).map(([name, value]) => ({ name, value }));

  // Подсчёт по статусам
  const statusCounts: Record<string, number> = {};
  tickets.forEach((t: any) => {
    statusCounts[t.Status] = (statusCounts[t.Status] || 0) + 1;
  });
  const statusData = Object.entries(statusCounts).map(([name, value]) => ({ name, value }));

  return (
    <div className="max-w-7xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900 tracking-tight">Глобальная BI-Аналитика</h1>
        <p className="text-sm text-slate-500 mt-1">Оценка эффективности классификации, уровня автоматизации процессов и нагрузки на каналы</p>
      </div>

      {/* Верхние Счётчики */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-5 mb-8">
        <div className="metric-card">
          <p className="text-slate-400 text-xs font-bold uppercase tracking-wider">Всего транзакций (Потоки)</p>
          <p className="text-3xl font-black text-slate-900 mt-1">{tickets.length}</p>
        </div>
        <div className="metric-card">
          <p className="text-slate-400 text-xs font-bold uppercase tracking-wider">Успешный ИИ-Автоответ (RAG)</p>
          <p className="text-3xl font-black text-emerald-600 mt-1">
            {tickets.filter((t: any) => t.Status === 'waiting_for_feedback' || t.Status === 'resolved').length}
          </p>
        </div>
        <div className="metric-card">
          <p className="text-slate-400 text-xs font-bold uppercase tracking-wider">Эскалировано на операторов</p>
          <p className="text-3xl font-black text-amber-500 mt-1">
            {tickets.filter((t: any) => t.Status === 'waiting').length}
          </p>
        </div>
      </div>

      {/* Графики */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-6">

        {/* График по категориям (Интенты) */}
        <div className="bg-white p-6 rounded-2xl border border-slate-200/80 shadow-sm hover:shadow-md transition-all duration-200">
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-bold text-base text-slate-900 flex items-center gap-2">
              📊 Классификация интентов LLM
            </h2>
            <span className="text-xs bg-slate-100 text-slate-600 px-2.5 py-1 rounded-full font-medium">
              По категориям
            </span>
          </div>

          <div className="w-full" style={{ height: 380 }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={categoryData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" vertical={false} />
                <XAxis dataKey="name" tick={{ fill: '#64748b', fontSize: 12 }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fill: '#64748b', fontSize: 12 }} axisLine={false} tickLine={false} />
                <Tooltip 
                  contentStyle={{ backgroundColor: '#090d16', borderRadius: '12px', border: 'none', color: '#fff' }}
                  itemStyle={{ color: '#a78bfa' }}
                />
                {/* Добавили скругление баров radius={[6, 6, 0, 0]} */}
                <Bar dataKey="value" fill="#8b5cf6" radius={[6, 6, 0, 0]} barSize={32} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* График по статусам жизненного цикла */}
        <div className="bg-white p-6 rounded-2xl border border-slate-200/80 shadow-sm hover:shadow-md transition-all duration-200">
          <div className="flex items-center justify-between mb-4">
            <h2 className="font-bold text-base text-slate-900 flex items-center gap-2">
              🔄 Статусы жизненного цикла тикетов
            </h2>
            <span className="text-xs bg-slate-100 text-slate-600 px-2.5 py-1 rounded-full font-medium">
              В реальном времени
            </span>
          </div>

          <div className="w-full" style={{ height: 380 }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={statusData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" vertical={false} />
                <XAxis dataKey="name" tick={{ fill: '#64748b', fontSize: 12 }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fill: '#64748b', fontSize: 12 }} axisLine={false} tickLine={false} />
                <Tooltip 
                  contentStyle={{ backgroundColor: '#090d16', borderRadius: '12px', border: 'none', color: '#fff' }}
                  itemStyle={{ color: '#3b82f6' }}
                />
                <Bar dataKey="value" fill="#3b82f6" radius={[6, 6, 0, 0]} barSize={32} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>

      </div>

      {/* Каналы связи */}
      <div className="bg-white rounded-xl border border-slate-200/80 shadow-sm overflow-hidden">
        <div className="p-4 bg-slate-50 border-b border-slate-200">
          <h2 className="font-bold text-sm text-slate-800 uppercase tracking-wider">Подключенные ингресс-шлюзы (API Gateways)</h2>
        </div>
        <table className="w-full border-collapse">
          <thead>
            <tr className="border-b border-slate-200 text-slate-400 text-xs font-bold uppercase tracking-wider bg-slate-50/50">
              <th className="p-4 text-left">Шлюз / Коннектор</th>
              <th className="p-4 text-left">Технологический тип</th>
              <th className="p-4 text-left">Интеграционный статус</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 text-sm">
            {channels.map((ch: any) => (
              <tr key={ch.ID} className="hover:bg-slate-50/50">
                <td className="p-4 font-semibold text-slate-800">{ch.Name}</td>
                <td className="p-4"><span className="bg-slate-100 text-slate-700 font-mono text-xs px-2 py-0.5 rounded">{ch.Type}</span></td>
                <td className="p-4">
                  <span className={`inline-flex items-center gap-1.5 text-xs font-medium ${ch.IsActive ? 'text-emerald-700' : 'text-slate-400'}`}>
                    <span className={`w-1.5 h-1.5 rounded-full ${ch.IsActive ? 'bg-emerald-500' : 'bg-slate-300'}`} />
                    {ch.IsActive ? 'Трафик активен' : 'Соединение разорвано'}
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