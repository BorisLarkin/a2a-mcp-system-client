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
    <div className="max-w-2xl">
      <h1 className="text-2xl font-bold mb-6">Настройки системы</h1>

      <div className="bg-white rounded-lg shadow p-6 space-y-5">
        <div className="flex items-center justify-between">
          <label className="font-medium">AI-обработка</label>
          <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} className="w-5 h-5" />
        </div>

        <div className="flex items-center justify-between">
          <label className="font-medium">Автоответ</label>
          <input type="checkbox" checked={autoRespond} onChange={e => setAutoRespond(e.target.checked)} className="w-5 h-5" />
        </div>

        <div>
          <label className="font-medium block mb-1">Порог уверенности: {confidenceThreshold}</label>
          <input
            type="range" min="0.5" max="0.99" step="0.01"
            value={confidenceThreshold} onChange={e => setConfidenceThreshold(parseFloat(e.target.value))}
            className="w-full"
          />
          <div className="flex justify-between text-xs text-gray-400">
            <span>0.5 (чаще эскалация)</span>
            <span>0.99 (чаще автоответ)</span>
          </div>
        </div>

        <div>
          <label className="font-medium block mb-1">Стиль общения</label>
          <select value={communicationStyle} onChange={e => setCommunicationStyle(e.target.value)} className="border p-2 rounded w-full">
            <option value="friendly">Дружелюбный</option>
            <option value="professional">Профессиональный</option>
            <option value="balanced">Сбалансированный</option>
          </select>
        </div>

        <div>
          <label className="font-medium block mb-1">Контекст компании</label>
          <textarea
            value={companyContext} onChange={e => setCompanyContext(e.target.value)}
            placeholder="Описание компании для AI..."
            className="border p-2 rounded w-full h-24"
          />
        </div>

        <button
          onClick={handleSave}
          disabled={mutation.isPending}
          className="bg-blue-600 text-white px-6 py-2 rounded-lg hover:bg-blue-700 disabled:opacity-50"
        >
          {mutation.isPending ? 'Сохранение...' : 'Сохранить настройки'}
        </button>

        {saved && <p className="text-green-600 text-sm">✅ Настройки сохранены</p>}
        {mutation.error && <p className="text-red-600 text-sm">❌ Ошибка сохранения</p>}
      </div>
    </div>
  );
}