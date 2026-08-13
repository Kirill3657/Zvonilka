// src/components/AdminPanel.jsx
import React, { useState, useEffect } from 'react';
import { useTheme } from '../ThemeContext';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:5000';

const AdminPanel = ({ token, onClose }) => {
    const { t } = useTheme();
    const [users, setUsers] = useState([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        fetch(`${API_URL}/api/users`, {
            headers: { 'Authorization': `Bearer ${token}` }
        })
            .then(res => res.json())
            .then(data => {
                setUsers(data);
                setLoading(false);
            })
            .catch(() => setLoading(false));
    }, [token]);

    const toggleAdmin = async (userId, currentAdminStatus) => {
        if (!window.confirm(`Сменить статус админа для ${userId}?`)) return;
        try {
            const res = await fetch(`${API_URL}/api/users/${userId}/admin`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`,
                },
                body: JSON.stringify({ isAdmin: !currentAdminStatus }),
            });
            if (!res.ok) throw new Error('Ошибка');
            setUsers(users.map(u => u.id === userId ? { ...u, isAdmin: !currentAdminStatus } : u));
        } catch (error) {
            alert('Ошибка обновления');
        }
    };

    return (
        <div className="admin-panel">
            <div className="admin-panel-header">
                <h2>👑 Админ-панель</h2>
                <button onClick={onClose} className="close-btn">✖</button>
            </div>
            {loading ? (
                <p>Загрузка...</p>
            ) : (
                <table className="admin-table">
                    <thead>
                        <tr>
                            <th>Email</th>
                            <th>Имя</th>
                            <th>Админ</th>
                            <th>Действие</th>
                        </tr>
                    </thead>
                    <tbody>
                        {users.map(u => (
                            <tr key={u.id}>
                                <td>{u.email}</td>
                                <td>{u.firstName || '—'}</td>
                                <td>{u.isAdmin ? '✅' : '❌'}</td>
                                <td>
                                    <button onClick={() => toggleAdmin(u.id, u.isAdmin)}>
                                        {u.isAdmin ? 'Снять' : 'Назначить'}
                                    </button>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            )}
        </div>
    );
};

export default AdminPanel;