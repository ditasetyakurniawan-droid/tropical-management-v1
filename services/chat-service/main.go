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
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/api/chat/messages", a.handleMessages)
	mux.HandleFunc("/api/chat/stream", a.handleStream)

	log.Println(serviceName + " listening on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": serviceName})
}

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

// ============================================================
// MESSAGES
// ============================================================

func (a *app) handleMessages(w http.ResponseWriter, r *http.Request) {
	userID, userName, role, err := identity(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.getMessages(w, r)
	case http.MethodPost:
		a.postMessage(w, r, userID, userName, role)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) getMessages(w http.ResponseWriter, r *http.Request) {
	limit := parseMessageLimit(r.URL.Query().Get("limit"))

	rows, err := a.db.Query(selectMessagesQuery, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	result, err := scanChatMessages(rows, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	reverseMessages(result)
	httpx.JSON(w, http.StatusOK, result)
}

func parseMessageLimit(raw string) int {
	if raw == "" {
		return defaultMessageLimit
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 || parsed > maxMessageLimit {
		return defaultMessageLimit
	}
	return parsed
}

func scanChatMessages(rows *sql.Rows, limit int) ([]chatMessage, error) {
	result := make([]chatMessage, 0, limit)
	for rows.Next() {
		var msg chatMessage
		if err := rows.Scan(&msg.ID, &msg.UserID, &msg.UserName, &msg.Role, &msg.Body, &msg.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func reverseMessages(msgs []chatMessage) {
	for left, right := 0, len(msgs)-1; left < right; left, right = left+1, right-1 {
		msgs[left], msgs[right] = msgs[right], msgs[left]
	}
}

func (a *app) postMessage(w http.ResponseWriter, r *http.Request, userID, userName, role string) {
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

	msg, err := a.persistMessage(userID, userName, role, body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errPersistMessage)
		return
	}

	a.broker.publish(msg)
	httpx.JSON(w, http.StatusCreated, msg)
}

func (a *app) persistMessage(userID, userName, role, body string) (chatMessage, error) {
	res, err := a.db.Exec(insertMessageQuery, userID, userName, role, body)
	if err != nil {
		return chatMessage{}, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return chatMessage{}, err
	}

	return chatMessage{
		ID:        id,
		UserID:    userID,
		UserName:  userName,
		Role:      role,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// ============================================================
// STREAM
// ============================================================

func (a *app) handleStream(w http.ResponseWriter, r *http.Request) {
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

	setupSSEHeaders(w)
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	client := a.broker.subscribe()
	defer a.broker.unsubscribe(client)

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	a.runStreamLoop(w, r, flusher, client, heartbeat)
}

func setupSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

func (a *app) runStreamLoop(w http.ResponseWriter, r *http.Request, flusher http.Flusher, client chan chatMessage, heartbeat *time.Ticker) {
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
