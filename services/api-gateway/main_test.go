package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/trafficx"
	"github.com/golang-jwt/jwt/v5"
)

func TestHealthzHandler(t *testing.T) {
	w := httptest.NewRecorder()
	httpx.HealthHandler(serviceName)(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), serviceName) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func signedToken(t *testing.T, secret []byte, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func validClaims(role string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"sub":   42,
		"name":  "Test User",
		"email": "test@example.com",
		"role":  role,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
}

func TestPathMatchesPrefix(t *testing.T) {
	for _, tc := range []struct {
		path, prefix string
		want         bool
	}{
		{"/api/sales", "/api/sales", true},
		{"/api/sales/1", "/api/sales", true},
		{"/api/sales-report", "/api/sales", false},
		{"/api/sale", "/api/sales", false},
	} {
		if got := pathMatchesPrefix(tc.path, tc.prefix); got != tc.want {
			t.Fatalf("pathMatchesPrefix(%q,%q)=%v want %v", tc.path, tc.prefix, got, tc.want)
		}
	}
}

func TestAllowedMatrix(t *testing.T) {
	tests := []struct {
		role, method, path string
		want               bool
	}{
		{"admin", http.MethodDelete, "/api/users", true},
		{"auditor", http.MethodGet, "/api/sales", true},
		{"auditor", http.MethodPatch, "/api/issues/1", true},
		{"auditor", http.MethodPost, "/api/sales", false},
		{"staff", http.MethodPost, "/api/sales", true},
		{"staff", http.MethodPost, "/api/sales-report", false},
		{"staff", http.MethodPost, "/api/chat/messages", true},
		{"staff", http.MethodDelete, "/api/chat/messages", false},
		{"unknown", http.MethodGet, "/api/sales", false},
	}
	for _, tc := range tests {
		if got := allowed(tc.role, tc.method, tc.path); got != tc.want {
			t.Fatalf("allowed(%q,%q,%q)=%v want %v", tc.role, tc.method, tc.path, got, tc.want)
		}
	}
}

func TestGatewayRequiresAuthentication(t *testing.T) {
	g := &gateway{secret: []byte("01234567890123456789012345678901")}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/sales", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestGatewayPublicLoginAndAuthorizedProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"path": r.URL.Path,
			"id":   r.Header.Get("X-User-ID"),
			"name": r.Header.Get("X-User-Name"),
			"role": r.Header.Get("X-User-Role"),
		})
	}))
	defer upstream.Close()

	secret := []byte("01234567890123456789012345678901")
	g := &gateway{
		secret: secret,
		routes: []route{{prefix: "/api/auth", proxy: proxy(upstream.URL)}, {prefix: "/api/sales", proxy: proxy(upstream.URL)}},
	}

	t.Run("login is public", func(t *testing.T) {
		w := httptest.NewRecorder()
		g.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{}`)))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"path":"/api/auth/login"`) {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("verified identity replaces untrusted headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/sales", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+signedToken(t, secret, validClaims("staff")))
		req.Header.Set("X-User-ID", "attacker")
		req.Header.Set("X-User-Name", "attacker")
		req.Header.Set("X-User-Role", "admin")
		w := httptest.NewRecorder()
		g.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		body := w.Body.String()
		for _, want := range []string{`"id":"42"`, `"name":"Test User"`, `"role":"staff"`} {
			if !strings.Contains(body, want) {
				t.Fatalf("body=%q missing %q", body, want)
			}
		}
	})
}

func TestGatewayCORSPreflight(t *testing.T) {
	g := &gateway{}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest(http.MethodOptions, "/api/sales", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "X-Request-ID") {
		t.Fatalf("allow headers=%q", w.Header().Get("Access-Control-Allow-Headers"))
	}
	if w.Header().Get("Access-Control-Expose-Headers") != "X-Request-ID" {
		t.Fatalf("expose headers=%q", w.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestGatewayRejectsTokenWithoutExpiration(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	claims := validClaims("admin")
	delete(claims, "exp")
	req := httptest.NewRequest(http.MethodGet, "/api/sales", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken(t, secret, claims))
	g := &gateway{secret: secret}
	if _, err := g.claims(req); err == nil {
		t.Fatal("token without expiration should be rejected")
	}
}

func TestGatewayReturns404ForPrefixLookalike(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	defer upstream.Close()
	g := &gateway{secret: secret, routes: []route{{prefix: "/api/sales", proxy: proxy(upstream.URL)}}}
	req := httptest.NewRequest(http.MethodGet, "/api/sales-report", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken(t, secret, validClaims("staff")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestGatewayRejectsForbiddenMutation(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	g := &gateway{secret: secret}
	req := httptest.NewRequest(http.MethodPost, "/api/sales", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+signedToken(t, secret, validClaims("auditor")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestGatewayUsesEmailWhenNameIsBlank(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"name": r.Header.Get("X-User-Name")})
	}))
	defer upstream.Close()

	secret := []byte("01234567890123456789012345678901")
	claims := validClaims("admin")
	claims["name"] = "   "
	claims["email"] = "fallback@example.com"
	g := &gateway{secret: secret, routes: []route{{prefix: "/api/sales", proxy: proxy(upstream.URL)}}}
	req := httptest.NewRequest(http.MethodGet, "/api/sales", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken(t, secret, claims))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"name":"fallback@example.com"`) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestGatewayAuthorizedUnknownRouteReturns404(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	g := &gateway{secret: secret}
	req := httptest.NewRequest(http.MethodGet, "/api/not-configured", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken(t, secret, validClaims("admin")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestGatewayRejectsNonHS256Token(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	tok := jwt.NewWithClaims(jwt.SigningMethodHS384, validClaims("admin"))
	signed, err := tok.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/sales", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	g := &gateway{secret: secret}
	if _, err := g.claims(req); err == nil {
		t.Fatal("non-HS256 token should be rejected")
	}
}

func TestProxyErrorHandlerMasksUpstreamFailure(t *testing.T) {
	p := proxy("http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodGet, "/api/sales", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "upstream service unavailable") {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestProxyPanicsForInvalidURL(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("proxy should panic for an invalid URL")
		}
	}()
	_ = proxy("http://[::1")
}

func TestGatewayRejectsWhenConcurrencyLimitIsSaturated(t *testing.T) {
	limiter := trafficx.NewConcurrencyLimiter(1)
	if !limiter.TryAcquire() {
		t.Fatal("failed to occupy gateway slot")
	}
	defer limiter.Release()

	g := &gateway{inFlight: limiter}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{}`)))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After=%q", got)
	}
}

func TestGatewayDoesNotCountSSEAgainstNormalConcurrencyLimit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	limiter := trafficx.NewConcurrencyLimiter(1)
	if !limiter.TryAcquire() {
		t.Fatal("failed to occupy gateway slot")
	}
	defer limiter.Release()

	secret := []byte("01234567890123456789012345678901")
	g := &gateway{
		secret:         secret,
		inFlight:       limiter,
		streamInFlight: trafficx.NewConcurrencyLimiter(1),
		routes:         []route{{prefix: "/api/chat", proxy: proxy(upstream.URL)}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/chat/stream", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken(t, secret, validClaims("staff")))
	w := httptest.NewRecorder()
	g.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestGatewayRejectsWhenSSEConcurrencyLimitIsSaturated(t *testing.T) {
	streamLimiter := trafficx.NewConcurrencyLimiter(1)
	if !streamLimiter.TryAcquire() {
		t.Fatal("failed to occupy SSE gateway slot")
	}
	defer streamLimiter.Release()

	g := &gateway{streamInFlight: streamLimiter}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/chat/stream", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}
