package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/resend/resend-go/v2"
)

// --- Структуры ---
type Message struct {
	ID            string    `json:"id"`
	Text          string    `json:"text"`
	UserID        string    `json:"userId"`
	DisplayName   string    `json:"displayName"`
	CreatedAt     time.Time `json:"createdAt"`
	Edited        bool      `json:"edited"`
	EditedByAdmin bool      `json:"editedByAdmin"`
}

type CodeRecord struct {
	Code      string
	ExpiresAt time.Time
}

// --- Глобальные переменные ---
var (
	db          *sql.DB
	codesStore  = make(map[string]CodeRecord)
	codesMutex  sync.RWMutex
	upgrader    = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clients     = make(map[*websocket.Conn]string)
	clientsMux  sync.RWMutex
	resendKey   string
	adminEmail  string
	jwtSecret   []byte
)

// --- JWT ---
func generateToken(userId string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": userId,
		"exp":    time.Now().Add(time.Hour * 24 * 7).Unix(),
	})
	return token.SignedString(jwtSecret)
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userId, ok := claims["userId"].(string)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), "userId", userId)
		next(w, r.WithContext(ctx))
	}
}

// --- Генерация кода ---
func generateCode() string {
	return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
}

// --- База данных ---
func initDB() error {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		return fmt.Errorf("DATABASE_URL не задан")
	}
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("ошибка открытия БД: %w", err)
	}
	if err = db.Ping(); err != nil {
		return fmt.Errorf("не могу подключиться к БД: %w", err)
	}
	log.Println("✅ Подключение к БД успешно")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			theme TEXT DEFAULT 'light',
			lang TEXT DEFAULT 'ru'
		);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			text TEXT NOT NULL,
			userId TEXT NOT NULL,
			displayName TEXT NOT NULL,
			createdAt TIMESTAMP DEFAULT NOW(),
			edited BOOLEAN DEFAULT FALSE,
			editedByAdmin BOOLEAN DEFAULT FALSE
		);
	`)
	if err != nil {
		return fmt.Errorf("ошибка создания таблиц: %w", err)
	}
	log.Println("✅ Таблицы созданы (или уже существуют)")
	return nil
}

// --- Хендлеры ---

// Отправка кода
func sendCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Email string }
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Email == "" {
		http.Error(w, "Email обязателен", http.StatusBadRequest)
		return
	}
	code := generateCode()
	codesMutex.Lock()
	codesStore[req.Email] = CodeRecord{Code: code, ExpiresAt: time.Now().Add(5 * time.Minute)}
	codesMutex.Unlock()

	client := resend.NewClient(resendKey)
	htmlContent := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
	  <meta charset="UTF-8">
	  <meta name="viewport" content="width=device-width, initial-scale=1.0">
	  <title>Код подтверждения</title>
	  <style>
		body { font-family: 'Segoe UI', Arial, sans-serif; background: #f0f2f5; margin: 0; padding: 20px; }
		.container { max-width: 480px; margin: 0 auto; background: #ffffff; padding: 40px 30px; border-radius: 16px; box-shadow: 0 8px 24px rgba(0,0,0,0.06); text-align: center; }
		.logo { font-size: 48px; margin-bottom: 10px; }
		h1 { font-size: 24px; color: #1a1a2e; margin: 0 0 8px 0; font-weight: 700; }
		.sub { color: #555; font-size: 16px; margin-bottom: 30px; }
		.code { font-size: 52px; font-weight: 700; letter-spacing: 8px; color: #1a73e8; background: #e8f0fe; padding: 12px 20px; border-radius: 12px; display: inline-block; margin: 20px 0; font-family: 'Courier New', monospace; }
		.info { font-size: 14px; color: #666; margin: 20px 0 30px; }
		.footer { font-size: 13px; color: #aaa; border-top: 1px solid #eee; padding-top: 20px; margin-top: 20px; }
		.footer a { color: #1a73e8; text-decoration: none; }
	  </style>
	</head>
	<body>
	  <div class="container">
		<div class="logo">📱</div>
		<h1>Zvonilka</h1>
		<p class="sub">Ваш код подтверждения</p>
		<div class="code">%s</div>
		<p class="info">Код действителен <strong>5 минут</strong>.<br>Если вы не запрашивали код — просто проигнорируйте это письмо.</p>
		<div class="footer">Команда Zvonilka &mdash; <a href="https://zvonilka.site">zvonilka.site</a></div>
	  </div>
	</body>
	</html>
	`, code)

	params := &resend.SendEmailRequest{
		From:    "Zvonilka <hello@zvonilka.site>", // замени на свой домен
		To:      []string{req.Email},
		Subject: "Код подтверждения для Zvonilka",
		Html:    htmlContent,
	}
	_, err = client.Emails.Send(params)
	if err != nil {
		log.Printf("Ошибка отправки письма: %v", err)
		http.Error(w, "Ошибка отправки письма", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Проверка кода -> выдаёт JWT
func verifyCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Email, Code string }
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Email == "" || req.Code == "" {
		http.Error(w, "Email и код обязательны", http.StatusBadRequest)
		return
	}
	codesMutex.RLock()
	record, exists := codesStore[req.Email]
	codesMutex.RUnlock()
	if !exists {
		json.NewEncoder(w).Encode(map[string]string{"error": "Код не найден"})
		return
	}
	if time.Now().After(record.ExpiresAt) {
		codesMutex.Lock()
		delete(codesStore, req.Email)
		codesMutex.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"error": "Код истёк"})
		return
	}
	if record.Code != req.Code {
		json.NewEncoder(w).Encode(map[string]string{"error": "Неверный код"})
		return
	}
	codesMutex.Lock()
	delete(codesStore, req.Email)
	codesMutex.Unlock()

	userId := req.Email
	_, _ = db.Exec("INSERT INTO users (id, email) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING", userId, req.Email)

	token, err := generateToken(userId)
	if err != nil {
		http.Error(w, "Ошибка генерации токена", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "userId": userId, "token": token})
}

// Получение всех сообщений (защищено JWT)
func getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := db.Query("SELECT id, text, userId, displayName, createdAt, edited, editedByAdmin FROM messages ORDER BY createdAt ASC")
	if err != nil {
		log.Printf("❌ Ошибка запроса: %v", err)
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Text, &m.UserID, &m.DisplayName, &m.CreatedAt, &m.Edited, &m.EditedByAdmin); err == nil {
			messages = append(messages, m)
		}
	}
	if messages == nil {
		messages = []Message{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// Отправка нового сообщения (для polling)
func postMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		UserId      string `json:"userId"`
		Text        string `json:"text"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserId == "" || req.Text == "" {
		http.Error(w, "Нет данных", http.StatusBadRequest)
		return
	}
	id := fmt.Sprintf("%x", time.Now().UnixNano())
	displayName := req.DisplayName
	if displayName == "" {
		displayName = strings.Split(req.UserId, "@")[0]
	}
	_, err := db.Exec(
		"INSERT INTO messages (id, text, userId, displayName, createdAt) VALUES ($1, $2, $3, $4, NOW())",
		id, req.Text, req.UserId, displayName,
	)
	if err != nil {
		log.Printf("Ошибка вставки: %v", err)
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	var msg Message
	db.QueryRow("SELECT id, text, userId, displayName, createdAt, edited, editedByAdmin FROM messages WHERE id = $1", id).
		Scan(&msg.ID, &msg.Text, &msg.UserID, &msg.DisplayName, &msg.CreatedAt, &msg.Edited, &msg.EditedByAdmin)
	broadcastMessage(msg)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": msg})
}

// Редактирование сообщения
func editMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}
	msgID := pathParts[3]
	var req struct {
		Text   string `json:"text"`
		UserId string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Text == "" {
		http.Error(w, "Текст обязателен", http.StatusBadRequest)
		return
	}
	var msg Message
	err := db.QueryRow("SELECT id, text, userId, displayName, createdAt, edited, editedByAdmin FROM messages WHERE id = $1", msgID).
		Scan(&msg.ID, &msg.Text, &msg.UserID, &msg.DisplayName, &msg.CreatedAt, &msg.Edited, &msg.EditedByAdmin)
	if err != nil {
		http.Error(w, "Сообщение не найдено", http.StatusNotFound)
		return
	}
	isAdmin := req.UserId == adminEmail
	isOwner := msg.UserID == req.UserId
	if !isOwner && !isAdmin {
		http.Error(w, "Нет прав", http.StatusForbidden)
		return
	}
	_, err = db.Exec("UPDATE messages SET text = $1, edited = TRUE WHERE id = $2", req.Text, msgID)
	if err != nil {
		log.Printf("Ошибка обновления: %v", err)
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	var updated Message
	db.QueryRow("SELECT id, text, userId, displayName, createdAt, edited, editedByAdmin FROM messages WHERE id = $1", msgID).
		Scan(&updated.ID, &updated.Text, &updated.UserID, &updated.DisplayName, &updated.CreatedAt, &updated.Edited, &updated.EditedByAdmin)
	broadcastMessageUpdated(updated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": updated})
}

// --- WebSocket ---
func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Ошибка upgrade:", err)
		return
	}
	defer conn.Close()
	var userId string
	// Читаем команду set-user
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		if cmd, ok := msg["command"]; ok && cmd == "set-user" {
			if uid, ok := msg["userId"].(string); ok {
				userId = uid
				clientsMux.Lock()
				clients[conn] = userId
				clientsMux.Unlock()
				log.Printf("👤 Пользователь %s подключён", userId)
				break
			}
		}
	}
	if userId == "" {
		return
	}
	// Обработка new-message
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		if cmd, ok := msg["command"]; ok && cmd == "new-message" {
			text, _ := msg["text"].(string)
			displayName, _ := msg["displayName"].(string)
			if text == "" {
				continue
			}
			id := fmt.Sprintf("%x", time.Now().UnixNano())
			if displayName == "" {
				displayName = strings.Split(userId, "@")[0]
			}
			db.Exec(
				"INSERT INTO messages (id, text, userId, displayName, createdAt) VALUES ($1, $2, $3, $4, NOW())",
				id, text, userId, displayName,
			)
			var msgData Message
			db.QueryRow("SELECT id, text, userId, displayName, createdAt, edited, editedByAdmin FROM messages WHERE id = $1", id).
				Scan(&msgData.ID, &msgData.Text, &msgData.UserID, &msgData.DisplayName, &msgData.CreatedAt, &msgData.Edited, &msgData.EditedByAdmin)
			broadcastMessage(msgData)
		}
	}
	clientsMux.Lock()
	delete(clients, conn)
	clientsMux.Unlock()
}

// --- Рассылка через WebSocket ---
func broadcastMessage(msg Message) {
	clientsMux.RLock()
	defer clientsMux.RUnlock()
	for conn := range clients {
		if err := conn.WriteJSON(map[string]interface{}{"type": "message", "payload": msg}); err != nil {
			conn.Close()
			delete(clients, conn)
		}
	}
}
func broadcastMessageUpdated(msg Message) {
	clientsMux.RLock()
	defer clientsMux.RUnlock()
	for conn := range clients {
		if err := conn.WriteJSON(map[string]interface{}{"type": "message-updated", "payload": msg}); err != nil {
			conn.Close()
			delete(clients, conn)
		}
	}
}

// --- CORS ---
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next(w, r)
    }
}

// --- MAIN ---
func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env файл не найден, используются системные переменные")
	}
	resendKey = os.Getenv("RESEND_API_KEY")
	if resendKey == "" {
		log.Fatal("❌ RESEND_API_KEY не задан")
	}
	adminEmail = os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@example.com"
	}
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("❌ JWT_SECRET не задан")
	}
	if err := initDB(); err != nil {
		log.Fatal("❌ Ошибка инициализации БД:", err)
	}

	http.HandleFunc("/api/send-code", corsMiddleware(sendCodeHandler))
	http.HandleFunc("/api/verify-code", corsMiddleware(verifyCodeHandler))
	http.HandleFunc("/api/messages", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authMiddleware(getMessagesHandler)(w, r)
		} else if r.Method == http.MethodPost {
			postMessageHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	http.HandleFunc("/api/messages/", corsMiddleware(editMessageHandler))
	http.HandleFunc("/ws", wsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}
	log.Printf("✅ Бэкенд запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}