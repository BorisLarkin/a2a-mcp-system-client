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
      alert('✅ Документ векторизован и добавлен в Qdrant');
    },
    onError: (err) => alert('❌ Ошибка эмбеддинга: ' + err),
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

  return (
    <div className="max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900 tracking-tight">База знаний RAG (Qdrant)</h1>
        <p className="text-sm text-slate-500 mt-1">Наполнение контекста для извлечения релевантных решений ИИ-агентом генерации ответов</p>
      </div>

      {/* Переключатель табов */}
      <div className="flex border-b border-slate-200 mb-6">
        <button 
          onClick={() => setTab('add')}
          className={`pb-2 px-4 text-sm font-semibold border-b-2 transition-all ${tab === 'add' ? 'border-blue-600 text-blue-600' : 'border-transparent text-slate-500 hover:text-slate-800'}`}
        >
          Ручной ввод инкрементов
        </button>
        <button 
          onClick={() => setTab('tickets')}
          className={`pb-2 px-4 text-sm font-semibold border-b-2 transition-all ${tab === 'tickets' ? 'border-blue-600 text-blue-600' : 'border-transparent text-slate-500 hover:text-slate-800'}`}
        >
          Импорт из успешных кейсов
        </button>
      </div>

      {tab === 'add' && (
        <div className="bg-white border border-slate-200 rounded-xl shadow-sm p-6 space-y-4">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs font-bold text-slate-500 uppercase">Тема или Тезис</label>
            <input type="text" placeholder="Например: Инструкция по сбросу пароля в корпоративном VPN" value={title}
              onChange={e => setTitle(e.target.value)} className="w-full" />
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-xs font-bold text-slate-500 uppercase">Информационный контекст (Знания)</label>
            <textarea placeholder="Вставьте сюда сырой текст, который AI-генератор должен использовать как первоисточник..." value={content}
              onChange={e => setContent(e.target.value)} className="w-full h-44 resize-none" />
          </div>

          <div className="flex flex-col gap-1.5 w-64">
            <label className="text-xs font-bold text-slate-500 uppercase">Классификационный тег</label>
            <select value={category} onChange={e => setCategory(e.target.value)} className="w-full">
              <option value="техническая">Техническая поддержка</option>
              <option value="биллинг">Финансы и Биллинг</option>
              <option value="жалоба">Рекламации и Жалобы</option>
              <option value="общий_вопрос">Общие регламенты</option>
              <option value="общее">Прочее (General)</option>
            </select>
          </div>

          <div className="pt-4 border-t border-slate-100">
            <button onClick={handleAdd} disabled={addMutation.isPending}
              className="bg-blue-600 text-white font-medium px-5 py-2.5 rounded-lg text-sm hover:bg-blue-700 shadow-md shadow-blue-500/10 disabled:opacity-50">
              {addMutation.isPending ? 'Расчет эмбеддингов...' : '📚 Индексировать в векторную БД'}
            </button>
          </div>
        </div>
      )}

      {tab === 'tickets' && (
        <div className="bg-white rounded-xl border border-slate-200/80 shadow-sm overflow-hidden">
          <table className="w-full border-collapse">
            <thead>
              <tr className="bg-slate-50 border-b border-slate-200 text-slate-500 text-xs font-bold uppercase tracking-wider">
                <th className="p-4 text-left">ID Решения</th>
                <th className="p-4 text-left">Текст обращения</th>
                <th className="p-4 text-left">Верифицированная категория</th>
                <th className="p-4"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-sm">
              {(ticketsData?.tickets || []).map((t: any) => (
                <tr key={t.ID} className="hover:bg-slate-50/60">
                  <td className="p-4 font-mono text-xs text-slate-400">#{t.ID?.slice(0, 8)}</td>
                  <td className="p-4 text-slate-700 max-w-xs truncate font-medium">{t.OriginalText}</td>
                  <td className="p-4">
                    <span className="bg-slate-100 text-slate-700 text-xs px-2 py-0.5 rounded font-medium border border-slate-200/50">
                      {t.Category || '—'}
                    </span>
                  </td>
                  <td className="p-4 text-right">
                    <button onClick={() => handleFromTicket(t)}
                      className="bg-violet-50 text-violet-700 hover:bg-violet-100 text-xs font-bold px-3 py-1.5 rounded-md border border-violet-100 transition-all">
                      ⚡ Обучить RAG
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