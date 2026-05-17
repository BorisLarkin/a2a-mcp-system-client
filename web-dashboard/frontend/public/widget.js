(function() {
  // Конфигурация
  const API_URL = window.WIDGET_API_URL || '';
  const API_KEY = window.WIDGET_API_KEY || '';
  const HEADER_TITLE = window.WIDGET_HEADER_TITLE || 'Поддержка';
  const PLACEHOLDER = window.WIDGET_PLACEHOLDER || 'Опишите вашу проблему...';
  const OFFLINE_RESPONSE = 'Спасибо за обращение! Мы получили ваше сообщение и ответим при первой возможности.';

  // Создание DOM-элементов
  const style = document.createElement('style');
  style.textContent = `
    #support-widget * { box-sizing: border-box; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
    #support-widget-btn { position: fixed; bottom: 20px; right: 20px; width: 60px; height: 60px; border-radius: 50%; background: #3b82f6; color: white; border: none; cursor: pointer; font-size: 28px; z-index: 9999; box-shadow: 0 4px 12px rgba(0,0,0,0.2); transition: transform 0.2s; }
    #support-widget-btn:hover { transform: scale(1.1); }
    #support-widget-panel { position: fixed; bottom: 90px; right: 20px; width: 380px; max-width: calc(100vw - 40px); height: 520px; max-height: calc(100vh - 120px); background: white; border-radius: 12px; box-shadow: 0 8px 32px rgba(0,0,0,0.2); z-index: 9998; display: none; flex-direction: column; overflow: hidden; }
    #support-widget-panel.open { display: flex; }
    #support-widget-header { background: #3b82f6; color: white; padding: 14px 16px; font-weight: 600; font-size: 15px; }
    #support-widget-messages { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 10px; background: #f9fafb; }
    #support-widget-messages .msg { max-width: 85%; padding: 10px 14px; border-radius: 12px; font-size: 14px; line-height: 1.4; word-wrap: break-word; }
    #support-widget-messages .msg.user { align-self: flex-end; background: #3b82f6; color: white; border-bottom-right-radius: 4px; }
    #support-widget-messages .msg.agent { align-self: flex-start; background: white; color: #1f2937; border-bottom-left-radius: 4px; border: 1px solid #e5e7eb; }
    #support-widget-messages .msg.system { align-self: center; background: #f3f4f6; color: #6b7280; font-size: 12px; padding: 6px 12px; border-radius: 8px; max-width: 100%; }
    #support-widget-messages .btns { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 6px; }
    #support-widget-messages .btns button { padding: 6px 12px; border-radius: 6px; border: none; cursor: pointer; font-size: 12px; font-weight: 500; }
    #support-widget-messages .btns button.resolve { background: #10b981; color: white; }
    #support-widget-messages .btns button.escalate { background: #f59e0b; color: white; }
    #support-widget-input-area { display: flex; border-top: 1px solid #e5e7eb; padding: 10px; gap: 8px; }
    #support-widget-input { flex: 1; border: 1px solid #d1d5db; border-radius: 8px; padding: 10px 14px; font-size: 14px; outline: none; }
    #support-widget-input:focus { border-color: #3b82f6; }
    #support-widget-send { background: #3b82f6; color: white; border: none; border-radius: 8px; padding: 10px 16px; cursor: pointer; font-size: 14px; font-weight: 500; }
    #support-widget-send:disabled { opacity: 0.5; cursor: default; }
  `;
  document.head.appendChild(style);

  // Кнопка открытия
  const btn = document.createElement('button');
  btn.id = 'support-widget-btn';
  btn.innerHTML = '💬';
  document.body.appendChild(btn);

  // Панель чата
  const panel = document.createElement('div');
  panel.id = 'support-widget-panel';
  panel.innerHTML = `
    <div id="support-widget-header">${HEADER_TITLE}</div>
    <div id="support-widget-messages"></div>
    <div id="support-widget-input-area">
      <input id="support-widget-input" type="text" placeholder="${PLACEHOLDER}" />
      <button id="support-widget-send">➤</button>
    </div>
  `;
  document.body.appendChild(panel);

  const messagesEl = document.getElementById('support-widget-messages');
  const inputEl = document.getElementById('support-widget-input');
  const sendBtn = document.getElementById('support-widget-send');

  let isOpen = false;
  let ticketId = null;

  // Генерация ID клиента (сохраняется в localStorage)
  const getClientId = () => {
    let id = localStorage.getItem('widget_client_id');
    if (!id) {
      id = 'widget_' + Math.random().toString(36).substr(2, 9);
      localStorage.setItem('widget_client_id', id);
    }
    return id;
  };

  const clientId = getClientId();

  // Добавление сообщения
  const addMessage = (text, type = 'agent', buttons = null) => {
    const div = document.createElement('div');
    div.className = `msg ${type}`;
    div.textContent = text;
    if (buttons) {
      const btnsDiv = document.createElement('div');
      btnsDiv.className = 'btns';
      buttons.forEach(b => {
        const btn = document.createElement('button');
        btn.className = b.action;
        btn.textContent = b.label;
        btn.onclick = b.onClick;
        btnsDiv.appendChild(btn);
      });
      div.appendChild(btnsDiv);
    }
    messagesEl.appendChild(div);
    messagesEl.scrollTop = messagesEl.scrollHeight;
  };

  // Отправка сообщения
  const sendMessage = async () => {
    const text = inputEl.value.trim();
    if (!text) return;

    addMessage(text, 'user');
    inputEl.value = '';
    sendBtn.disabled = true;

    try {
      const res = await fetch(`${API_URL}/api/v1/public/tickets`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-API-Key': API_KEY },
        body: JSON.stringify({
          text: text,
          client_external_id: clientId,
          channel_type: 'web',
          metadata: { source: 'web_widget', client_id: clientId }
        })
      });

      if (res.ok) {
        const data = await res.json();
        ticketId = data.ticket_id;
        addMessage('⏳ Ваше обращение принято, ожидайте ответа...', 'system');
        pollTicketStatus(ticketId);
      } else {
        addMessage(OFFLINE_RESPONSE, 'agent');
      }
    } catch {
      addMessage(OFFLINE_RESPONSE, 'agent');
    } finally {
      sendBtn.disabled = false;
    }
  };

  // Polling статуса
  const pollTicketStatus = async (tid) => {
    const maxAttempts = 100;
    let attempts = 0;

    const check = async () => {
      attempts++;
      try {
        const res = await fetch(`${API_URL}/api/v1/public/tickets/${tid}`, {
          headers: { 'X-API-Key': API_KEY }
        });
        if (res.ok) {
          const data = await res.json();
          const ticket = data.ticket;
          const messages = data.messages || [];

          // Очищаем системные сообщения "ожидайте"
          const sysMsgs = messagesEl.querySelectorAll('.msg.system');
          sysMsgs.forEach(m => m.remove());

          // Показываем сообщения
          messages.forEach(m => {
            const msgId = m.ID || m.id;
            const existing = messagesEl.querySelector(`[data-msg-id="${msgId}"]`);
            if (!existing) {
              const senderType = (m.SenderType || m.sender_type || '').toLowerCase();
              if (senderType === 'client') return; // свои сообщения уже показаны
              
              const div = document.createElement('div');
              div.className = `msg ${senderType === 'ai' ? 'agent' : senderType === 'operator' ? 'agent' : 'agent'}`;
              div.setAttribute('data-msg-id', msgId);
              div.textContent = (senderType === 'operator' ? '👨‍💼 Оператор: ' : '') + (m.MessageText || m.message_text || '');
              
              messagesEl.appendChild(div);
            }
          });

          // Всегда показываем кнопки, если тикет в активном статусе
          const existingBtns = messagesEl.querySelectorAll('.btns');
          existingBtns.forEach(b => b.remove());
              
          // Показываем кнопки для всех активных статусов (не resolved/closed)
          if (ticket.Status !== 'resolved' && ticket.Status !== 'closed') {
              const existingBtns = messagesEl.querySelectorAll('.btns');
              existingBtns.forEach(b => b.remove());
          
              const btnsDiv = document.createElement('div');
              btnsDiv.className = 'btns';
              btnsDiv.style.padding = '0 16px';
              
              const resolvedBtn = document.createElement('button');
              resolvedBtn.className = 'resolve';
              resolvedBtn.textContent = '✅ Проблема решена';
              resolvedBtn.onclick = () => sendFeedback(tid, 'resolved');
              
              const escalateBtn = document.createElement('button');
              escalateBtn.className = 'escalate';
              escalateBtn.textContent = '👨‍💼 Нужна помощь';
              escalateBtn.onclick = () => sendFeedback(tid, 'escalate');
              
              btnsDiv.appendChild(resolvedBtn);
              btnsDiv.appendChild(escalateBtn);
              messagesEl.appendChild(btnsDiv);
          }
          messagesEl.scrollTop = messagesEl.scrollHeight;

          // Если тикет закрыт
          if (ticket.Status === 'resolved' || ticket.Status === 'closed') {
            addMessage('✅ Диалог завершён. Спасибо за обращение!', 'system');
            return;
          }

          //if (ticket.Status === 'waiting') {
            //addMessage('👨‍💼 Ваше обращение передано оператору. Ожидайте ответа.', 'system');
          //}
        }
      } catch (e) {
        console.error('Poll error:', e);
      }

      if (attempts < maxAttempts) {
        setTimeout(check, 3000);
      }
    };

    setTimeout(check, 2000);
  };

  // Отправка обратной связи
  const sendFeedback = async (tid, action) => {
    try {
      await fetch(`${API_URL}/api/v1/public/tickets/${tid}/feedback`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'X-API-Key': API_KEY },
        body: JSON.stringify({
          status: action === 'resolved' ? 'resolved' : 'waiting',
          feedback_status: action === 'resolved' ? 'resolved' : 'escalate'
        })
      });
      // Убираем кнопки
      const btns = messagesEl.querySelectorAll('.btns');
      btns.forEach(b => b.remove());
      addMessage(action === 'resolved' ? '✅ Спасибо за подтверждение!' : '👨‍💼 Оператор уже уведомлён.', 'system');
    } catch {
      addMessage('❌ Ошибка. Попробуйте позже.', 'system');
    }
  };

  // События
  btn.addEventListener('click', () => {
    isOpen = !isOpen;
    panel.classList.toggle('open', isOpen);
    btn.innerHTML = isOpen ? '✕' : '💬';
  });

  sendBtn.addEventListener('click', sendMessage);
  inputEl.addEventListener('keydown', e => {
    if (e.key === 'Enter') sendMessage();
  });
})();