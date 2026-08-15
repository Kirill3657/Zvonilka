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
	"github.com/rs/cors"
)

type Message struct {
	ID          string    `json:"id"`
	Text        string    `json:"text"`
	UserID      string    `json:"userId"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

type CodeRecord struct {
	Code      string
	ExpiresAt time.Time
}

var (
	db          *sql.DB
	codesStore  = make(map[string]CodeRecord)
	codesMutex  sync.RWMutex
	upgrader    = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clients     = make(map[*websocket.Conn]string)
	clientsMux  sync.RWMutex
	jwtSecret   []byte
)

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
		userId, _ := claims["userId"].(string)
		ctx := context.WithValue(r.Context(), "userId", userId)
		next(w, r.WithContext(ctx))
	}
}

func generateCode() string {
	return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
}

func initDB() error {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		return fmt.Errorf("DATABASE_URL не задан")
	}
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	if err = db.Ping(); err != nil {
		return err
	}
	log.Println("✅ Подключение к БД успешно")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL
		);
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			text TEXT NOT NULL,
			userId TEXT NOT NULL,
			displayName TEXT NOT NULL,
			createdAt TIMESTAMP DEFAULT NOW()
		);
	`)
	if err != nil {
		return err
	}
	log.Println("✅ Таблицы созданы")
	return nil
}

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

	client := resend.NewClient(os.Getenv("RESEND_API_KEY"))
	params := &resend.SendEmailRequest{
		From:    "Zvonilka Team <hello@mail.zvonilka.site>",
		To:      []string{req.Email},
		Subject: "Код подтверждения",
		Html:    fmt.Sprintf("<h2>Ваш код: %s</h2><p>Действует 5 минут.</p>", code),
	}
	_, err = client.Emails.Send(params)
	if err != nil {
		log.Printf("Ошибка отправки письма: %v", err)
		http.Error(w, "Ошибка отправки письма", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"userId":  userId,
		"token":   token,
	})
}

func getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, text, userId, displayName, createdAt FROM messages ORDER BY createdAt ASC")
	if err != nil {
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		var m Message
		rows.Scan(&m.ID, &m.Text, &m.UserID, &m.DisplayName, &m.CreatedAt)
		messages = append(messages, m)
	}
	if messages == nil {
		messages = []Message{}
	}
	json.NewEncoder(w).Encode(messages)
}

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
		http.Error(w, "Ошибка БД", http.StatusInternalServerError)
		return
	}
	var msg Message
	db.QueryRow("SELECT id, text, userId, displayName, createdAt FROM messages WHERE id = $1", id).
		Scan(&msg.ID, &msg.Text, &msg.UserID, &msg.DisplayName, &msg.CreatedAt)
	broadcastMessage(msg)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": msg})
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
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
			db.QueryRow("SELECT id, text, userId, displayName, createdAt FROM messages WHERE id = $1", id).
				Scan(&msgData.ID, &msgData.Text, &msgData.UserID, &msgData.DisplayName, &msgData.CreatedAt)
			broadcastMessage(msgData)
		}
	}
	clientsMux.Lock()
	delete(clients, conn)
	clientsMux.Unlock()
}

func broadcastMessage(msg Message) {
	clientsMux.RLock()
	defer clientsMux.RUnlock()
	for conn := range clients {
		conn.WriteJSON(map[string]interface{}{"type": "message", "payload": msg})
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env файл не найден")
	}
	if os.Getenv("RESEND_API_KEY") == "" {
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

	mux.HandleFunc("/api/send-code", sendCodeHandler)
	mux.HandleFunc("/api/verify-code", verifyCodeHandler)
	mux.HandleFunc("/api/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authMiddleware(getMessagesHandler)(w, r)
		} else if r.Method == http.MethodPost {
			postMessageHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/ws", wsHandler)

	// Настраиваем CORS через rs/cors
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: true,
	})
	handler := c.Handler(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}
	log.Printf("✅ Бэкенд запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}