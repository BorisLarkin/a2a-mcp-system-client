import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';

const COLORS = ['#3b82f6', '#f59e0b', '#8b5cf6', '#10b981', '#ef4444'];

export default function AdminDashboard() {
  const { data: analytics } = useQuery({
    queryKey: ['admin', 'analytics'],
    queryFn: () => api('/admin/analytics'),
  });

  const { data: ticketsData } = useQuery({
    queryKey: ['tickets', 'all'],
    queryFn: () => api('/tickets?limit=1000'),
  });

  const tickets = ticketsData?.tickets || [];

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

  const { data: channelsData } = useQuery({
    queryKey: ['admin', 'channels'],
    queryFn: () => api('/admin/channels'),
  });

  const channels = channelsData?.channels || [];

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Дашборд администратора</h1>

      <div className="grid grid-cols-3 gap-4 mb-8">
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-gray-500 text-sm">Всего тикетов</p>
          <p className="text-3xl font-bold">{tickets.length}</p>
        </div>
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-gray-500 text-sm">Автоответов</p>
          <p className="text-3xl font-bold text-green-600">
            {tickets.filter((t: any) => t.Status === 'waiting_for_feedback' || t.Status === 'resolved').length}
          </p>
        </div>
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-gray-500 text-sm">Эскалировано</p>
          <p className="text-3xl font-bold text-yellow-600">
            {tickets.filter((t: any) => t.Status === 'waiting').length}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        <div className="bg-white p-4 rounded-lg shadow">
          <h2 className="font-bold mb-3">По категориям</h2>
          <ResponsiveContainer width="100%" height={250}>
            <PieChart>
              <Pie data={categoryData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={80} label>
                {categoryData.map((_, index) => (
                  <Cell key={index} fill={COLORS[index % COLORS.length]} />
                ))}
              </Pie>
              <Tooltip />
            </PieChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white p-4 rounded-lg shadow">
          <h2 className="font-bold mb-3">По статусам</h2>
          <ResponsiveContainer width="100%" height={250}>
            <BarChart data={statusData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="name" />
              <YAxis />
              <Tooltip />
              <Bar dataKey="value" fill="#3b82f6" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>
      <div className="bg-white p-4 rounded-lg shadow mt-6">
            <h2 className="font-bold mb-3">Каналы связи</h2>
            <table className="w-full">
                <thead className="bg-gray-50">
                    <tr>
                        <th className="p-2 text-left text-sm">Название</th>
                        <th className="p-2 text-left text-sm">Тип</th>
                        <th className="p-2 text-left text-sm">Активен</th>
                    </tr>
                </thead>
                <tbody>
                    {channels.map((ch: any) => (
                        <tr key={ch.ID} className="border-t">
                            <td className="p-2 text-sm">{ch.Name}</td>
                            <td className="p-2 text-sm">{ch.Type}</td>
                            <td className="p-2 text-sm">{ch.IsActive ? '✅' : '❌'}</td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    </div>
  );
}