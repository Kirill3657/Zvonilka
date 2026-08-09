// src/components/Chat.jsx
import React, { useState, useEffect, useRef } from 'react';
import io from 'socket.io-client';

// === Адрес бэкенда: берём из переменной окружения, или localhost для разработки ===
const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:5000';

// === Администратор (замени email на свой, если нужно) ===
const ADMIN_EMAIL = 'wwwkirillstarcraft@gmail.com';

const Chat = ({ userId, userEmail }) => {
  const [messages, setMessages] = useState([]);
  const [inputText, setInputText] = useState('');
  const [editingId, setEditingId] = useState(null);
  const [editText, setEditText] = useState('');
  const socketRef = useRef(null);

  // Проверка, является ли пользователь админом
  const isAdmin = userEmail === ADMIN_EMAIL;

  // Подключение к WebSocket и загрузка истории
  useEffect(() => {
    // Создаём соединение с бэкендом
    socketRef.current = io(API_URL);
    socketRef.current.emit('set-user', userId);

    // Приём новых сообщений
    socketRef.current.on('message', (msg) => {
      setMessages((prev) => [...prev, msg]);
    });

    // Приём обновлений (редактирование)
    socketRef.current.on('message-updated', (updatedMsg) => {
      setMessages((prev) =>
        prev.map((m) => (m.id === updatedMsg.id ? updatedMsg : m))
      );
    });

    // Загрузка истории сообщений
    fetch(`${API_URL}/api/messages`)
      .then((res) => res.json())
      .then((data) => setMessages(data))
      .catch((err) => console.error('Ошибка загрузки истории:', err));

    // Отключаем сокет при размонтировании
    return () => {
      socketRef.current.disconnect();
    };
  }, [userId]);

  // Отправка нового сообщения
  const sendMessage = () => {
    if (!inputText.trim()) return;
    socketRef.current.emit('new-message', {
      userId,
      text: inputText,
      displayName: userEmail.split('@')[0],
    });
    setInputText('');
  };

  // Начать редактирование
  const startEdit = (msg) => {
    setEditingId(msg.id);
    setEditText(msg.text);
  };

  // Отменить редактирование
  const cancelEdit = () => {
    setEditingId(null);
    setEditText('');
  };

  // Сохранить изменения
  const saveEdit = async () => {
    if (!editText.trim()) return;
    try {
      const res = await fetch(`${API_URL}/api/messages/${editingId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: editText, userId }),
      });
      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.error || 'Не удалось отредактировать');
      }
      setEditingId(null);
      setEditText('');
    } catch (error) {
      alert('Ошибка: ' + error.message);
    }
  };

  // Выход
  const handleLogout = () => {
    window.location.reload();
  };

  return (
    <div style={{ maxWidth: 600, margin: '0 auto', padding: 20 }}>
      {/* Шапка */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>🔥 Zvonilka</h2>
        <div>
          <span style={{ marginRight: 10 }}>{userEmail} {isAdmin && '👑'}</span>
          <button
            onClick={handleLogout}
            style={{
              padding: '5px 15px',
              background: '#dc3545',
              color: 'white',
              border: 'none',
              borderRadius: 4,
              cursor: 'pointer',
            }}
          >
            Выйти
          </button>
        </div>
      </div>

      {/* Окно сообщений */}
      <div
        style={{
          border: '1px solid #ccc',
          height: 400,
          overflowY: 'scroll',
          padding: 10,
          background: '#f9f9f9',
          borderRadius: 8,
          marginTop: 10,
        }}
      >
        {messages.map((msg) => (
          <div
            key={msg.id}
            style={{
              margin: '8px 0',
              textAlign: msg.userId === userId ? 'right' : 'left',
            }}
          >
            {editingId === msg.id ? (
              <div
                style={{
                  display: 'inline-block',
                  background: '#fff',
                  padding: '8px 12px',
                  borderRadius: 12,
                  border: '1px solid #007bff',
                }}
              >
                <input
                  value={editText}
                  onChange={(e) => setEditText(e.target.value)}
                  style={{ width: '200px', padding: '4px' }}
                  autoFocus
                />
                <button onClick={saveEdit} style={{ marginLeft: 8 }}>💾</button>
                <button onClick={cancelEdit} style={{ marginLeft: 4 }}>✖️</button>
              </div>
            ) : (
              <div
                style={{
                  display: 'inline-block',
                  background: msg.userId === userId ? '#007bff' : '#e9ecef',
                  color: msg.userId === userId ? '#fff' : '#000',
                  padding: '8px 12px',
                  borderRadius: 12,
                  maxWidth: '80%',
                }}
              >
                <div style={{ fontSize: 12, opacity: 0.7 }}>
                  {msg.displayName}
                  {msg.editedByAdmin && ' (отредактировано админом)'}
                </div>
                {msg.text}
                {msg.edited && <span style={{ fontSize: 10, opacity: 0.5, marginLeft: 6 }}>(ред.)</span>}
                {(msg.userId === userId || isAdmin) && (
                  <button
                    onClick={() => startEdit(msg)}
                    style={{
                      marginLeft: 8,
                      background: 'none',
                      border: 'none',
                      cursor: 'pointer',
                      fontSize: 12,
                    }}
                  >
                    ✏️
                  </button>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Форма отправки */}
      <div style={{ display: 'flex', marginTop: 10, gap: 10 }}>
        <input
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          placeholder="Введите сообщение..."
          style={{ flex: 1, padding: 10, borderRadius: 20, border: '1px solid #ccc' }}
          onKeyDown={(e) => e.key === 'Enter' && sendMessage()}
        />
        <button
          onClick={sendMessage}
          style={{
            padding: '10px 20px',
            background: '#007bff',
            color: '#fff',
            border: 'none',
            borderRadius: 20,
            cursor: 'pointer',
          }}
        >
          Отправить
        </button>
      </div>
    </div>
  );
};

export default Chat;