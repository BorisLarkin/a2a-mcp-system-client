import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { api } from '@/api/client';

export default function Knowledge() {
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<'add' | 'tickets'>('add');
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [category, setCategory] = useState('общее');

  const { data: ticketsData } = useQuery({
    queryKey: ['tickets', 'resolved'],
    queryFn: () => api('/tickets?status=resolved,closed&limit=50'),
    enabled: tab === 'tickets',
  });

  const addMutation = useMutation({
    mutationFn: (doc: any) => api('/admin/knowledge', { method: 'POST', body: JSON.stringify(doc) }),
    onSuccess: () => {
      setTitle('');
      setContent('');
      alert('✅ Документ добавлен в базу знаний');
    },
    onError: (err) => alert('❌ Ошибка: ' + err),
  });

  const handleAdd = () => {
    if (!title || !content) return;
    addMutation.mutate({ title, content, category });
  };

  const handleFromTicket = (ticket: any) => {
    addMutation.mutate({
      title: ticket.Subject || ticket.OriginalText?.slice(0, 100) || 'Без названия',
      content: ticket.AIResponse || ticket.OriginalText,
      category: ticket.Category || 'общее',
      ticket_id: ticket.ID,
    });
  };

  const tickets = ticketsData?.tickets || [];

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">База знаний</h1>

      <div className="flex gap-2 mb-4">
        <button onClick={() => setTab('add')}
          className={`px-4 py-1 rounded text-sm ${tab === 'add' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}>
          Добавить документ
        </button>
        <button onClick={() => setTab('tickets')}
          className={`px-4 py-1 rounded text-sm ${tab === 'tickets' ? 'bg-blue-600 text-white' : 'bg-gray-200'}`}>
          Из решённых тикетов
        </button>
      </div>

      {tab === 'add' && (
        <div className="bg-white p-4 rounded-lg shadow">
          <input type="text" placeholder="Название документа" value={title}
            onChange={e => setTitle(e.target.value)}
            className="w-full border p-2 rounded mb-3" />
          <textarea placeholder="Содержание документа" value={content}
            onChange={e => setContent(e.target.value)}
            className="w-full border p-2 rounded mb-3 h-32" />
          <select value={category} onChange={e => setCategory(e.target.value)}
            className="border p-2 rounded mb-3">
            <option value="техническая">Техническая</option>
            <option value="биллинг">Биллинг</option>
            <option value="жалоба">Жалоба</option>
            <option value="общий_вопрос">Общий вопрос</option>
            <option value="общее">Другое</option>
          </select>
          <button onClick={handleAdd} disabled={addMutation.isPending}
            className="bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700">
            {addMutation.isPending ? 'Добавление...' : '📚 Добавить в базу знаний'}
          </button>
        </div>
      )}

      {tab === 'tickets' && (
        <div className="bg-white rounded-lg shadow overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50">
              <tr>
                <th className="p-3 text-left text-sm">ID</th>
                <th className="p-3 text-left text-sm">Текст</th>
                <th className="p-3 text-left text-sm">Категория</th>
                <th className="p-3"></th>
              </tr>
            </thead>
            <tbody>
              {tickets.map((t: any) => (
                <tr key={t.ID} className="border-t">
                  <td className="p-3 text-sm font-mono">{t.ID?.slice(0, 8)}</td>
                  <td className="p-3 text-sm max-w-xs truncate">{t.OriginalText}</td>
                  <td className="p-3 text-sm">{t.Category || '—'}</td>
                  <td className="p-3">
                    <button onClick={() => handleFromTicket(t)}
                      className="text-blue-600 hover:underline text-sm">
                      📚 В БЗ
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}