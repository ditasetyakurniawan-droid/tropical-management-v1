package main

import (
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
