package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/dbx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
)

type chatMessage struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Role      string    `json:"role"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type broker struct {
	mu      sync.RWMutex
	clients map[chan chatMessage]struct{}
}

func newBroker() *broker { return &broker{clients: map[chan chatMessage]struct{}{}} }
func (b *broker) subscribe() chan chatMessage {
	ch := make(chan chatMessage, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}
func (b *broker) unsubscribe(ch chan chatMessage) {
	b.mu.Lock()
	delete(b.clients, ch)
	close(ch)
	b.mu.Unlock()
}
func (b *broker) publish(message chatMessage) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- message:
		default:
			// A slow browser must never block the whole chat room.
		}
	}
}

type app struct {
	db     *sql.DB
	broker *broker
}

func main() {
	dsn := httpx.Env("CHAT_DB_DSN", "tropical:tropical@tcp(mysql:3306)/tropical_auth?parseTime=true&charset=utf8mb4")
	db, err := dbx.Open(dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	a := &app{db: db, broker: newBroker()}
	if err := a.migrate(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "chat-service"})
	})
	mux.HandleFunc("/api/chat/messages", a.messages)
	mux.HandleFunc("/api/chat/stream", a.stream)

	log.Println("chat-service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (a *app) migrate() error {
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		user_id VARCHAR(64) NOT NULL,
		user_name VARCHAR(120) NOT NULL,
		role VARCHAR(30) NOT NULL,
		body VARCHAR(1000) NOT NULL,
		created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		INDEX idx_chat_created_at(created_at),
		INDEX idx_chat_user(user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
}

func cleanChatBody(raw string) (string, error) {
	body := strings.TrimSpace(raw)
	if body == "" {
		return "", errors.New("message is required")
	}
	if utf8.RuneCountInString(body) > 1000 {
		return "", errors.New("message exceeds 1000 characters")
	}
	return body, nil
}

func identity(r *http.Request) (userID, userName, role string, err error) {
	userID = strings.TrimSpace(r.Header.Get("X-User-ID"))
	userName = strings.TrimSpace(r.Header.Get("X-User-Name"))
	role = strings.ToLower(strings.TrimSpace(r.Header.Get("X-User-Role")))
	if userID == "" || userName == "" || (role != "admin" && role != "auditor" && role != "staff") {
		return "", "", "", errors.New("trusted gateway identity headers required")
	}
	return userID, userName, role, nil
}

func (a *app) messages(w http.ResponseWriter, r *http.Request) {
	if _, _, _, err := identity(r); err != nil {
		httpx.JSON(w, 401, map[string]string{"error": err.Error()})
		return
	}

	switch r.Method {
	case http.MethodGet:
		limit := 100
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
				limit = parsed
			}
		}
		rows, err := a.db.Query("SELECT id,user_id,user_name,role,body,created_at FROM chat_messages ORDER BY id DESC LIMIT ?", limit)
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		result := make([]chatMessage, 0, limit)
		for rows.Next() {
			var message chatMessage
			if err := rows.Scan(&message.ID, &message.UserID, &message.UserName, &message.Role, &message.Body, &message.CreatedAt); err == nil {
				result = append(result, message)
			}
		}
		for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
			result[left], result[right] = result[right], result[left]
		}
		httpx.JSON(w, 200, result)

	case http.MethodPost:
		userID, userName, role, err := identity(r)
		if err != nil {
			httpx.JSON(w, 401, map[string]string{"error": err.Error()})
			return
		}
		var input struct {
			Body string `json:"body"`
		}
		if err := httpx.DecodeJSON(r, &input); err != nil {
			httpx.JSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		body, err := cleanChatBody(input.Body)
		if err != nil {
			httpx.JSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		result, err := a.db.Exec("INSERT INTO chat_messages(user_id,user_name,role,body) VALUES(?,?,?,?)", userID, userName, role, body)
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": "failed to persist chat message"})
			return
		}
		id, _ := result.LastInsertId()
		message := chatMessage{ID: id, UserID: userID, UserName: userName, Role: role, Body: body, CreatedAt: time.Now().UTC()}
		a.broker.publish(message)
		httpx.JSON(w, 201, message)

	default:
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (a *app) stream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if _, _, _, err := identity(r); err != nil {
		httpx.JSON(w, 401, map[string]string{"error": err.Error()})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.JSON(w, 500, map[string]string{"error": "streaming unsupported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	client := a.broker.subscribe()
	defer a.broker.unsubscribe(client)
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case message := <-client:
			payload, err := json.Marshal(message)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
