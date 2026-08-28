package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func authToken(t *testing.T, secret []byte, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func authClaims(role string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"sub":   7,
		"name":  "Admin User",
		"email": "admin@example.com",
		"role":  role,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
}

func TestHealthzHandler(t *testing.T) {
	w := httptest.NewRecorder()
	healthzHandler(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), serviceName) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestValidRole(t *testing.T) {
	for _, role := range []string{roleAdmin, roleAuditor, roleStaff} {
		if !validRole(role) {
			t.Fatalf("expected role %q to be valid", role)
		}
	}
	for _, role := range []string{"", "manager", "ADMIN", "owner"} {
		if validRole(role) {
			t.Fatalf("expected role %q to be invalid", role)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := normalizeEmail("  USER@Example.COM \n"); got != "user@example.com" {
		t.Fatalf("normalizeEmail()=%q", got)
	}
}

func TestValidPassword(t *testing.T) {
	if !validPassword("123456789012") {
		t.Fatal("12-character password should be valid")
	}
	if validPassword("12345678901") {
		t.Fatal("11-character password should be invalid")
	}
}

func TestSessionTTL(t *testing.T) {
	if sessionTTL != 30*time.Minute {
		t.Fatalf("expected 30 minute session TTL, got %s", sessionTTL)
	}
}

func TestUsersCreateInputSanitizeAndValidate(t *testing.T) {
	input := usersCreateInput{Name: "  Alice  ", Email: " ALICE@EXAMPLE.COM ", Password: "LongPassword12!", Role: " ADMIN "}
	input.sanitize()
	if input.Name != "Alice" || input.Email != "alice@example.com" || input.Role != roleAdmin || !input.valid() {
		t.Fatalf("unexpected sanitized input: %+v", input)
	}

	input.Password = "short123"
	if input.valid() {
		t.Fatal("password shorter than 12 characters should be invalid")
	}
}

func TestUsersUpdateInputSanitize(t *testing.T) {
	input := usersUpdateInput{Name: "  Alice  ", Role: " AUDITOR "}
	input.sanitize()
	if input.Name != "Alice" || input.Role != roleAuditor {
		t.Fatalf("unexpected sanitized input: %+v", input)
	}
}

func TestParseClaimsAndMe(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	a := &app{secret: secret}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+authToken(t, secret, authClaims(roleAdmin)))

	claims, err := a.parseClaims(req)
	if err != nil || claims["role"] != roleAdmin {
		t.Fatalf("claims=%v err=%v", claims, err)
	}

	w := httptest.NewRecorder()
	a.me(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"role":"admin"`) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestParseClaimsRequiresExpiration(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	claims := authClaims(roleAdmin)
	delete(claims, "exp")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+authToken(t, secret, claims))
	if _, err := (&app{secret: secret}).parseClaims(req); err == nil {
		t.Fatal("token without exp should fail")
	}
}

func TestLoginRejectsInvalidRequestsBeforeDatabaseAccess(t *testing.T) {
	a := &app{}

	w := httptest.NewRecorder()
	a.login(w, httptest.NewRequest(http.MethodGet, "/api/auth/login", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.login(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@example.com","unknown":true}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestChangePasswordValidationBeforeDatabaseAccess(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	a := &app{secret: secret}

	w := httptest.NewRecorder()
	a.changePassword(w, httptest.NewRequest(http.MethodGet, "/api/auth/change-password", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", w.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"current_password":"old","new_password":"short"}`))
	req.Header.Set("Authorization", "Bearer "+authToken(t, secret, authClaims(roleAdmin)))
	w = httptest.NewRecorder()
	a.changePassword(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "at least 12") {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestUsersAuthorizationAndMethodValidation(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	a := &app{secret: secret}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+authToken(t, secret, authClaims(roleStaff)))
	w := httptest.NewRecorder()
	a.users(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin status=%d", w.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+authToken(t, secret, authClaims(roleAdmin)))
	w = httptest.NewRecorder()
	a.users(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unsupported method status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestUsersCreateAndUpdateValidationBeforeDatabaseAccess(t *testing.T) {
	a := &app{}

	w := httptest.NewRecorder()
	a.usersCreate(w, httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"A","email":"a@example.com","password":"short","role":"admin"}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.usersUpdate(w, httptest.NewRequest(http.MethodPatch, "/api/users", strings.NewReader(`{"id":1,"name":"A","role":"admin","password":"short"}`)))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "at least 12") {
		t.Fatalf("update status=%d body=%q", w.Code, w.Body.String())
	}
}
