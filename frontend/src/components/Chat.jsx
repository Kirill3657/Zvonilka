// src/components/Chat.jsx
import React, { useState, useEffect, useRef } from 'react';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:5000';
const ADMIN_EMAIL = 'wwwkirillstarcraft@gmail.com';

const Chat = ({ userId, userEmail }) => {
  const [messages, setMessages] = useState([]); // всегда массив
  const [inputText, setInputText] = useState('');
  const [editingId, setEditingId] = useState(null);
  const [editText, setEditText] = useState('');
  const wsRef = useRef(null);
  const isAdmin = userEmail === ADMIN_EMAIL;

  // --- WebSocket ---
  useEffect(() => {
    const wsUrl = API_URL.replace(/^http/, 'ws') + '/ws';
    wsRef.current = new WebSocket(wsUrl);

    wsRef.current.onopen = () => {
      console.log('🔌 WebSocket подключён');
      wsRef.current.send(JSON.stringify({ command: 'set-user', userId }));
    };

    wsRef.current.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === 'message') {
          setMessages(prev => [...prev, data.payload]);
        } else if (data.type === 'message-updated') {
          setMessages(prev => prev.map(m => m.id === data.payload.id ? data.payload : m));
        }
      } catch (e) { console.error('Ошибка парсинга WebSocket', e); }
    };

    wsRef.current.onerror = (error) => {
      console.error('❌ Ошибка WebSocket:', error);
    };

    wsRef.current.onclose = () => {
      console.log('🔌 WebSocket отключён');
    };

    // --- Загрузка истории ---
    fetch(`${API_URL}/api/messages`)
      .then(res => res.json())
      .then(data => {
        if (Array.isArray(data)) {
          setMessages(data);
        } else {
          console.warn('Неверный формат данных:', data);
          setMessages([]);
        }
      })
      .catch(err => {
        console.error('Ошибка загрузки истории:', err);
        setMessages([]);
      });

    return () => {
      if (wsRef.current) wsRef.current.close();
    };
  }, [userId]);

  // --- Отправка ---
  const sendMessage = () => {
    if (!inputText.trim()) return;
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        command: 'new-message',
        text: inputText,
        displayName: userEmail.split('@')[0],
      }));
      setInputText('');
    } else {
      alert('Нет соединения с сервером');
    }
  };

  // --- Редактирование (HTTP) ---
  const startEdit = (msg) => { setEditingId(msg.id); setEditText(msg.text); };
  const cancelEdit = () => { setEditingId(null); setEditText(''); };
  const saveEdit = async () => {
    if (!editText.trim()) return;
    try {
      const res = await fetch(`${API_URL}/api/messages/${editingId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: editText, userId }),
      });
      if (!res.ok) throw new Error('Не удалось отредактировать');
      setEditingId(null);
      setEditText('');
    } catch (error) {
      alert('Ошибка: ' + error.message);
    }
  };

  const handleLogout = () => window.location.reload();

  // --- Рендер ---
  return (
    <div style={{ maxWidth: 600, margin: '0 auto', padding: 20 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>🔥 Zvonilka</h2>
        <div>
          <span style={{ marginRight: 10 }}>{userEmail} {isAdmin && '👑'}</span>
          <button onClick={handleLogout} style={{ padding: '5px 15px', background: '#dc3545', color: 'white', border: 'none', borderRadius: 4, cursor: 'pointer' }}>
            Выйти
          </button>
        </div>
      </div>

      <div style={{ border: '1px solid #ccc', height: 400, overflowY: 'scroll', padding: 10, background: '#f9f9f9', borderRadius: 8, marginTop: 10 }}>
        {/* Используем (messages || []).map для защиты */}
        {(messages || []).map((msg) => (
          <div key={msg.id} style={{ margin: '8px 0', textAlign: msg.userId === userId ? 'right' : 'left' }}>
            {editingId === msg.id ? (
              <div style={{ display: 'inline-block', background: '#fff', padding: '8px 12px', borderRadius: 12, border: '1px solid #007bff' }}>
                <input value={editText} onChange={(e) => setEditText(e.target.value)} style={{ width: '200px', padding: '4px' }} autoFocus />
                <button onClick={saveEdit} style={{ marginLeft: 8 }}>💾</button>
                <button onClick={cancelEdit} style={{ marginLeft: 4 }}>✖️</button>
              </div>
            ) : (
              <div style={{ display: 'inline-block', background: msg.userId === userId ? '#007bff' : '#e9ecef', color: msg.userId === userId ? '#fff' : '#000', padding: '8px 12px', borderRadius: 12, maxWidth: '80%' }}>
                <div style={{ fontSize: 12, opacity: 0.7 }}>{msg.displayName}{msg.editedByAdmin && ' (админ)'}</div>
                {msg.text}
                {msg.edited && <span style={{ fontSize: 10, opacity: 0.5, marginLeft: 6 }}>(ред.)</span>}
                {(msg.userId === userId || isAdmin) && (
                  <button onClick={() => startEdit(msg)} style={{ marginLeft: 8, background: 'none', border: 'none', cursor: 'pointer', fontSize: 12 }}>✏️</button>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      <div style={{ display: 'flex', marginTop: 10, gap: 10 }}>
        <input
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          placeholder="Введите сообщение..."
          style={{ flex: 1, padding: 10, borderRadius: 20, border: '1px solid #ccc' }}
          onKeyDown={(e) => e.key === 'Enter' && sendMessage()}
        />
        <button onClick={sendMessage} style={{ padding: '10px 20px', background: '#007bff', color: '#fff', border: 'none', borderRadius: 20, cursor: 'pointer' }}>
          Отправить
        </button>
      </div>
    </div>
  );
};

export default Chat;