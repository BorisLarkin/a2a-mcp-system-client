import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { api } from '@/api/client';

const STATUS_LABELS: Record<string, string> = {
  new: 'Новый',
  in_progress: 'В работе',
  waiting: 'Ожидает',
  waiting_for_feedback: 'Ждёт ответа',
  resolved: 'Решён',
  closed: 'Закрыт',
};

export default function TicketDetail() {
  const { id } = useParams();
  const [reply, setReply] = useState('');

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['ticket', id],
    queryFn: () => api(`/tickets/${id}`),
  });

  if (isLoading) return <p className="p-6">Загрузка...</p>;

  const ticket = data?.ticket;
  const messages = data?.messages || [];
  const ai = ticket?.AIAnalysis;

  const sendReply = async () => {
    if (!reply.trim()) return;
    await api(`/tickets/${id}/messages`, {
      method: 'POST',
      body: JSON.stringify({ message_text: reply, sender_type: 'operator' }),
    });
    setReply('');
    refetch();
  };

  return (
    <div className="grid grid-cols-3 gap-6">
      <div className="col-span-2">
        <div className="flex items-center justify-between mb-4">
          <h1 className="text-xl font-bold">Тикет #{id?.slice(0, 8)}</h1>
          <span className={`px-3 py-1 rounded text-sm ${STATUS_LABELS[ticket?.Status] ? 'bg-blue-100 text-blue-700' : 'bg-gray-100'}`}>
            {STATUS_LABELS[ticket?.Status] || ticket?.Status}
          </span>
        </div>

        <div className="bg-white rounded-lg shadow p-4 mb-4">
          <p className="text-sm text-gray-500 mb-1">Клиент: {ticket?.Client?.Name || 'Неизвестный'}</p>
          <p className="text-gray-800">{ticket?.OriginalText}</p>
        </div>

        <div className="bg-white rounded-lg shadow p-4 mb-4">
          <h2 className="font-bold mb-3">Чат</h2>
          <div className="max-h-96 overflow-y-auto space-y-3">
            {messages.map((m: any) => (
              <div key={m.ID} className={`flex ${m.SenderType === 'operator' ? 'justify-end' : 'justify-start'}`}>
                <div className={`max-w-[75%] p-3 rounded-lg ${
                  m.SenderType === 'operator' ? 'bg-blue-100 text-blue-900' :
                  m.SenderType === 'ai' ? 'bg-green-100 text-green-900' :
                  'bg-gray-100 text-gray-900'
                }`}>
                  <p className="text-xs text-gray-500 mb-1">
                    {m.SenderType === 'operator' ? 'Оператор' : m.SenderType === 'ai' ? 'AI' : 'Клиент'}
                  </p>
                  <p className="text-sm whitespace-pre-wrap">{m.MessageText}</p>
                </div>
              </div>
            ))}
          </div>
        </div>

        {ticket?.Status !== 'resolved' && ticket?.Status !== 'closed' && (
          <div className="flex gap-2">
            <input
              type="text" value={reply} onChange={e => setReply(e.target.value)}
              placeholder="Введите ответ клиенту..."
              className="flex-1 border p-2 rounded-lg"
              onKeyDown={e => e.key === 'Enter' && sendReply()}
            />
            <button onClick={sendReply} className="bg-blue-600 text-white px-6 py-2 rounded-lg hover:bg-blue-700">
              Отправить
            </button>
          </div>
        )}
      </div>

      <div className="space-y-4">
        {ai?.classification && (
          <div className="bg-white rounded-lg shadow p-4">
            <h2 className="font-bold mb-2">AI-анализ</h2>
            <div className="space-y-2 text-sm">
              <p><strong>Категория:</strong> {ai.classification.category || ai.classification.predicted_class || '—'}</p>
              <p><strong>Уверенность:</strong> {ai.classification.confidence ? `${Math.round(ai.classification.confidence * 100)}%` : '—'}</p>
              {ai.suggested_team && <p><strong>Команда:</strong> {ai.suggested_team}</p>}
            </div>
          </div>
        )}

        {ai?.execution_log && (
          <div className="bg-white rounded-lg shadow p-4">
            <h2 className="font-bold mb-2">Лог выполнения</h2>
            <div className="text-xs text-gray-600 max-h-64 overflow-y-auto">
              {ai.execution_log.map((line: string, i: number) => (
                <p key={i} className="mb-1">{line}</p>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}