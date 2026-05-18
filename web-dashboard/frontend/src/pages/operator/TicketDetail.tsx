import { useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
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

const STATUS_COLORS: Record<string, string> = {
  new: 'bg-blue-50 text-blue-700 border-blue-200/60',
  in_progress: 'bg-violet-50 text-violet-700 border-violet-200/60',
  waiting: 'bg-amber-50 text-amber-700 border-amber-200/60',
  waiting_for_feedback: 'bg-emerald-50 text-emerald-700 border-emerald-200/60',
  resolved: 'bg-slate-50 text-slate-600 border-slate-200',
  closed: 'bg-rose-50 text-rose-700 border-rose-200/60',
};

export default function TicketDetail() {
  const { id } = useParams();
  const [reply, setReply] = useState('');
  const queryClient = useQueryClient();

  // ВСЕ ХУКИ ДО УСЛОВНОГО ВОЗВРАТА
  useWebSocket((data) => {
    console.log('WS message:', data.type, data.data);
    if (data.type === 'ticket_updated' && data.data?.ticket_id === id) {
        console.log('Invalidating ticket query');
        queryClient.invalidateQueries({ queryKey: ['ticket', id] });
    }
    if (data.type === 'message_added' && data.data?.ticket_id === id) {
        queryClient.invalidateQueries({ queryKey: ['ticket', id] });
    }
  });

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['ticket', id],
    queryFn: () => api(`/tickets/${id}`),
  });

  // Условный возврат ТОЛЬКО после всех хуков
  if (isLoading) return <p className="p-6 text-slate-500">Загрузка...</p>;

  const ticket = data?.ticket;
  const messages = data?.messages || [];
  const ai = ticket?.AIAnalysis;
  const isClosed = ticket?.Status === 'resolved' || ticket?.Status === 'closed';

  const sendReply = async () => {
    if (!reply.trim()) return;
    await api(`/tickets/${id}/messages`, {
      method: 'POST',
      body: JSON.stringify({ message_text: reply, sender_type: 'operator' }),
    });
    setReply('');
    refetch();
  };

  const addToKnowledge = async () => {
    // Собираем все сообщения оператора
    const operatorMessages = messages
      .filter((m: any) => m.SenderType === 'operator')
      .map((m: any) => m.MessageText)
      .join('\n\n');

    const content = operatorMessages || ticket?.AIResponse || ticket?.OriginalText;
    if (!content) return;

    try {
      await api('/admin/knowledge', {
        method: 'POST',
        body: JSON.stringify({
          title: ticket?.Subject || ticket?.OriginalText?.slice(0, 100) || 'Без названия',
          content: content,
          category: ticket?.Category || 'общее',
          ticket_id: ticket?.ID,
        }),
      });
      alert('✅ Документ добавлен в базу знаний');
    } catch (e) {
      alert('❌ Ошибка при добавлении документа');
    }
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 max-w-7xl mx-auto">
      
      {/* Основное окно чата и ответов оператора (Занимает 2 колонки) */}
      <div className="lg:col-span-2 space-y-6">
        
        {/* Карточка самого тикета */}
        <div className="bg-white rounded-xl shadow-sm border border-slate-200/80 p-6">
          <div className="flex justify-between items-start gap-4 mb-4">
            <div>
              <span className="text-xs font-mono text-slate-400 bg-slate-100 px-2 py-0.5 rounded">
                ID: {id?.slice(0, 8)}
              </span>
              <h1 className="text-xl font-bold text-slate-900 mt-1.5">
                Обращение от {ticket?.Client?.Name || 'Клиента'}
              </h1>
            </div>
            
            {/* Статус тикета */}
            <span className={`px-3 py-1 text-xs font-semibold rounded-full border ${
              STATUS_COLORS[ticket?.Status] || 'bg-slate-100 text-slate-700'
            }`}>
              {STATUS_LABELS[ticket?.Status] || ticket?.Status}
            </span>
          </div>
          
          <div className="text-xs font-medium text-slate-400 uppercase tracking-wider mb-2">
            Первичное сообщение пользователя
          </div>
          <div className="bg-slate-50 rounded-xl p-4 text-slate-700 border border-slate-100 text-sm leading-relaxed whitespace-pre-wrap">
            {ticket?.OriginalText}
          </div>
        </div>

        {/* 💬 ЛЕНТА ДИАЛОГА */}
        <div className="bg-white rounded-xl shadow-sm border border-slate-200/80 p-6 space-y-4">
          <h2 className="font-bold text-base text-slate-900 border-b border-slate-100 pb-3 flex items-center gap-2">
            💬 История переписки и логи ответов
          </h2>
          
          <div className="max-h-[450px] overflow-y-auto space-y-4 pr-1">
            {/* Если система сгенерировала быстрый автоответ ИИ */}
            {ticket?.AIResponse && (
              <div className="ai-card-glow p-4 rounded-xl space-y-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="ai-pulse-dot" />
                    <span className="text-xs font-bold text-violet-700 uppercase tracking-wide">
                      Автоответ Системы (AI-Диспетчер)
                    </span>
                  </div>
                </div>
                <div className="text-sm text-slate-700 leading-relaxed whitespace-pre-wrap">
                  {ticket.AIResponse}
                </div>
              </div>
            )}

            {/* Основной список сообщений */}
            {messages.length > 0 ? (
              messages.map((m: any) => {
                const isOperator = m.SenderType === 'operator';
                const isAi = m.SenderType === 'ai';
                
                return (
                  <div key={m.ID} className={`flex ${isOperator ? 'justify-end' : 'justify-start'}`}>
                    <div className={`max-w-[80%] p-4 rounded-2xl shadow-xs leading-relaxed text-sm ${
                      isOperator 
                        ? 'bg-blue-600 text-white rounded-tr-none' 
                        : isAi 
                          ? 'ai-card-glow text-slate-800 rounded-tl-none'
                          : 'bg-slate-100 text-slate-800 rounded-tl-none border border-slate-200/50'
                    }`}>
                      <div className={`text-[10px] font-bold uppercase tracking-wider mb-1 opacity-60 ${
                        isOperator ? 'text-blue-100' : isAi ? 'text-violet-700' : 'text-slate-500'
                      }`}>
                        {isOperator ? '👨‍💻 Оператор' : isAi ? '🤖 AI-Ассистент' : '👤 Клиент'}
                      </div>
                      <div className="whitespace-pre-wrap">{m.MessageText}</div>
                    </div>
                  </div>
                );
              })
            ) : (
              !ticket?.AIResponse && (
                <p className="text-sm text-slate-400 italic text-center py-6">
                  Дополнительных сообщений в этом тикете пока нет.
                </p>
              )
            )}
          </div>
        </div>

        {/* Форма отправки ответа оператора */}
        {!isClosed ? (
          <div className="flex gap-2 bg-white p-2 rounded-xl border border-slate-200/80 shadow-xs">
            <input
              type="text" 
              value={reply} 
              onChange={e => setReply(e.target.value)}
              placeholder="Введите ответ клиенту (Отправить через Enter)..."
              className="flex-1 px-4 py-2 text-sm bg-transparent border-0 focus:outline-none focus:ring-0"
              onKeyDown={e => e.key === 'Enter' && sendReply()}
            />
            <button 
              onClick={sendReply} 
              className="bg-blue-600 text-white px-5 py-2 text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors shadow-xs"
            >
              Отправить
            </button>
          </div>
        ) : (
          <div className="bg-slate-100 text-slate-600 border border-slate-200/60 rounded-xl p-4 text-center text-sm font-medium">
            🔒 Тикет закрыт со статусом решён. Ответы больше не принимаются.
          </div>
        )}
      </div>

      {/* 👉 Сайдбар аналитики AI и протокола MCP (Занимает 1 колонку) */}
      <div className="space-y-6">
        
        {/* Блок AI Анализа */}
        {ai?.classification && (
          <div className="ai-card-glow rounded-xl p-5 relative overflow-hidden">
            <div className="absolute top-0 right-0 bg-violet-500/10 text-violet-700 px-3 py-1 text-[10px] uppercase font-bold rounded-bl-lg flex items-center gap-1">
              <span className="ai-pulse-dot"></span> Core Agent AI
            </div>
            
            <h2 className="font-bold text-base text-slate-900 mb-4 flex items-center gap-2">
              🧠 AI-Аналитика
            </h2>
            
            <div className="space-y-3 text-sm">
              <div className="flex justify-between py-1.5 border-b border-slate-100">
                <span className="text-slate-500">Классификация:</span>
                <span className="font-semibold text-violet-700 bg-violet-50 px-2 py-0.5 rounded text-xs">
                  {ai.classification.category || ai.classification.predicted_class || '—'}
                </span>
              </div>
              <div className="flex justify-between py-1.5 border-b border-slate-100">
                <span className="text-slate-500">Уверенность LLM:</span>
                <span className="font-mono font-bold text-slate-800">
                  {ai.classification.confidence ? `${Math.round(ai.classification.confidence * 100)}%` : '—'}
                </span>
              </div>
            </div>
          </div>
        )}

        {/* Блок логов MCP (Вызовы инструментов/баз данных) */}
        {ai?.execution_log && (
          <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-5">
            <h2 className="font-bold text-base text-slate-900 mb-3 flex items-center gap-2">
              🛠️ Оркестрация & MCP Лог
            </h2>
            <div className="mcp-terminal max-h-72 overflow-y-auto text-xs whitespace-pre-wrap">
              {Array.isArray(ai.execution_log) 
                ? ai.execution_log.map((line: string, i: number) => <p key={i} className="mb-1">{line}</p>)
                : ai.execution_log
              }
            </div>
          </div>
        )}

        {/* ✨ КНОПКА БАЗЫ ЗНАНИЙ (ТЕПЕРЬ ЗДЕСЬ) */}
        {(ticket?.Status === 'resolved' || ticket?.Status === 'closed') && (
          <div className="bg-white rounded-xl shadow-sm border border-slate-200 p-4 space-y-2">
            <div className="text-xs text-slate-400 font-medium uppercase tracking-wider">
              Администрирование знаний
            </div>
            <button 
              onClick={addToKnowledge} 
              className="w-full bg-emerald-600 text-white px-4 py-2.5 text-sm font-medium rounded-lg hover:bg-emerald-700 transition-colors shadow-xs flex items-center justify-center gap-2"
            >
              📚 Экспортировать решение в БЗ
            </button>
          </div>
        )}
        
      </div>
    </div>
  );
}