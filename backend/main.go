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

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	IsAdmin   bool   `json:"isAdmin"`
	Theme     string `json:"theme"`
	Lang      string `json:"lang"`
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
	jwtSecret   []byte
)

// --- JWT ---
func generateToken(userId string, isAdmin bool) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId":  userId,
		"isAdmin": isAdmin,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
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
		userId, _ := claims["userId"].(string)
		isAdmin, _ := claims["isAdmin"].(bool)
		ctx := context.WithValue(r.Context(), "userId", userId)
		ctx = context.WithValue(ctx, "isAdmin", isAdmin)
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

	// Создаём таблицу users (если её нет)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("ошибка создания таблицы users: %w", err)
	}

	// Добавляем новые колонки (если их ещё нет)
	columns := map[string]string{
		"first_name": "TEXT",
		"last_name":  "TEXT",
		"is_admin":   "BOOLEAN DEFAULT FALSE",
		"theme":      "TEXT DEFAULT 'light'",
		"lang":       "TEXT DEFAULT 'ru'",
	}
	for col, def := range columns {
		_, err = db.Exec(fmt.Sprintf("ALTER TABLE users ADD COLUMN IF NOT EXISTS %s %s;", col, def))
		if err != nil {
			log.Printf("⚠️ Не удалось добавить колонку %s: %v", col, err)
		}
	}
	// Создаём таблицу messages
	_, err = db.Exec(`
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
		return fmt.Errorf("ошибка создания таблицы messages: %w", err)
	}
	log.Println("✅ Таблицы и колонки созданы (или уже существуют)")
	return nil
}

// --- CORS Middleware (с логированием) ---
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🌐 CORS: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

// --- Хендлеры ---

// 1. Отправка кода
func sendCodeHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
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

	client := resend.NewClient(os.Getenv("RESEND_API_KEY"))
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
		.code-box { background: #e8f0fe; padding: 20px; border-radius: 12px; margin: 20px 0; }
		.code { font-size: 52px; font-weight: 700; letter-spacing: 8px; color: #1a73e8; font-family: 'Courier New', monospace; }
		.info { font-size: 14px; color: #666; margin: 20px 0; }
		.copy-btn { background: #1a73e8; color: #fff; border: none; padding: 12px 24px; border-radius: 8px; font-size: 16px; cursor: pointer; text-decoration: none; display: inline-block; }
		.footer { font-size: 13px; color: #aaa; border-top: 1px solid #eee; padding-top: 20px; margin-top: 20px; }
		.footer a { color: #1a73e8; text-decoration: none; }
	</style>
	</head>
	<body>
	  <div class="container">
		<div class="logo">📱</div>
		<h1>Zvonilka</h1>
		<p class="sub">Ваш код подтверждения</p>
		<div class="code-box">
		  <div class="code" id="code">%s</div>
		</div>
		<button class="copy-btn" onclick="navigator.clipboard.writeText(document.getElementById('code').innerText)">Скопировать код</button>
		<p class="info">Код действителен <strong>5 минут</strong>.</p>
		<div class="footer">Команда Zvonilka &mdash; <a href="https://zvonilka.site">zvonilka.site</a></div>
	  </div>
	</body>
	</html>
	`, code)

	params := &resend.SendEmailRequest{
		From:    "Zvonilka <hello@zvonilka.site>",
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

// 2. Проверка кода
func verifyCodeHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
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
	var isAdmin bool
	err = db.QueryRow("SELECT is_admin FROM users WHERE id = $1", userId).Scan(&isAdmin)
	if err == sql.ErrNoRows {
		_, err = db.Exec("INSERT INTO users (id, email, is_admin) VALUES ($1, $2, $3)", userId, req.Email, false)
		if err != nil {
			http.Error(w, "Ошибка создания пользователя", http.StatusInternalServerError)
			return
		}
		isAdmin = false
	} else if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}

	token, err := generateToken(userId, isAdmin)
	if err != nil {
		http.Error(w, "Ошибка генерации токена", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"userId":  userId,
		"token":   token,
		"isAdmin": isAdmin,
	})
}

// 3. Получение профиля
func getUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
	userId, ok := r.Context().Value("userId").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var user User
	err := db.QueryRow("SELECT id, email, first_name, last_name, is_admin, theme, lang FROM users WHERE id = $1", userId).
		Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.IsAdmin, &user.Theme, &user.Lang)
	if err != nil {
		http.Error(w, "Пользователь не найден", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// 4. Обновление профиля
func updateProfileHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userId, ok := r.Context().Value("userId").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Theme     string `json:"theme"`
		Lang      string `json:"lang"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Неверный запрос", http.StatusBadRequest)
		return
	}
	_, err = db.Exec(`
		UPDATE users SET first_name = $1, last_name = $2, theme = $3, lang = $4 WHERE id = $5
	`, req.FirstName, req.LastName, req.Theme, req.Lang, userId)
	if err != nil {
		http.Error(w, "Ошибка обновления", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// 5. Получение пользователей (админ)
func getUsersHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
	isAdmin, ok := r.Context().Value("isAdmin").(bool)
	if !ok || !isAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	rows, err := db.Query("SELECT id, email, first_name, last_name, is_admin, theme, lang FROM users ORDER BY email")
	if err != nil {
		log.Printf("❌ Ошибка запроса users: %v", err)
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.IsAdmin, &u.Theme, &u.Lang)
		users = append(users, u)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// 6. Назначение/снятие админа
func setAdminHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
	isAdmin, ok := r.Context().Value("isAdmin").(bool)
	if !ok || !isAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Error(w, "Неверный путь", http.StatusBadRequest)
		return
	}
	targetUserId := pathParts[4]
	var req struct{ IsAdmin bool }
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Неверный запрос", http.StatusBadRequest)
		return
	}
	_, err = db.Exec("UPDATE users SET is_admin = $1 WHERE id = $2", req.IsAdmin, targetUserId)
	if err != nil {
		log.Printf("❌ Ошибка обновления is_admin: %v", err)
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// 7. Получение сообщений
func getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
	rows, err := db.Query("SELECT id, text, userId, displayName, createdAt, edited, editedByAdmin FROM messages ORDER BY createdAt ASC")
	if err != nil {
		log.Printf("❌ Ошибка запроса messages: %v", err)
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		var m Message
		rows.Scan(&m.ID, &m.Text, &m.UserID, &m.DisplayName, &m.CreatedAt, &m.Edited, &m.EditedByAdmin)
		messages = append(messages, m)
	}
	if messages == nil {
		messages = []Message{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// 8. Отправка сообщения (HTTP)
func postMessageHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		UserId      string `json:"userId"`
		Text        string `json:"text"`
		DisplayName string `json:"displayName"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.UserId == "" || req.Text == "" {
		http.Error(w, "Нет данных", http.StatusBadRequest)
		return
	}
	id := fmt.Sprintf("%x", time.Now().UnixNano())
	displayName := req.DisplayName
	if displayName == "" {
		displayName = strings.Split(req.UserId, "@")[0]
	}
	_, err = db.Exec(
		"INSERT INTO messages (id, text, userId, displayName, createdAt) VALUES ($1, $2, $3, $4, NOW())",
		id, req.Text, req.UserId, displayName,
	)
	if err != nil {
		log.Printf("❌ Ошибка вставки сообщения: %v", err)
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

// 9. Редактирование сообщения
func editMessageHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
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
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Text == "" {
		http.Error(w, "Текст обязателен", http.StatusBadRequest)
		return
	}
	var msg Message
	err = db.QueryRow("SELECT id, text, userId, displayName, createdAt, edited, editedByAdmin FROM messages WHERE id = $1", msgID).
		Scan(&msg.ID, &msg.Text, &msg.UserID, &msg.DisplayName, &msg.CreatedAt, &msg.Edited, &msg.EditedByAdmin)
	if err != nil {
		http.Error(w, "Сообщение не найдено", http.StatusNotFound)
		return
	}
	isAdmin, _ := r.Context().Value("isAdmin").(bool)
	isOwner := msg.UserID == req.UserId
	if !isOwner && !isAdmin {
		http.Error(w, "Нет прав", http.StatusForbidden)
		return
	}
	_, err = db.Exec("UPDATE messages SET text = $1, edited = $2 WHERE id = $3", req.Text, !isAdmin, msgID)
	if err != nil {
		log.Printf("❌ Ошибка обновления сообщения: %v", err)
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

// 10. WebSocket
func wsHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("📨 %s %s", r.Method, r.URL.Path)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Ошибка upgrade:", err)
		return
	}
	defer conn.Close()
	var userId string
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

// --- Broadcast ---
func broadcastMessage(msg Message) {
	clientsMux.RLock()
	defer clientsMux.RUnlock()
	for conn := range clients {
		conn.WriteJSON(map[string]interface{}{"type": "message", "payload": msg})
	}
}

func broadcastMessageUpdated(msg Message) {
	clientsMux.RLock()
	defer clientsMux.RUnlock()
	for conn := range clients {
		conn.WriteJSON(map[string]interface{}{"type": "message-updated", "payload": msg})
	}
}

// --- MAIN ---
func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env файл не найден")
	}
	resendKey = os.Getenv("RESEND_API_KEY")
	if resendKey == "" {
		log.Fatal("❌ RESEND_API_KEY не задан")
	}
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal("❌ JWT_SECRET не задан")
	}
	if err := initDB(); err != nil {
		log.Fatal("❌ Ошибка инициализации БД:", err)
	}

	mux := http.NewServeMux()

	// Регистрируем маршруты с CORS middleware
	mux.HandleFunc("/api/send-code", corsMiddleware(sendCodeHandler))
	mux.HandleFunc("/api/verify-code", corsMiddleware(verifyCodeHandler))

	mux.HandleFunc("/api/profile", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authMiddleware(getUserProfileHandler)(w, r)
		} else if r.Method == http.MethodPut {
			authMiddleware(updateProfileHandler)(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/api/users", corsMiddleware(authMiddleware(getUsersHandler)))
	mux.HandleFunc("/api/users/", corsMiddleware(authMiddleware(setAdminHandler)))

	mux.HandleFunc("/api/messages", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authMiddleware(getMessagesHandler)(w, r)
		} else if r.Method == http.MethodPost {
			postMessageHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/api/messages/", corsMiddleware(authMiddleware(editMessageHandler)))

	mux.HandleFunc("/ws", corsMiddleware(wsHandler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}
	log.Printf("✅ Бэкенд запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}