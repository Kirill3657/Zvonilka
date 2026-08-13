// src/components/ProfileSetup.jsx
import React, { useState } from 'react';
import { useTheme } from '../ThemeContext';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:5000';

const ProfileSetup = ({ userId, token, onComplete }) => {
    const { t } = useTheme();
    const [firstName, setFirstName] = useState('');
    const [lastName, setLastName] = useState('');
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setLoading(true);
        try {
            const res = await fetch(`${API_URL}/api/profile`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`,
                },
                body: JSON.stringify({ firstName, lastName }),
            });
            if (!res.ok) throw new Error('Ошибка сохранения');
            onComplete();
        } catch (error) {
            alert(error.message);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="auth-container">
            <h2>{t('welcome')}</h2>
            <p style={{ textAlign: 'center', marginBottom: 20 }}>Введите ваше имя (необязательно)</p>
            <form onSubmit={handleSubmit}>
                <input
                    type="text"
                    placeholder="Имя"
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    className="auth-input"
                />
                <input
                    type="text"
                    placeholder="Фамилия (необязательно)"
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    className="auth-input"
                />
                <button type="submit" disabled={loading} className="auth-btn">
                    {loading ? '...' : 'Продолжить'}
                </button>
            </form>
        </div>
    );
};

export default ProfileSetup;