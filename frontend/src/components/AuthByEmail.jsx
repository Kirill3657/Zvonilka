// src/components/AuthByEmail.jsx
import React, { useState } from 'react';
import { useTheme } from '../ThemeContext';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:5000';

const AuthByEmail = ({ onAuthSuccess }) => {
    const { t, theme } = useTheme();
    const [email, setEmail] = useState('');
    const [code, setCode] = useState('');
    const [isCodeSent, setIsCodeSent] = useState(false);
    const [loading, setLoading] = useState(false);

    const handleSendCode = async () => {
        if (!email) return alert(t('error'));
        setLoading(true);
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
                alert(t('error') + ': ' + data.error);
            }
        } catch (error) {
            alert(t('reconnect'));
        } finally {
            setLoading(false);
        }
    };

    const handleVerifyCode = async () => {
        if (!code) return alert(t('error'));
        setLoading(true);
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
                alert(t('error') + ': ' + data.error);
            }
        } catch (error) {
            alert(t('reconnect'));
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="auth-container">
            <h2>{isCodeSent ? t('login') : t('register')}</h2>
            {!isCodeSent ? (
                <>
                    <input
                        type="email"
                        placeholder={t('emailPlaceholder')}
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                    />
                    <button onClick={handleSendCode} disabled={loading}>
                        {loading ? '...' : t('sendCode')}
                    </button>
                </>
            ) : (
                <>
                    <p style={{ textAlign: 'center', marginBottom: 10 }}>
                        {t('codeSent')} {email}
                    </p>
                    <input
                        type="text"
                        placeholder={t('codePlaceholder')}
                        value={code}
                        onChange={(e) => setCode(e.target.value)}
                    />
                    <button onClick={handleVerifyCode} disabled={loading}>
                        {loading ? '...' : t('verifyCode')}
                    </button>
                </>
            )}
            <div className="toggle-link">
                {isCodeSent ? (
                    <span onClick={() => setIsCodeSent(false)}>← {t('register')}</span>
                ) : (
                    <span onClick={() => alert('Уже есть аккаунт? Введи почту и получи код')}>
                        {t('login')}
                    </span>
                )}
            </div>
        </div>
    );
};

export default AuthByEmail;// src/components/AuthByEmail.jsx
import React, { useState } from 'react';
import { useTheme } from '../ThemeContext';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:5000';

const AuthByEmail = ({ onAuthSuccess }) => {
    const { t } = useTheme(); // убрали theme
    const [email, setEmail] = useState('');
    const [code, setCode] = useState('');
    const [isCodeSent, setIsCodeSent] = useState(false);
    const [loading, setLoading] = useState(false);

    const handleSendCode = async () => {
        if (!email) return alert(t('error'));
        setLoading(true);
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
                alert(t('error') + ': ' + data.error);
            }
        } catch (error) {
            alert(t('reconnect'));
        } finally {
            setLoading(false);
        }
    };

    const handleVerifyCode = async () => {
        if (!code) return alert(t('error'));
        setLoading(true);
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
                alert(t('error') + ': ' + data.error);
            }
        } catch (error) {
            alert(t('reconnect'));
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="auth-container">
            <h2>{isCodeSent ? t('login') : t('register')}</h2>
            {!isCodeSent ? (
                <>
                    <input
                        type="email"
                        placeholder={t('emailPlaceholder')}
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                    />
                    <button onClick={handleSendCode} disabled={loading}>
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
                    <input
                        type="text"
                        placeholder={t('codePlaceholder')}
                        value={code}
                        onChange={(e) => setCode(e.target.value)}
                    />
                    <button onClick={handleVerifyCode} disabled={loading}>
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