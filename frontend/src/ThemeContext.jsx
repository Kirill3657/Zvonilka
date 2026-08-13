// src/ThemeContext.jsx
import React, { createContext, useState, useContext, useEffect } from 'react';

const ThemeContext = createContext();

export const ThemeProvider = ({ children }) => {
    // Загружаем сохранённые настройки из localStorage
    const savedTheme = localStorage.getItem('theme') || 'light';
    const savedLang = localStorage.getItem('lang') || 'ru';

    const [theme, setTheme] = useState(savedTheme);
    const [lang, setLang] = useState(savedLang);

    useEffect(() => {
        document.documentElement.setAttribute('data-theme', theme);
        localStorage.setItem('theme', theme);
    }, [theme]);

    useEffect(() => {
        localStorage.setItem('lang', lang);
    }, [lang]);

    const toggleTheme = () => {
        setTheme(prev => prev === 'light' ? 'dark' : 'light');
    };

    const toggleLang = () => {
        setLang(prev => prev === 'ru' ? 'en' : 'ru');
    };

    const t = (key) => {
        const translations = {
            ru: {
                login: 'Вход',
                register: 'Регистрация',
                emailPlaceholder: 'Введи свою почту',
                codePlaceholder: 'Введи код из письма',
                sendCode: 'Отправить код',
                verifyCode: 'Подтвердить',
                codeSent: 'Код отправлен на',
                error: 'Ошибка',
                reconnect: 'Нет соединения с сервером',
                logout: 'Выйти',
                send: 'Отправить',
                placeholder: 'Введите сообщение...',
                edit: 'Изменить',
                cancel: 'Отмена',
                save: 'Сохранить',
                admin: 'админ',
                edited: '(ред.)',
                copied: 'Код скопирован!',
                welcome: 'Добро пожаловать',
            },
            en: {
                login: 'Login',
                register: 'Register',
                emailPlaceholder: 'Enter your email',
                codePlaceholder: 'Enter code from email',
                sendCode: 'Send code',
                verifyCode: 'Verify',
                codeSent: 'Code sent to',
                error: 'Error',
                reconnect: 'No connection to server',
                logout: 'Logout',
                send: 'Send',
                placeholder: 'Type a message...',
                edit: 'Edit',
                cancel: 'Cancel',
                save: 'Save',
                admin: 'admin',
                edited: '(edited)',
                copied: 'Code copied!',
                welcome: 'Welcome',
            }
        };
        return translations[lang]?.[key] || key;
    };

    return (
        <ThemeContext.Provider value={{ theme, lang, toggleTheme, toggleLang, t }}>
            {children}
        </ThemeContext.Provider>
    );
};

export const useTheme = () => useContext(ThemeContext);