import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { api } from '@/api/client';
import { useState } from 'react';
import { useWebSocket } from '@/hooks/useWebSocket';

interface Stats {
  waiting: number;
  in_progress: number;
  waiting_for_feedback: number;
  resolved_today: number;
}

export default function OperatorDashboard() {
  const queryClient = useQueryClient();
    
  useWebSocket((data) => {
      if (data.type === 'ticket_created' || data.type === 'ticket_updated' || data.type === 'new_escalated') {
          // Обновляем список тикетов
          queryClient.invalidateQueries({ queryKey: ['tickets'] });
          queryClient.invalidateQueries({ queryKey: ['queue'] });
      }
  });

  const { data: allTickets } = useQuery({
    queryKey: ['tickets', 'all'],
    queryFn: () => api('/tickets?limit=100'),
  });

  const tickets = allTickets?.tickets || [];

  const stats: Stats = {
    waiting: tickets.filter((t: any) => t.Status === 'waiting' || t.Status === 'new').length,
    in_progress: tickets.filter((t: any) => t.Status === 'in_progress').length,
    waiting_for_feedback: tickets.filter((t: any) => t.Status === 'waiting_for_feedback').length,
    resolved_today: tickets.filter((t: any) => t.Status === 'resolved').length,
  };

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Дашборд оператора</h1>
      
      <div className="grid grid-cols-4 gap-4 mb-8">
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-gray-500 text-sm">Ожидают обработки</p>
          <p className="text-3xl font-bold text-yellow-600">{stats.waiting}</p>
        </div>
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-gray-500 text-sm">В работе</p>
          <p className="text-3xl font-bold text-purple-600">{stats.in_progress}</p>
        </div>
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-gray-500 text-sm">Ждут ответа клиента</p>
          <p className="text-3xl font-bold text-green-600">{stats.waiting_for_feedback}</p>
        </div>
        <div className="bg-white p-4 rounded-lg shadow">
          <p className="text-gray-500 text-sm">Решено сегодня</p>
          <p className="text-3xl font-bold text-gray-600">{stats.resolved_today}</p>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        <div className="bg-white p-4 rounded-lg shadow">
          <h2 className="font-bold mb-3">Последние тикеты</h2>
          {tickets.slice(0, 5).map((t: any) => (
            <div key={t.ID} className="border-b py-2 last:border-0">
              <Link to={`/tickets/${t.ID}`} className="text-blue-600 hover:underline font-mono text-sm">
                {t.ID?.slice(0, 8)}
              </Link>
              <span className="text-gray-600 text-sm ml-2">{t.OriginalText?.slice(0, 60)}...</span>
              <span className={`ml-2 px-2 py-0.5 rounded text-xs ${
                t.Status === 'waiting' ? 'bg-yellow-100 text-yellow-700' :
                t.Status === 'waiting_for_feedback' ? 'bg-green-100 text-green-700' :
                'bg-gray-100 text-gray-700'
              }`}>{t.Status}</span>
            </div>
          ))}
          <Link to="/queue" className="text-blue-600 hover:underline text-sm mt-3 inline-block">
            Все тикеты →
          </Link>
        </div>

        <div className="bg-white p-4 rounded-lg shadow">
          <h2 className="font-bold mb-3">Быстрые действия</h2>
          <div className="space-y-2">
            <Link to="/queue?status=waiting" className="block p-3 bg-yellow-50 rounded hover:bg-yellow-100 text-sm">
              🔴 Ожидающие обработки — {stats.waiting}
            </Link>
            <Link to="/queue?status=waiting_for_feedback" className="block p-3 bg-green-50 rounded hover:bg-green-100 text-sm">
              🟢 Ждут ответа клиента — {stats.waiting_for_feedback}
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}