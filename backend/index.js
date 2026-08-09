const express = require('express');
const { Resend } = require('resend');
const http = require('http');
const { Server } = require('socket.io');
require('dotenv').config();

const app = express();
app.use(express.json());

const server = http.createServer(app);
const io = new Server(server, {
  cors: { origin: '*' },
});

const resend = new Resend(process.env.RESEND_API_KEY);

// --- Хранилище ---
const codes = new Map();
const messages = [];
const users = new Map();

function generateCode() {
  return Math.floor(100000 + Math.random() * 900000).toString();
}

// --- Авторизация ---
app.post('/api/send-code', async (req, res) => {
  const { email } = req.body;
  if (!email) return res.status(400).json({ error: 'Email обязателен' });
  const code = generateCode();
  codes.set(email, { code, expiresAt: Date.now() + 5 * 60 * 1000 });
  try {
    await resend.emails.send({
      from: 'onboarding@resend.dev',
      to: [email],
      subject: 'Код подтверждения для Zvonilka',
      html: `<h2>Ваш код: ${code}</h2><p>Действует 5 минут.</p>`,
    });
    res.json({ success: true });
  } catch (error) {
    console.error(error);
    res.status(500).json({ error: 'Ошибка отправки письма' });
  }
});

app.post('/api/verify-code', (req, res) => {
  const { email, code } = req.body;
  const record = codes.get(email);
  if (!record) return res.status(400).json({ error: 'Код не найден' });
  if (Date.now() > record.expiresAt) {
    codes.delete(email);
    return res.status(400).json({ error: 'Код истёк' });
  }
  if (record.code !== code) return res.status(400).json({ error: 'Неверный код' });
  codes.delete(email);
  const userId = email;
  if (!users.has(userId)) {
    users.set(userId, { email, socketId: null });
  }
  res.json({ success: true, userId });
});

// --- Получить историю ---
app.get('/api/messages', (req, res) => {
  res.json(messages);
});

// --- РЕДАКТИРОВАНИЕ (с поддержкой админа) ---
app.put('/api/messages/:id', (req, res) => {
  const { id } = req.params;
  const { text, userId } = req.body;
  if (!text) return res.status(400).json({ error: 'Текст обязателен' });

  const msgIndex = messages.findIndex(m => m.id === id);
  if (msgIndex === -1) return res.status(404).json({ error: 'Сообщение не найдено' });

  const isAdmin = userId === process.env.ADMIN_EMAIL;
  const isOwner = messages[msgIndex].userId === userId;

  if (!isOwner && !isAdmin) {
    return res.status(403).json({ error: 'Нет прав на редактирование' });
  }

  messages[msgIndex].text = text;
  messages[msgIndex].edited = true;
  messages[msgIndex].editedAt = new Date().toISOString();
  if (!isOwner) {
    messages[msgIndex].editedByAdmin = true;
  }

  io.emit('message-updated', messages[msgIndex]);
  res.json({ success: true, message: messages[msgIndex] });
});

// --- Socket.IO ---
io.on('connection', (socket) => {
  console.log('🔌 Новый клиент:', socket.id);
  socket.on('set-user', (userId) => {
    if (users.has(userId)) {
      users.set(userId, { ...users.get(userId), socketId: socket.id });
      console.log(`👤 Пользователь ${userId} подключён`);
    }
  });
  socket.on('new-message', (data) => {
    const { userId, text, displayName } = data;
    if (!userId || !text) return;
    const msg = {
      id: Date.now().toString(36) + Math.random().toString(36).substr(2, 5),
      text,
      userId,
      displayName: displayName || userId.split('@')[0],
      createdAt: new Date().toISOString(),
      edited: false,
    };
    messages.push(msg);
    io.emit('message', msg);
  });
  socket.on('disconnect', () => {
    console.log('❌ Клиент отключён:', socket.id);
  });
});

const PORT = process.env.PORT || 5000;
server.listen(PORT, () => console.log(`✅ Бэкенд запущен на порту ${PORT}`));