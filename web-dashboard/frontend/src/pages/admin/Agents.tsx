import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { api } from '@/api/client';

export default function Agents() {
  const queryClient = useQueryClient();
  const [showAdd, setShowAdd] = useState(false);
  const [newEndpoint, setNewEndpoint] = useState('');
  const [selectedAgent, setSelectedAgent] = useState<any>(null);
  const [tab, setTab] = useState<'all' | 'own' | 'common'>('all');

  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'agents'],
    queryFn: () => api('/admin/agents'),
  });

  const addMutation = useMutation({
    mutationFn: (endpoint: string) =>
      api('/admin/agents', { method: 'POST', body: JSON.stringify({ endpoint }) }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'agents'] });
      setShowAdd(false);
      setNewEndpoint('');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api(`/admin/agents/${id}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'agents'] }),
  });

  const allAgents = data?.agents || [];
  const commonAgents = allAgents.filter((a: any) => a.DispatcherID === null);
  const ownAgents = allAgents.filter((a: any) => a.DispatcherID !== null);
  const displayed = tab === 'own' ? ownAgents : tab === 'common' ? commonAgents : allAgents;

  const getSkillsFromMetadata = (agent: any) => {
    if (agent.Metadata?.skills && Array.isArray(agent.Metadata.skills)) {
      return agent.Metadata.skills;
    }
    return [];
  };

  return (
    <div className="max-w-7xl mx-auto">
      {/* Шапка */}
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 tracking-tight">AI-Агенты (A2A Протокол)</h1>
          <p className="text-sm text-slate-500 mt-1">Подключение, мониторинг и менеджмент распределенных микросервисов ИИ</p>
        </div>
        <button 
          onClick={() => setShowAdd(!showAdd)} 
          className="bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 shadow-sm shadow-blue-500/10 flex items-center gap-2"
        >
          <span>{showAdd ? 'Свернуть форму' : '+ Подключить агента'}</span>
        </button>
      </div>

      {/* Форма добавления */}
      {showAdd && (
        <div className="bg-white p-5 rounded-xl border border-slate-200 shadow-sm mb-6 max-w-2xl">
          <h2 className="font-bold text-slate-900 mb-1">Регистрация внешнего агента</h2>
          <p className="text-xs text-slate-500 mb-4">Оркестратор выполнит handshake-запрос для верификации MCP-инструментов</p>
          <div className="flex gap-3">
            <input 
              type="text" value={newEndpoint} onChange={e => setNewEndpoint(e.target.value)}
              placeholder="http://100.93.170.55:9004" className="flex-1" 
            />
            <button 
              onClick={() => addMutation.mutate(newEndpoint)} disabled={addMutation.isPending}
              className="bg-slate-900 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-slate-800 disabled:opacity-50"
            >
              {addMutation.isPending ? 'Проверка...' : 'Активировать'}
            </button>
          </div>
          {addMutation.isError && <p className="text-red-600 text-xs mt-2 font-medium">❌ {String(addMutation.error)}</p>}
          {addMutation.isSuccess && <p className="text-emerald-600 text-xs mt-2 font-medium">✅ Агент успешно интегрирован в mesh-сеть</p>}
        </div>
      )}

      {/* Табы фильтрации */}
      <div className="flex gap-1.5 p-1 bg-slate-200/60 rounded-lg w-max mb-6">
        <button onClick={() => setTab('all')} className={`px-4 py-1.5 rounded-md text-xs font-semibold ${tab === 'all' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-600 hover:text-slate-900'}`}>
          Все системы ({allAgents.length})
        </button>
        <button onClick={() => setTab('own')} className={`px-4 py-1.5 rounded-md text-xs font-semibold ${tab === 'own' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-600 hover:text-slate-900'}`}>
          Локальные ({ownAgents.length})
        </button>
        <button onClick={() => setTab('common')} className={`px-4 py-1.5 rounded-md text-xs font-semibold ${tab === 'common' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-600 hover:text-slate-900'}`}>
          Глобальные Cloud-агенты ({commonAgents.length})
        </button>
      </div>

      {/* Контент */}
      {isLoading ? (
        <div className="text-center p-12 text-slate-500 text-sm">Синхронизация узлов сети...</div>
      ) : displayed.length === 0 ? (
        <div className="bg-white rounded-xl border border-slate-200 p-12 text-center text-slate-400 text-sm">Активные агенты в выбранной группе отсутствуют</div>
      ) : (
        <div className="bg-white rounded-xl border border-slate-200/80 shadow-sm overflow-hidden">
          <table className="w-full border-collapse">
            <thead>
              <tr className="bg-slate-50 border-b border-slate-200 text-slate-500 text-xs font-bold uppercase tracking-wider">
                <th className="p-4 text-left">Идентификатор агента</th>
                <th className="p-4 text-left">Архитектурный тип</th>
                <th className="p-4 text-left">Сетевой Endpoint</th>
                <th className="p-4 text-left">MCP Skills</th>
                <th className="p-4 text-left">Статус шины</th>
                <th className="p-4"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-sm">
              {displayed.map((agent: any) => {
                const skills = Array.isArray(agent.Skills) ? agent.Skills : [];
                const displaySkills = getSkillsFromMetadata(agent).length > 0 ? getSkillsFromMetadata(agent) : skills;
                
                return (
                  <tr key={agent.ID} className="hover:bg-slate-50/80 cursor-pointer" onClick={() => setSelectedAgent(agent)}>
                    <td className="p-4 font-semibold text-slate-900">{agent.Name}</td>
                    <td className="p-4 text-slate-600">
                      <span className="bg-slate-100 text-slate-700 font-mono text-xs px-2 py-0.5 rounded">
                        {agent.AgentType}
                      </span>
                    </td>
                    <td className="p-4 font-mono text-xs text-slate-500">{agent.Endpoint}</td>
                    <td className="p-4 font-medium text-violet-600">{displaySkills.length} шт.</td>
                    <td className="p-4">
                      <span className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        agent.Status === 'online' ? 'bg-emerald-50 text-emerald-700 border border-emerald-100' : 'bg-rose-50 text-rose-700 border border-rose-100'
                      }`}>
                        <span className={`w-1.5 h-1.5 rounded-full ${agent.Status === 'online' ? 'bg-emerald-500' : 'bg-rose-500'}`} />
                        {agent.Status}
                      </span>
                    </td>
                    <td className="p-4 text-right">
                      {agent.DispatcherID !== null && (
                        <button
                          onClick={(e) => { 
                            e.stopPropagation(); 
                            if (confirm('Разорвать соединение с агентом?')) deleteMutation.mutate(agent.ID); 
                          }}
                          className="text-rose-600 hover:text-rose-700 hover:underline text-xs font-semibold"
                        >
                          Отключить
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Модальное окно деталей (Фирменный AI-стиль) */}
      {selectedAgent && (
        <div className="fixed inset-0 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={() => setSelectedAgent(null)}>
          <div className="ai-card-glow rounded-2xl p-6 max-w-xl w-full max-h-[85vh] overflow-y-auto shadow-2xl" onClick={e => e.stopPropagation()}>
            <div className="flex justify-between items-start mb-4 border-b border-slate-100 pb-3">
              <div>
                <span className="text-[10px] bg-violet-100 text-violet-800 font-bold px-2 py-0.5 rounded uppercase tracking-wider">Спецификация манифеста</span>
                <h2 className="text-xl font-bold text-slate-900 mt-1">{selectedAgent.Name}</h2>
              </div>
              <button onClick={() => setSelectedAgent(null)} className="text-slate-400 hover:text-slate-600 font-bold text-lg">&times;</button>
            </div>
            
            <div className="space-y-4 text-sm">
              <div className="grid grid-cols-2 gap-4 bg-slate-50 p-3 rounded-xl border border-slate-100">
                <div>
                  <p className="text-xs text-slate-400 font-medium">Протокол вызова</p>
                  <p className="font-semibold text-slate-800 mt-0.5">{selectedAgent.AgentType}</p>
                </div>
                <div>
                  <p className="text-xs text-slate-400 font-medium">Статус ноды</p>
                  <p className="font-semibold text-emerald-600 mt-0.5">{selectedAgent.Status}</p>
                </div>
              </div>

              <div>
                <p className="text-xs text-slate-400 font-medium mb-1">Сетевой адрес (URL)</p>
                <code className="block bg-slate-900 text-slate-200 text-xs p-2 rounded-lg font-mono overflow-x-auto">{selectedAgent.Endpoint}</code>
              </div>

              <div>
                <p className="text-xs text-slate-400 font-medium mb-2">Зарегистрированные MCP-навыки (Skills)</p>
                {(getSkillsFromMetadata(selectedAgent).length > 0 ? getSkillsFromMetadata(selectedAgent) : (Array.isArray(selectedAgent.Skills) ? selectedAgent.Skills : [])).map((skill: any, i: number) => (
                  <div key={i} className="border border-purple-100 bg-purple-50/20 p-3 rounded-xl mt-2">
                    <p className="font-semibold text-violet-800 font-mono text-xs">{skill.id}</p>
                    <p className="text-slate-600 text-xs mt-0.5">{skill.description}</p>
                    
                    {skill.input_schema && (
                      <details className="mt-2 group">
                        <summary className="text-blue-600 group-hover:text-blue-700 cursor-pointer text-xs font-medium outline-none">Схема входящих параметров (JSON Schema)</summary>
                        <pre className="text-[11px] mt-1.5 bg-slate-950 text-slate-300 p-2 rounded-lg overflow-x-auto font-mono max-h-40">{JSON.stringify(skill.input_schema, null, 2)}</pre>
                      </details>
                    )}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}