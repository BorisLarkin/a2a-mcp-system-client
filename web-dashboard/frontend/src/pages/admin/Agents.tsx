import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { api } from '@/api/client';

function safeParseJSON(raw: string | undefined, fallback: any = []): any {
  if (!raw || raw === 'null') return fallback;
  try { return JSON.parse(raw); } catch { return fallback; }
}

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
  const ownAgents = data?.own || [];
  const commonAgents = data?.common || [];
  const displayed = tab === 'own' ? ownAgents : tab === 'common' ? commonAgents : allAgents;

  const renderTable = (agents: any[]) => (
    <table className="w-full">
      <thead className="bg-gray-50">
        <tr>
          <th className="p-3 text-left text-sm font-medium text-gray-500">Имя</th>
          <th className="p-3 text-left text-sm font-medium text-gray-500">Тип</th>
          <th className="p-3 text-left text-sm font-medium text-gray-500">Endpoint</th>
          <th className="p-3 text-left text-sm font-medium text-gray-500">Навыков</th>
          <th className="p-3 text-left text-sm font-medium text-gray-500">Статус</th>
          <th className="p-3"></th>
        </tr>
      </thead>
      <tbody>
        {agents.map((agent: any) => {
          const skills = safeParseJSON(agent.Skills?.raw, []);
          return (
            <tr key={agent.ID} className="border-t hover:bg-gray-50 cursor-pointer" onClick={() => setSelectedAgent(agent)}>
              <td className="p-3 text-sm font-medium">{agent.Name}</td>
              <td className="p-3 text-sm">{agent.AgentType}</td>
              <td className="p-3 text-sm font-mono">{agent.Endpoint}</td>
              <td className="p-3 text-sm">{Array.isArray(skills) ? skills.length : 0}</td>
              <td className="p-3 text-sm">
                <span className={`px-2 py-1 rounded text-xs ${agent.Status === 'online' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                  {agent.Status}
                </span>
              </td>
              <td className="p-3">
                {agent.DispatcherID !== null && agent.DispatcherID !== '00000000-0000-0000-0000-000000000000' && (
                  <button
                    onClick={(e) => { e.stopPropagation(); if (confirm('Удалить агента?')) deleteMutation.mutate(agent.ID); }}
                    className="text-red-500 hover:underline text-sm"
                  >
                    Удалить
                  </button>
                )}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Агенты</h1>
        <button onClick={() => setShowAdd(true)} className="bg-blue-600 text-white px-4 py-2 rounded-lg hover:bg-blue-700">
          + Добавить агента
        </button>
      </div>

      {showAdd && (
        <div className="bg-white p-4 rounded-lg shadow mb-4">
          <h2 className="font-bold mb-2">Добавить агента</h2>
          <p className="text-sm text-gray-500 mb-2">Введите endpoint агента (оркестратор проверит его доступность)</p>
          <div className="flex gap-2">
            <input type="text" value={newEndpoint} onChange={e => setNewEndpoint(e.target.value)}
              placeholder="http://100.93.170.55:9004" className="flex-1 border p-2 rounded" />
            <button onClick={() => addMutation.mutate(newEndpoint)} disabled={addMutation.isPending}
              className="bg-green-600 text-white px-4 py-2 rounded hover:bg-green-700 disabled:opacity-50">
              {addMutation.isPending ? 'Добавление...' : 'Добавить'}
            </button>
            <button onClick={() => setShowAdd(false)} className="text-gray-500 hover:underline px-2">Отмена</button>
          </div>
          {addMutation.isError && <p className="text-red-600 text-sm mt-2">Ошибка: {String(addMutation.error)}</p>}
          {addMutation.isSuccess && <p className="text-green-600 text-sm mt-2">Агент успешно добавлен!</p>}
        </div>
      )}

      <div className="flex gap-2 mb-4">
        {[{ key: 'all', label: `Все (${allAgents.length})` },
          { key: 'own', label: `Мои (${ownAgents.length})` },
          { key: 'common', label: `Общие (${commonAgents.length})` }].map(t => (
          <button key={t.key} onClick={() => setTab(t.key as any)}
            className={`px-4 py-1 rounded text-sm ${tab === t.key ? 'bg-blue-600 text-white' : 'bg-gray-200 hover:bg-gray-300'}`}>
            {t.label}
          </button>
        ))}
      </div>

      {isLoading ? <p className="p-6">Загрузка...</p> :
        displayed.length === 0 ? <div className="bg-white rounded-lg shadow p-6 text-center text-gray-500">Нет агентов</div> :
        <div className="bg-white rounded-lg shadow overflow-hidden">{renderTable(displayed)}</div>
      }

      {selectedAgent && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setSelectedAgent(null)}>
          <div className="bg-white rounded-lg shadow-xl p-6 max-w-lg w-full max-h-[80vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
            <h2 className="text-xl font-bold mb-4">{selectedAgent.Name}</h2>
            <div className="space-y-3 text-sm">
              <p><strong>Тип:</strong> {selectedAgent.AgentType}</p>
              <p><strong>Endpoint:</strong> <code>{selectedAgent.Endpoint}</code></p>
              <p><strong>Capabilities:</strong> {safeParseJSON(selectedAgent.Capabilities?.raw, []).join(', ') || '—'}</p>
              <div>
                <strong>Навыки:</strong>
                {safeParseJSON(selectedAgent.Skills?.raw, []).map((skill: any, i: number) => (
                  <div key={i} className="bg-gray-50 p-3 rounded mt-2">
                    <p className="font-medium">{skill.id}</p>
                    <p className="text-gray-500 text-xs">{skill.description}</p>
                    {skill.input_schema && (
                      <details className="mt-1">
                        <summary className="text-blue-600 cursor-pointer text-xs">Input Schema</summary>
                        <pre className="text-xs mt-1 bg-gray-100 p-2 rounded overflow-x-auto">{JSON.stringify(skill.input_schema, null, 2)}</pre>
                      </details>
                    )}
                    {skill.output_schema && (
                      <details className="mt-1">
                        <summary className="text-blue-600 cursor-pointer text-xs">Output Schema</summary>
                        <pre className="text-xs mt-1 bg-gray-100 p-2 rounded overflow-x-auto">{JSON.stringify(skill.output_schema, null, 2)}</pre>
                      </details>
                    )}
                  </div>
                ))}
              </div>
            </div>
            <button onClick={() => setSelectedAgent(null)} className="mt-4 bg-gray-200 px-4 py-2 rounded hover:bg-gray-300">Закрыть</button>
          </div>
        </div>
      )}
    </div>
  );
}