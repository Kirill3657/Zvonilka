// src/App.jsx
import React, { useState, useEffect } from 'react';
import { ThemeProvider } from './ThemeContext';
import AuthByEmail from './components/AuthByEmail';
import ProfileSetup from './components/ProfileSetup';
import Chat from './components/Chat';
import AdminPanel from './components/AdminPanel';
import './css/style.css';

function AppContent() {
    const [user, setUser] = useState(null);
    const [showProfileSetup, setShowProfileSetup] = useState(false);
    const [isAdmin, setIsAdmin] = useState(false);
    const [isAdminSubdomain, setIsAdminSubdomain] = useState(false);

    useEffect(() => {
        // Определяем, зашли ли мы на admin.zvonilka.site
        const hostname = window.location.hostname;
        setIsAdminSubdomain(hostname === 'admin.zvonilka.site');

        const token = localStorage.getItem('token');
        const userId = localStorage.getItem('userId');
        const userEmail = localStorage.getItem('userEmail');
        const firstName = localStorage.getItem('firstName');

        if (token && userId && userEmail) {
            // Получаем профиль пользователя
            fetch(`${process.env.REACT_APP_API_URL || 'http://localhost:5000'}/api/profile`, {
                headers: { 'Authorization': `Bearer ${token}` }
            })
                .then(res => res.json())
                .then(data => {
                    setIsAdmin(data.isAdmin || false);
                    // Если мы на админ-субдомене и пользователь админ – показываем админ-панель
                    if (hostname === 'admin.zvonilka.site' && data.isAdmin) {
                        setUser({ userId, email: userEmail, token });
                        return;
                    }
                    // Если на админ-субдомене, но не админ – редирект
                    if (hostname === 'admin.zvonilka.site' && !data.isAdmin) {
                        window.location.href = 'https://zvonilka.site';
                        return;
                    }
                    // Иначе обычный пользователь
                    if (data.firstName) {
                        localStorage.setItem('firstName', data.firstName);
                        setShowProfileSetup(false);
                    } else {
                        setShowProfileSetup(true);
                    }
                    setUser({ userId, email: userEmail, token });
                })
                .catch(() => {
                    setUser({ userId, email: userEmail, token });
                });
        }
    }, []);

    const handleAuthSuccess = (email, userId, token) => {
        localStorage.setItem('token', token);
        localStorage.setItem('userId', userId);
        localStorage.setItem('userEmail', email);
        setUser({ userId, email, token });
        // Загружаем профиль
        fetch(`${process.env.REACT_APP_API_URL || 'http://localhost:5000'}/api/profile`, {
            headers: { 'Authorization': `Bearer ${token}` }
        })
            .then(res => res.json())
            .then(data => {
                setIsAdmin(data.isAdmin || false);
                if (data.firstName) {
                    localStorage.setItem('firstName', data.firstName);
                    setShowProfileSetup(false);
                } else {
                    setShowProfileSetup(true);
                }
            })
            .catch(() => setShowProfileSetup(true));
    };

    const handleProfileComplete = () => {
        setShowProfileSetup(false);
    };

    const handleLogout = () => {
        localStorage.clear();
        setUser(null);
        setShowProfileSetup(false);
        setIsAdmin(false);
    };

    if (!user) {
        return <AuthByEmail onAuthSuccess={handleAuthSuccess} />;
    }

    // Если на админ-субдомене и админ – показываем админ-панель
    if (isAdminSubdomain && isAdmin) {
        return <AdminPanel token={user.token} onClose={() => {
            window.location.href = 'https://zvonilka.site';
        }} />;
    }

    // Если на админ-субдомене, но не админ (редирект уже сделан, но на всякий случай)
    if (isAdminSubdomain && !isAdmin) {
        window.location.href = 'https://zvonilka.site';
        return null;
    }

    if (showProfileSetup) {
        return <ProfileSetup userId={user.userId} token={user.token} onComplete={handleProfileComplete} />;
    }

    return <Chat userId={user.userId} userEmail={user.email} token={user.token} onLogout={handleLogout} isAdmin={isAdmin} />;
}

function App() {
    return (
        <ThemeProvider>
            <AppContent />
        </ThemeProvider>
    );
}

export default App;