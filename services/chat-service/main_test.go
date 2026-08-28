package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthzHandler(t *testing.T) {
	w := httptest.NewRecorder()
	healthzHandler(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), serviceName) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestCleanChatBody(t *testing.T) {
	body, err := cleanChatBody("  halo tim restoran  ")
	if err != nil || body != "halo tim restoran" {
		t.Fatalf("unexpected clean result: body=%q err=%v", body, err)
	}
	if _, err := cleanChatBody("   "); err == nil {
		t.Fatal("expected empty chat message to fail")
	}
	if _, err := cleanChatBody(strings.Repeat("界", 1001)); err == nil {
		t.Fatal("expected >1000 runes to fail")
	}
	if _, err := cleanChatBody(strings.Repeat("界", 1000)); err != nil {
		t.Fatalf("1000 runes should be valid: %v", err)
	}
}

func TestChatRoleAndIdentity(t *testing.T) {
	for _, role := range []string{roleAdmin, roleAuditor, roleStaff} {
		if !validRole(role) {
			t.Fatalf("role %q should be valid", role)
		}
	}
	if validRole("owner") {
		t.Fatal("owner should be invalid")
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-User-ID", " 42 ")
	r.Header.Set("X-User-Name", " Alice ")
	r.Header.Set("X-User-Role", " STAFF ")
	id, name, role, err := identity(r)
	if err != nil || id != "42" || name != "Alice" || role != roleStaff {
		t.Fatalf("identity=(%q,%q,%q,%v)", id, name, role, err)
	}

	r.Header.Del("X-User-ID")
	if _, _, _, err := identity(r); err == nil {
		t.Fatal("missing trusted identity should fail")
	}
}

func TestParseMessageLimit(t *testing.T) {
	cases := map[string]int{
		"":    defaultMessageLimit,
		"1":   1,
		"200": maxMessageLimit,
		"201": defaultMessageLimit,
		"0":   defaultMessageLimit,
		"bad": defaultMessageLimit,
	}
	for raw, want := range cases {
		if got := parseMessageLimit(raw); got != want {
			t.Fatalf("parseMessageLimit(%q)=%d want %d", raw, got, want)
		}
	}
}

func TestReverseMessages(t *testing.T) {
	msgs := []chatMessage{{ID: 1}, {ID: 2}, {ID: 3}}
	reverseMessages(msgs)
	if msgs[0].ID != 3 || msgs[1].ID != 2 || msgs[2].ID != 1 {
		t.Fatalf("unexpected order: %+v", msgs)
	}
}

func TestBrokerPublishSubscribe(t *testing.T) {
	b := newBroker()
	ch := b.subscribe()
	msg := chatMessage{ID: 9, Body: "hello"}
	b.publish(msg)
	select {
	case got := <-ch:
		if got.ID != msg.ID || got.Body != msg.Body {
			t.Fatalf("message=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broker message")
	}
	b.unsubscribe(ch)
	if len(b.clients) != 0 {
		t.Fatalf("clients=%d", len(b.clients))
	}
}

func TestSetupSSEHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	setupSSEHeaders(w)
	if w.Header().Get("Content-Type") != "text/event-stream" || w.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("headers=%v", w.Header())
	}
}

func TestChatHandlersRejectInvalidRequestsBeforeDatabaseAccess(t *testing.T) {
	a := &app{broker: newBroker()}

	w := httptest.NewRecorder()
	a.handleMessages(w, httptest.NewRequest(http.MethodGet, "/api/chat/messages", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized messages status=%d", w.Code)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/chat/messages", nil)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Name", "A")
	req.Header.Set("X-User-Role", roleStaff)
	w = httptest.NewRecorder()
	a.handleMessages(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("delete messages status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.postMessage(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"body":"   "}`)), "1", "A", roleStaff)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty post status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.handleStream(w, httptest.NewRequest(http.MethodPost, "/api/chat/stream", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("stream post status=%d", w.Code)
	}
}

func TestChatDatabasePaths(t *testing.T) {
	now := time.Date(2026, 8, 29, 11, 0, 0, 0, time.UTC)
	db, script := openTestDB(t,
		execStep("CREATE TABLE IF NOT EXISTS chat_messages", 0, 0),
		queryStep("FROM chat_messages ORDER BY id DESC", []string{"id", "user_id", "user_name", "role", "body", "created_at"},
			row(int64(2), "2", "Bob", roleAuditor, "second", now),
			row(int64(1), "1", "Alice", roleStaff, "first", now.Add(-time.Minute))),
		execStep("INSERT INTO chat_messages", 3, 1),
	)
	a := &app{db: db, broker: newBroker()}

	if err := a.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/chat/messages?limit=2", nil)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Name", "Alice")
	req.Header.Set("X-User-Role", roleStaff)
	w := httptest.NewRecorder()
	a.handleMessages(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"body":"first"`) {
		t.Fatalf("messages status=%d body=%q", w.Code, w.Body.String())
	}
	if strings.Index(w.Body.String(), `"first"`) > strings.Index(w.Body.String(), `"second"`) {
		t.Fatalf("messages were not returned oldest-first: %q", w.Body.String())
	}

	client := a.broker.subscribe()
	defer a.broker.unsubscribe(client)
	post := httptest.NewRequest(http.MethodPost, "/api/chat/messages", strings.NewReader(`{"body":" hello team "}`))
	post.Header.Set("X-User-ID", "1")
	post.Header.Set("X-User-Name", "Alice")
	post.Header.Set("X-User-Role", roleStaff)
	w = httptest.NewRecorder()
	a.handleMessages(w, post)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":3`) || !strings.Contains(w.Body.String(), `"body":"hello team"`) {
		t.Fatalf("post status=%d body=%q", w.Code, w.Body.String())
	}
	select {
	case msg := <-client:
		if msg.ID != 3 || msg.Body != "hello team" {
			t.Fatalf("published message=%+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("message was not published to broker")
	}
	script.assertDone(t)
}

func TestChatDatabaseErrorsAreHandled(t *testing.T) {
	boom := errors.New("db unavailable")
	db, script := openTestDB(t,
		queryErrorStep("FROM chat_messages ORDER BY id DESC", boom),
		execErrorStep("INSERT INTO chat_messages", boom),
	)
	a := &app{db: db, broker: newBroker()}

	req := httptest.NewRequest(http.MethodGet, "/api/chat/messages", nil)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Name", "Alice")
	req.Header.Set("X-User-Role", roleStaff)
	w := httptest.NewRecorder()
	a.handleMessages(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("query error status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.postMessage(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"body":"hello"}`)), "1", "Alice", roleStaff)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("insert error status=%d body=%q", w.Code, w.Body.String())
	}
	script.assertDone(t)
}

func TestHandleStreamConnectsAndStopsOnCanceledContext(t *testing.T) {
	a := &app{broker: newBroker()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/chat/stream", nil).WithContext(ctx)
	req.Header.Set("X-User-ID", "1")
	req.Header.Set("X-User-Name", "Alice")
	req.Header.Set("X-User-Role", roleStaff)
	w := httptest.NewRecorder()
	a.handleStream(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), ": connected") {
		t.Fatalf("stream status=%d body=%q", w.Code, w.Body.String())
	}
	if len(a.broker.clients) != 0 {
		t.Fatalf("stream client leak: %d", len(a.broker.clients))
	}
}

type signalingStreamRecorder struct {
	*httptest.ResponseRecorder
	flushed chan struct{}
}

func newSignalingStreamRecorder() *signalingStreamRecorder {
	return &signalingStreamRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		flushed:          make(chan struct{}, 1),
	}
}

func (w *signalingStreamRecorder) Flush() {
	w.ResponseRecorder.Flush()
	select {
	case w.flushed <- struct{}{}:
	default:
	}
}

func TestRunStreamLoopMessageAndHeartbeat(t *testing.T) {
	t.Run("writes broker message", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, "/api/chat/stream", nil).WithContext(ctx)
		client := make(chan chatMessage, 1)
		client <- chatMessage{ID: 11, UserID: "7", UserName: "Alice", Role: roleStaff, Body: "hello"}
		heartbeat := time.NewTicker(time.Hour)
		defer heartbeat.Stop()
		w := newSignalingStreamRecorder()
		done := make(chan struct{})

		go func() {
			(&app{}).runStreamLoop(w, req, w, client, heartbeat)
			close(done)
		}()

		select {
		case <-w.flushed:
			cancel()
		case <-time.After(time.Second):
			t.Fatal("stream did not flush broker message")
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("stream did not stop after cancellation")
		}
		if body := w.Body.String(); !strings.Contains(body, "event: message") || !strings.Contains(body, `"body":"hello"`) {
			t.Fatalf("unexpected stream body %q", body)
		}
	})

	t.Run("writes heartbeat", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := httptest.NewRequest(http.MethodGet, "/api/chat/stream", nil).WithContext(ctx)
		client := make(chan chatMessage)
		heartbeat := time.NewTicker(time.Millisecond)
		defer heartbeat.Stop()
		w := newSignalingStreamRecorder()
		done := make(chan struct{})

		go func() {
			(&app{}).runStreamLoop(w, req, w, client, heartbeat)
			close(done)
		}()

		select {
		case <-w.flushed:
			cancel()
		case <-time.After(time.Second):
			t.Fatal("stream did not flush heartbeat")
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("stream did not stop after cancellation")
		}
		if body := w.Body.String(); !strings.Contains(body, ": heartbeat") {
			t.Fatalf("unexpected heartbeat body %q", body)
		}
	})
}
