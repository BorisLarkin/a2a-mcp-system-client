import { useQuery, useMutation } from '@tanstack/react-query';
import { useState, useEffect } from 'react';
import { api } from '@/api/client';

export default function Settings() {
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['admin', 'settings'],
    queryFn: () => api('/admin/settings'),
  });

  const [enabled, setEnabled] = useState(true);
  const [autoRespond, setAutoRespond] = useState(true);
  const [confidenceThreshold, setConfidenceThreshold] = useState(0.7);
  const [communicationStyle, setCommunicationStyle] = useState('friendly');
  const [companyContext, setCompanyContext] = useState('');
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (data?.ai_settings) {
        const s = data.ai_settings;
        setEnabled(s.Enabled ?? true);
        setAutoRespond(s.AutoRespond ?? true);
        setConfidenceThreshold(s.ConfidenceThreshold ?? 0.7);
        setCommunicationStyle(s.CommunicationStyle ?? 'friendly');
        setCompanyContext(s.SystemContext ?? '');
    }
  }, [data]);

  const mutation = useMutation({
    mutationFn: (settings: any) => api('/admin/settings', { method: 'PUT', body: JSON.stringify(settings) }),
    onSuccess: () => {
      setSaved(true);
      refetch();
      setTimeout(() => setSaved(false), 3000);
    },
  });

  const handleSave = () => {
    mutation.mutate({
      enabled,
      auto_respond: autoRespond,
      confidence_threshold: confidenceThreshold,
      communication_style: communicationStyle,
      system_context: companyContext,
    });
  };

  if (isLoading) return <p className="p-6">Загрузка...</p>;

  return (
    <div className="max-w-3xl mx-auto bg-white border border-slate-200/80 rounded-xl shadow-sm p-6">
      <div className="border-b border-slate-100 pb-4 mb-6">
        <h1 className="text-xl font-bold text-slate-900">Настройки оркестратора и LLM</h1>
        <p className="text-sm text-slate-500 mt-1">Глобальные параметры автоматизации Saiga 8B и диспетчерской прокси</p>
      </div>

      <div className="space-y-6">
        {/* Ползунок Порога уверенности */}
        <div className="bg-slate-50 p-4 rounded-xl border border-slate-100">
          <div className="flex justify-between items-center mb-2">
            <label className="font-semibold text-sm text-slate-800">Порог уверенности классификации</label>
            <span className="bg-blue-600 text-white font-mono text-xs px-2 py-0.5 rounded-md font-bold">
              {confidenceThreshold}
            </span>
          </div>
          <input 
            type="range" min="0.5" max="0.99" step="0.01" 
            value={confidenceThreshold} 
            onChange={e => setConfidenceThreshold(parseFloat(e.target.value))}
            className="w-full h-2 bg-slate-200 rounded-lg appearance-none cursor-pointer accent-blue-600"
          />
          <div className="flex justify-between text-[11px] text-slate-400 mt-1 font-medium">
            <span>0.5 (Агрессивный автоответ)</span>
            <span>0.99 (Частая эскалация человеку)</span>
          </div>
        </div>

        {/* Селектор стиля */}
        <div className="flex flex-col gap-1.5">
          <label className="font-semibold text-sm text-slate-800">Стиль ответов генератора</label>
          <select 
            value={communicationStyle} 
            onChange={e => setCommunicationStyle(e.target.value)}
            className="w-full"
          >
            <option value="friendly">Дружелюбный (Паблик / Поддержка)</option>
            <option value="professional">Профессиональный (B2B Диспетчерская)</option>
            <option value="balanced">Сбалансированный</option>
          </select>
        </div>

        {/* Системный контекст */}
        <div className="flex flex-col gap-1.5">
          <label className="font-semibold text-sm text-slate-800">Контекст компании (RAG System Prompt)</label>
          <textarea
            value={companyContext} 
            onChange={e => setCompanyContext(e.target.value)}
            placeholder="Укажите специфику работы диспетчерской, критические инструкции..."
            className="w-full h-32 resize-none"
          />
        </div>

        {/* Кнопка сохранения */}
        <div className="pt-4 border-t border-slate-100 flex items-center gap-4">
          <button
            onClick={handleSave}
            disabled={mutation.isPending}
            className="bg-blue-600 text-white font-medium px-5 py-2.5 rounded-lg hover:bg-blue-700 shadow-md shadow-blue-500/10 disabled:opacity-50"
          >
            {mutation.isPending ? 'Синхронизация...' : 'Сохранить конфигурацию'}
          </button>
          {saved && <span className="text-emerald-600 text-sm font-medium flex items-center gap-1">✅ Успешно применено в Go-оркестраторе</span>}
        </div>
      </div>
    </div>
  );
}