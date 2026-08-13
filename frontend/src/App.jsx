// src/App.jsx
import React, { useState, useEffect } from 'react';
import { ThemeProvider, useTheme } from './ThemeContext';
import AuthByEmail from './components/AuthByEmail';
import Chat from './components/Chat';
import './style.css';

function AppContent() {
    const [user, setUser] = useState(null);

    useEffect(() => {
        const token = localStorage.getItem('token');
        const userId = localStorage.getItem('userId');
        const userEmail = localStorage.getItem('userEmail');
        if (token && userId && userEmail) {
            setUser({ userId, email: userEmail, token });
        }
    }, []);

    const handleAuthSuccess = (email, userId, token) => {
        localStorage.setItem('token', token);
        localStorage.setItem('userId', userId);
        localStorage.setItem('userEmail', email);
        setUser({ userId, email, token });
    };

    const handleLogout = () => {
        localStorage.removeItem('token');
        localStorage.removeItem('userId');
        localStorage.removeItem('userEmail');
        setUser(null);
    };

    if (!user) {
        return <AuthByEmail onAuthSuccess={handleAuthSuccess} />;
    }

    return <Chat userId={user.userId} userEmail={user.email} onLogout={handleLogout} />;
}

function App() {
    return (
        <ThemeProvider>
            <AppContent />
        </ThemeProvider>
    );
}

export default App;