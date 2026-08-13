// src/components/AuthByEmail.jsx
import React, { useState, useRef, useEffect } from 'react';
import { useTheme } from '../ThemeContext';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:5000';

const AuthByEmail = ({ onAuthSuccess }) => {
    const { t, theme, toggleTheme, lang, toggleLang } = useTheme();
    const [email, setEmail] = useState('');
    const [codeDigits, setCodeDigits] = useState(['', '', '', '', '', '']);
    const [isCodeSent, setIsCodeSent] = useState(false);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');
    const inputRefs = useRef([]);

    // Авто-переход между инпутами
    useEffect(() => {
        if (isCodeSent && inputRefs.current[0]) {
            inputRefs.current[0].focus();
        }
    }, [isCodeSent]);

    const handleCodeChange = (index, value) => {
        if (!/^\d*$/.test(value)) return; // только цифры
        const newDigits = [...codeDigits];
        newDigits[index] = value.slice(-1);
        setCodeDigits(newDigits);
        setError('');

        // Переход к следующему полю
        if (value && index < 5) {
            inputRefs.current[index + 1].focus();
        }
    };

    const handleKeyDown = (index, e) => {
        if (e.key === 'Backspace' && !codeDigits[index] && index > 0) {
            inputRefs.current[index - 1].focus();
        }
    };

    const handlePaste = (e) => {
        e.preventDefault();
        const paste = e.clipboardData.getData('text').slice(0, 6);
        if (/^\d{6}$/.test(paste)) {
            const digits = paste.split('');
            setCodeDigits(digits);
            // фокус на последнее поле
            if (inputRefs.current[5]) inputRefs.current[5].focus();
        }
    };

    const getCode = () => codeDigits.join('');

    const handleSendCode = async () => {
        if (!email) return setError(t('error'));
        setLoading(true);
        setError('');
        try {
            const res = await fetch(`${API_URL}/api/send-code`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email }),
            });
            const data = await res.json();
            if (data.success) {
                setIsCodeSent(true);
                alert(t('codeSent') + ' ' + email);
            } else {
                setError(data.error || t('error'));
            }
        } catch (error) {
            setError(t('reconnect'));
        } finally {
            setLoading(false);
        }
    };

    const handleVerifyCode = async () => {
        const code = getCode();
        if (code.length !== 6) return setError(t('error'));
        setLoading(true);
        setError('');
        try {
            const res = await fetch(`${API_URL}/api/verify-code`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email, code }),
            });
            const data = await res.json();
            if (data.success) {
                onAuthSuccess(email, data.userId, data.token);
            } else {
                setError(data.error || t('error'));
            }
        } catch (error) {
            setError(t('reconnect'));
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="auth-container">
            <div className="auth-header">
                <h2>{isCodeSent ? t('login') : t('register')}</h2>
                <div className="auth-actions">
                    <button onClick={toggleTheme} title={theme === 'light' ? 'Тёмная тема' : 'Светлая тема'}>
                        {theme === 'light' ? '🌙' : '☀️'}
                    </button>
                    <button onClick={toggleLang} title={lang === 'ru' ? 'EN' : 'RU'}>
                        {lang === 'ru' ? 'EN' : 'RU'}
                    </button>
                </div>
            </div>
            {!isCodeSent ? (
                <>
                    <input
                        type="email"
                        placeholder={t('emailPlaceholder')}
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        className="auth-input"
                    />
                    <button onClick={handleSendCode} disabled={loading} className="auth-btn">
                        {loading ? '...' : t('sendCode')}
                    </button>
                    <div className="toggle-link">
                        <span onClick={() => alert('Введи почту и получи код')}>{t('login')}</span>
                    </div>
                </>
            ) : (
                <>
                    <p style={{ textAlign: 'center', marginBottom: 10 }}>
                        {t('codeSent')} {email}
                    </p>
                    <div className="code-inputs" onPaste={handlePaste}>
                        {codeDigits.map((digit, idx) => (
                            <input
                                key={idx}
                                ref={(el) => inputRefs.current[idx] = el}
                                type="text"
                                maxLength="1"
                                value={digit}
                                onChange={(e) => handleCodeChange(idx, e.target.value)}
                                onKeyDown={(e) => handleKeyDown(idx, e)}
                                className="code-digit"
                                autoComplete="off"
                            />
                        ))}
                    </div>
                    {error && <p className="auth-error">{error}</p>}
                    <button onClick={handleVerifyCode} disabled={loading} className="auth-btn">
                        {loading ? '...' : t('verifyCode')}
                    </button>
                    <div className="toggle-link">
                        <span onClick={() => setIsCodeSent(false)}>← {t('register')}</span>
                    </div>
                </>
            )}
        </div>
    );
};

export default AuthByEmail;