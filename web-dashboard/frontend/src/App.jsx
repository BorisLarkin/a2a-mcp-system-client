// ./web-dashboard/frontend/src/App.jsx
import React, { useState, useEffect } from 'react'

function App() {
  const [health, setHealth] = useState('checking...')

  useEffect(() => {
    fetch('/api/health')
      .then(res => res.json())
      .then(data => setHealth(data.status))
      .catch(() => setHealth('unavailable'))
  }, [])

  return (
    <div style={{ padding: '20px', fontFamily: 'Arial' }}>
      <h1>🚀 Диспетчерская поддержки</h1>
      <p>Статус API: <strong>{health}</strong></p>
      <p>Система успешно запущена!</p>
    </div>
  )
}

export default App