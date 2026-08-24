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

const (
	defaultDSN  = "tropical:tropical@tcp(mysql:3306)/tropical_auth?parseTime=true&charset=utf8mb4"
	serviceName = "chat-service"
	listenAddr  = ":8080"

	// Role
	roleAdmin   = "admin"
	roleAuditor = "auditor"
	roleStaff   = "staff"

	// Error messages
	errMethodNotAllowed     = "method not allowed"
	errInvalidJSON          = "invalid json"
	errMessageRequired      = "message is required"
	errMessageTooLong       = "message exceeds 1000 characters"
	errUnauthorized         = "unauthorized"
	errTrustedHeaders       = "trusted gateway identity headers required"
	errStreamingUnsupported = "streaming unsupported"
	errPersistMessage       = "failed to persist chat message"

	// Limits
	defaultMessageLimit = 100
	maxMessageLimit     = 200

	// SQL queries
	createChatMessagesTable = `CREATE TABLE IF NOT EXISTS chat_messages (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		user_id VARCHAR(64) NOT NULL,
		user_name VARCHAR(120) NOT NULL,
		role VARCHAR(30) NOT NULL,
		body VARCHAR(1000) NOT NULL,
		created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
		INDEX idx_chat_created_at(created_at),
		INDEX idx_chat_user(user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	selectMessagesQuery = `SELECT id,user_id,user_name,role,body,created_at
		FROM chat_messages ORDER BY id DESC LIMIT ?`

	insertMessageQuery = `INSERT INTO chat_messages(user_id,user_name,role,body)
		VALUES(?,?,?,?)`
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

func newBroker() *broker {
	return &broker{clients: map[chan chatMessage]struct{}{}}
}

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
	db, err := dbx.Open(httpx.Env("CHAT_DB_DSN", defaultDSN))
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
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": serviceName})
	})
	mux.HandleFunc("/api/chat/messages", a.messages)
	mux.HandleFunc("/api/chat/stream", a.stream)

	log.Println(serviceName + " listening on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}

// writeError mengirimkan JSON error secara konsisten.
func writeError(w http.ResponseWriter, status int, msg string) {
	httpx.JSON(w, status, map[string]string{"error": msg})
}

func (a *app) migrate() error {
	_, err := a.db.Exec(createChatMessagesTable)
	return err
}

func cleanChatBody(raw string) (string, error) {
	body := strings.TrimSpace(raw)
	if body == "" {
		return "", errors.New(errMessageRequired)
	}
	if utf8.RuneCountInString(body) > 1000 {
		return "", errors.New(errMessageTooLong)
	}
	return body, nil
}

func validRole(role string) bool {
	switch role {
	case roleAdmin, roleAuditor, roleStaff:
		return true
	default:
		return false
	}
}

func identity(r *http.Request) (userID, userName, role string, err error) {
	userID = strings.TrimSpace(r.Header.Get("X-User-ID"))
	userName = strings.TrimSpace(r.Header.Get("X-User-Name"))
	role = strings.ToLower(strings.TrimSpace(r.Header.Get("X-User-Role")))
	if userID == "" || userName == "" || !validRole(role) {
		return "", "", "", errors.New(errTrustedHeaders)
	}
	return userID, userName, role, nil
}

func (a *app) messages(w http.ResponseWriter, r *http.Request) {
	// Ambil identitas sekali untuk semua method
	userID, userName, role, err := identity(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	switch r.Method {
	case http.MethodGet:
		limit := defaultMessageLimit
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= maxMessageLimit {
				limit = parsed
			}
		}

		rows, err := a.db.Query(selectMessagesQuery, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()

		result := make([]chatMessage, 0, limit)
		for rows.Next() {
			var msg chatMessage
			if err := rows.Scan(&msg.ID, &msg.UserID, &msg.UserName, &msg.Role, &msg.Body, &msg.CreatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			result = append(result, msg)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Balik urutan agar kronologis (yang lama dulu)
		for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
			result[left], result[right] = result[right], result[left]
		}

		httpx.JSON(w, http.StatusOK, result)

	case http.MethodPost:
		var input struct {
			Body string `json:"body"`
		}
		if err := httpx.DecodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, errInvalidJSON)
			return
		}

		body, err := cleanChatBody(input.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		res, err := a.db.Exec(insertMessageQuery, userID, userName, role, body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, errPersistMessage)
			return
		}

		id, err := res.LastInsertId()
		if err != nil {
			writeError(w, http.StatusInternalServerError, errPersistMessage)
			return
		}

		msg := chatMessage{
			ID:        id,
			UserID:    userID,
			UserName:  userName,
			Role:      role,
			Body:      body,
			CreatedAt: time.Now().UTC(),
		}

		a.broker.publish(msg)
		httpx.JSON(w, http.StatusCreated, msg)

	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) stream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	if _, _, _, err := identity(r); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errStreamingUnsupported)
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
		case msg := <-client:
			payload, err := json.Marshal(msg)
			if err != nil {
				log.Printf("stream: failed to marshal message: %v", err)
				continue
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
}
