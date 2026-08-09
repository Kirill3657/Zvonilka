// src/components/AuthByEmail.jsx
import React, { useState } from 'react';

const AuthByEmail = ({ onAuthSuccess }) => {
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [isCodeSent, setIsCodeSent] = useState(false);
  const [loading, setLoading] = useState(false);

  // Отправка кода
  const handleSendCode = async () => {
    if (!email) return alert('Введите email');
    setLoading(true);
    try {
      const res = await fetch('/api/send-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      });
      const data = await res.json();
      if (data.success) {
        setIsCodeSent(true);
        alert('Код отправлен на почту!');
      } else {
        alert('Ошибка: ' + data.error);
      }
    } catch (error) {
      alert('Не удалось подключиться к серверу');
    } finally {
      setLoading(false);
    }
  };

  // Проверка кода
  const handleVerifyCode = async () => {
    if (!code) return alert('Введите код');
    setLoading(true);
    try {
      const res = await fetch('/api/verify-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, code }),
      });
      const data = await res.json();
      if (data.success) {
        alert('Код верный! Вход выполнен.');
        // Передаём в App и email, и userId
        onAuthSuccess(email, data.userId);
      } else {
        alert('Ошибка: ' + data.error);
      }
    } catch (error) {
      alert('Ошибка соединения с сервером');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ maxWidth: 400, margin: '100px auto', padding: 20, border: '1px solid #ccc', borderRadius: 8 }}>
      <h2 style={{ textAlign: 'center' }}>Вход в Zvonilka</h2>
      {!isCodeSent ? (
        <>
          <input
            type="email"
            placeholder="Введи свою почту"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            style={{ width: '100%', padding: 10, marginBottom: 10, borderRadius: 4, border: '1px solid #ccc' }}
          />
          <button
            onClick={handleSendCode}
            disabled={loading}
            style={{ width: '100%', padding: 10, background: '#007bff', color: 'white', border: 'none', borderRadius: 4, cursor: 'pointer' }}
          >
            {loading ? 'Отправка...' : 'Отправить код'}
          </button>
        </>
      ) : (
        <>
          <p style={{ textAlign: 'center' }}>Код отправлен на {email}</p>
          <input
            type="text"
            placeholder="Введи код из письма"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            style={{ width: '100%', padding: 10, marginBottom: 10, borderRadius: 4, border: '1px solid #ccc' }}
          />
          <button
            onClick={handleVerifyCode}
            disabled={loading}
            style={{ width: '100%', padding: 10, background: '#28a745', color: 'white', border: 'none', borderRadius: 4, cursor: 'pointer' }}
          >
            {loading ? 'Проверка...' : 'Подтвердить'}
          </button>
        </>
      )}
    </div>
  );
};

export default AuthByEmail;