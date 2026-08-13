// src/components/Chat.jsx
import React, { useState, useEffect, useRef } from 'react';
import { useTheme } from '../ThemeContext';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:5000';

const Chat = ({ userId, userEmail, token, onLogout, isAdmin }) => {
    const { theme, toggleTheme, lang, toggleLang, t } = useTheme();
    const [messages, setMessages] = useState([]);
    const [inputText, setInputText] = useState('');
    const [editingId, setEditingId] = useState(null);
    const [editText, setEditText] = useState('');
    const wsRef = useRef(null);

    // --- WebSocket и загрузка истории ---
    useEffect(() => {
        const wsUrl = API_URL.replace(/^http/, 'ws') + '/ws';
        wsRef.current = new WebSocket(wsUrl);
        wsRef.current.onopen = () => {
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
            } catch (e) { console.error('WebSocket parse error', e); }
        };
        wsRef.current.onerror = () => console.error('WebSocket error');
        wsRef.current.onclose = () => console.log('WebSocket closed');

        fetch(`${API_URL}/api/messages`, {
            headers: { 'Authorization': `Bearer ${token}` }
        })
            .then(res => res.json())
            .then(data => {
                if (Array.isArray(data)) setMessages(data);
                else setMessages([]);
            })
            .catch(() => setMessages([]));

        return () => {
            if (wsRef.current) wsRef.current.close();
        };
    }, [userId, token]);

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
            alert(t('reconnect'));
        }
    };

    // --- Редактирование ---
    const startEdit = (msg) => {
        setEditingId(msg.id);
        setEditText(msg.text);
    };
    const cancelEdit = () => {
        setEditingId(null);
        setEditText('');
    };
    const saveEdit = async () => {
        if (!editText.trim()) return;
        try {
            const res = await fetch(`${API_URL}/api/messages/${editingId}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`,
                },
                body: JSON.stringify({ text: editText, userId }),
            });
            if (!res.ok) throw new Error('Edit failed');
            setEditingId(null);
            setEditText('');
        } catch (error) {
            alert(t('error'));
        }
    };

    const handleCopy = (text) => {
        navigator.clipboard.writeText(text).then(() => alert(t('copied')));
    };

    // --- Выход ---
    const handleLogout = () => {
        localStorage.clear();
        onLogout();
    };

    // --- Открыть админ-панель на субдомене ---
    const openAdminPanel = () => {
        window.open('https://admin.zvonilka.site', '_blank');
    };

    return (
        <div className="chat-container">
            <header className="chat-header">
                <h2>📱 Zvonilka</h2>
                <div className="user-info">
                    <span>{userEmail} {isAdmin && '👑'}</span>
                    <div className="actions">
                        <button onClick={toggleTheme} title={theme === 'light' ? 'Тёмная тема' : 'Светлая тема'}>
                            {theme === 'light' ? '🌙' : '☀️'}
                        </button>
                        <button onClick={toggleLang} title={lang === 'ru' ? 'EN' : 'RU'}>
                            {lang === 'ru' ? 'EN' : 'RU'}
                        </button>
                        {isAdmin && (
                            <button onClick={openAdminPanel} className="admin-btn" title="Админ-панель">
                                ⚙️
                            </button>
                        )}
                        <button className="logout-btn" onClick={handleLogout}>{t('logout')}</button>
                    </div>
                </div>
            </header>

            <div className="messages-window">
                {messages.map((msg) => {
                    const isOwn = msg.userId === userId;
                    const avatarLetter = msg.displayName?.charAt(0)?.toUpperCase() || '?';
                    // Для админа не показываем пометку "ред."
                    const showEdited = msg.edited && !isAdmin;
                    return (
                        <div key={msg.id} className={`message-row ${isOwn ? 'own' : ''}`}>
                            {!isOwn && <div className="message-avatar" style={{ background: isAdmin ? '#e67e22' : 'var(--accent)' }}>{avatarLetter}</div>}
                            <div className="message-bubble">
                                <div className="message-meta">
                                    <span>{msg.displayName}</span>
                                    {msg.editedByAdmin && <span> {t('admin')}</span>}
                                    {showEdited && <span> {t('edited')}</span>}
                                </div>
                                {editingId === msg.id ? (
                                    <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                                        <input
                                            value={editText}
                                            onChange={(e) => setEditText(e.target.value)}
                                            style={{ flex: 1, padding: 6, borderRadius: 8, border: '1px solid var(--border-color)', background: 'var(--bg-input)', color: 'var(--text-primary)' }}
                                            autoFocus
                                        />
                                        <button onClick={saveEdit} style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: 18 }}>💾</button>
                                        <button onClick={cancelEdit} style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: 18 }}>✖️</button>
                                    </div>
                                ) : (
                                    <>
                                        <div>{msg.text}</div>
                                        <div className="message-actions">
                                            {(isOwn || isAdmin) && (
                                                <button onClick={() => startEdit(msg)}>✏️</button>
                                            )}
                                            <button onClick={() => handleCopy(msg.text)}>📋</button>
                                        </div>
                                    </>
                                )}
                            </div>
                            {isOwn && <div className="message-avatar" style={{ background: 'var(--accent)' }}>{avatarLetter}</div>}
                        </div>
                    );
                })}
            </div>

            <div className="chat-input">
                <input
                    value={inputText}
                    onChange={(e) => setInputText(e.target.value)}
                    placeholder={t('placeholder')}
                    onKeyDown={(e) => e.key === 'Enter' && sendMessage()}
                />
                <button onClick={sendMessage}>{t('send')}</button>
            </div>
        </div>
    );
};

export default Chat;