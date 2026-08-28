package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
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

func TestAuthDatabasePaths(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	oldPassword := "CurrentPass123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	db, script := openTestDB(t,
		execStep("CREATE TABLE IF NOT EXISTS users", 0, 0),
		execStep("ALTER TABLE users ADD COLUMN active", 0, 0),
		queryStep("SELECT COUNT(*) FROM users WHERE email", []string{"count"}, row(int64(1))),
		queryStep("FROM users WHERE email", []string{"id", "name", "email", "password_hash", "role", "active"},
			row(int64(7), "Admin User", "admin@example.com", string(hash), roleAdmin, true)),
		queryStep("SELECT password_hash,active FROM users", []string{"password_hash", "active"}, row(string(hash), true)),
		execStep("UPDATE users SET password_hash", 0, 1),
		queryStep("SELECT id,name,email,role,active FROM users", []string{"id", "name", "email", "role", "active"},
			row(int64(7), "Admin User", "admin@example.com", roleAdmin, true)),
		execStep("INSERT INTO users", 8, 1),
		execStep("UPDATE users SET name", 0, 1),
	)
	a := &app{db: db, secret: secret}

	if err := a.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := a.bootstrapAdmin(); err != nil {
		t.Fatalf("bootstrap existing admin: %v", err)
	}

	w := httptest.NewRecorder()
	a.login(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":" ADMIN@EXAMPLE.COM ","password":"CurrentPass123!"}`)))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"token"`) || !strings.Contains(w.Body.String(), `"role":"admin"`) {
		t.Fatalf("login status=%d body=%q", w.Code, w.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"current_password":"CurrentPass123!","new_password":"DifferentPass123!"}`))
	req.Header.Set("Authorization", "Bearer "+authToken(t, secret, authClaims(roleAdmin)))
	w = httptest.NewRecorder()
	a.changePassword(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "password updated") {
		t.Fatalf("change password status=%d body=%q", w.Code, w.Body.String())
	}

	adminToken := authToken(t, secret, authClaims(roleAdmin))
	req = httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	a.users(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"admin@example.com"`) {
		t.Fatalf("users list status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":" Alice ","email":" ALICE@EXAMPLE.COM ","password":"LongPassword12!","role":"staff"}`))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	a.users(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":8`) || !strings.Contains(w.Body.String(), `"email":"alice@example.com"`) {
		t.Fatalf("users create status=%d body=%q", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/users", strings.NewReader(`{"id":8,"name":" Alice Updated ","role":"auditor","active":true}`))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	a.users(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"role":"auditor"`) {
		t.Fatalf("users update status=%d body=%q", w.Code, w.Body.String())
	}
	script.assertDone(t)
}

func TestAuthDatabaseErrorsAndCredentialFailures(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	db, script := openTestDB(t,
		queryStep("FROM users WHERE email", []string{"id", "name", "email", "password_hash", "role", "active"}),
		queryErrorStep("SELECT id,name,email,role,active FROM users", errors.New("db unavailable")),
	)
	a := &app{db: db, secret: secret}

	w := httptest.NewRecorder()
	a.login(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"nobody@example.com","password":"LongPassword12!"}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing user status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.usersList(w)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("users list db error status=%d body=%q", w.Code, w.Body.String())
	}
	script.assertDone(t)
}

func TestAuthAdditionalValidationBranches(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	a := &app{secret: secret}

	t.Run("me rejects missing token", func(t *testing.T) {
		w := httptest.NewRecorder()
		a.me(w, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("change password rejects invalid session email", func(t *testing.T) {
		claims := authClaims(roleAdmin)
		delete(claims, "email")
		req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"current_password":"CurrentPass123!","new_password":"DifferentPass123!"}`))
		req.Header.Set("Authorization", "Bearer "+authToken(t, secret, claims))
		w := httptest.NewRecorder()
		a.changePassword(w, req)
		if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "invalid session") {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("change password rejects malformed payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"current_password":"x","unknown":true}`))
		req.Header.Set("Authorization", "Bearer "+authToken(t, secret, authClaims(roleAdmin)))
		w := httptest.NewRecorder()
		a.changePassword(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("change password rejects same password", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"current_password":"SamePassword12!","new_password":"SamePassword12!"}`))
		req.Header.Set("Authorization", "Bearer "+authToken(t, secret, authClaims(roleAdmin)))
		w := httptest.NewRecorder()
		a.changePassword(w, req)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "different") {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("users create rejects malformed payload", func(t *testing.T) {
		w := httptest.NewRecorder()
		a.usersCreate(w, httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"A","unknown":true}`)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("users update rejects missing id and invalid role", func(t *testing.T) {
		w := httptest.NewRecorder()
		a.usersUpdate(w, httptest.NewRequest(http.MethodPatch, "/api/users", strings.NewReader(`{"name":"A","role":"admin"}`)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("missing id status=%d body=%q", w.Code, w.Body.String())
		}

		w = httptest.NewRecorder()
		a.usersUpdate(w, httptest.NewRequest(http.MethodPatch, "/api/users", strings.NewReader(`{"id":1,"name":" ","role":"owner"}`)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid role status=%d body=%q", w.Code, w.Body.String())
		}
	})
}

func TestBootstrapAdminRejectsWeakConfiguredPassword(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "short")
	if err := (&app{}).bootstrapAdmin(); err == nil || !strings.Contains(err.Error(), "at least 12") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyUserUpdatePasswordAndDatabaseErrors(t *testing.T) {
	t.Run("updates password hash when supplied", func(t *testing.T) {
		db, script := openTestDB(t, execStep("password_hash", 0, 1))
		a := &app{db: db}
		input := usersUpdateInput{ID: 9, Name: "Alice", Role: roleStaff, Active: true, Password: "LongPassword12!"}
		if err := a.applyUserUpdate(input); err != nil {
			t.Fatalf("apply update with password: %v", err)
		}
		script.assertDone(t)
	})

	t.Run("returns database error without password", func(t *testing.T) {
		boom := errors.New("update failed")
		db, script := openTestDB(t, execErrorStep("active=? WHERE id=?", boom))
		a := &app{db: db}
		input := usersUpdateInput{ID: 9, Name: "Alice", Role: roleStaff, Active: true}
		if err := a.applyUserUpdate(input); !errors.Is(err, boom) {
			t.Fatalf("expected database error, got %v", err)
		}
		script.assertDone(t)
	})

	t.Run("returns database error with password", func(t *testing.T) {
		boom := errors.New("update failed")
		db, script := openTestDB(t, execErrorStep("password_hash", boom))
		a := &app{db: db}
		input := usersUpdateInput{ID: 9, Name: "Alice", Role: roleAuditor, Active: true, Password: "LongPassword12!"}
		if err := a.applyUserUpdate(input); !errors.Is(err, boom) {
			t.Fatalf("expected database error, got %v", err)
		}
		script.assertDone(t)
	})
}

func TestBootstrapAdminAndMigrationBranches(t *testing.T) {
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL_FILE", "")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD_FILE", "")

	t.Run("rejects short bootstrap password before database access", func(t *testing.T) {
		t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "ADMIN@EXAMPLE.COM")
		t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "short")
		if err := (&app{}).bootstrapAdmin(); err == nil || !strings.Contains(err.Error(), "at least 12") {
			t.Fatalf("expected short-password error, got %v", err)
		}
	})

	t.Run("creates admin when account is missing", func(t *testing.T) {
		t.Setenv("BOOTSTRAP_ADMIN_EMAIL", " ADMIN@EXAMPLE.COM ")
		t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "LongPassword12!")
		db, script := openTestDB(t,
			queryStep("SELECT COUNT(*) FROM users WHERE email", []string{"count"}, row(int64(0))),
			execStep("INSERT INTO users", 1, 1),
		)
		a := &app{db: db}
		if err := a.bootstrapAdmin(); err != nil {
			t.Fatalf("bootstrap admin: %v", err)
		}
		script.assertDone(t)
	})

	t.Run("returns count query error", func(t *testing.T) {
		t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
		t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "LongPassword12!")
		boom := errors.New("count failed")
		db, script := openTestDB(t, queryErrorStep("SELECT COUNT(*) FROM users WHERE email", boom))
		if err := (&app{db: db}).bootstrapAdmin(); !errors.Is(err, boom) {
			t.Fatalf("expected count error, got %v", err)
		}
		script.assertDone(t)
	})

	t.Run("returns insert error", func(t *testing.T) {
		t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
		t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "LongPassword12!")
		boom := errors.New("insert failed")
		db, script := openTestDB(t,
			queryStep("SELECT COUNT(*) FROM users WHERE email", []string{"count"}, row(int64(0))),
			execErrorStep("INSERT INTO users", boom),
		)
		if err := (&app{db: db}).bootstrapAdmin(); !errors.Is(err, boom) {
			t.Fatalf("expected insert error, got %v", err)
		}
		script.assertDone(t)
	})

	t.Run("migration returns create-table error", func(t *testing.T) {
		boom := errors.New("create failed")
		db, script := openTestDB(t, execErrorStep("CREATE TABLE IF NOT EXISTS users", boom))
		if err := (&app{db: db}).migrate(); !errors.Is(err, boom) {
			t.Fatalf("expected migration error, got %v", err)
		}
		script.assertDone(t)
	})

	t.Run("migration tolerates alter error", func(t *testing.T) {
		boom := errors.New("duplicate column")
		db, script := openTestDB(t,
			execStep("CREATE TABLE IF NOT EXISTS users", 0, 0),
			execErrorStep("ALTER TABLE users ADD COLUMN active", boom),
		)
		if err := (&app{db: db}).migrate(); err != nil {
			t.Fatalf("alter error should be restart-safe, got %v", err)
		}
		script.assertDone(t)
	})
}

func TestChangePasswordCredentialAndDatabaseBranches(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	oldPassword := "CurrentPass123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	newRequest := func(current, next string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{"current_password":"`+current+`","new_password":"`+next+`"}`))
		req.Header.Set("Authorization", "Bearer "+authToken(t, secret, authClaims(roleAdmin)))
		return req
	}

	t.Run("rejects same current and new password", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&app{secret: secret}).changePassword(w, newRequest(oldPassword, oldPassword))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("rejects inactive account", func(t *testing.T) {
		db, script := openTestDB(t, queryStep("SELECT password_hash,active FROM users", []string{"password_hash", "active"}, row(string(hash), false)))
		w := httptest.NewRecorder()
		(&app{db: db, secret: secret}).changePassword(w, newRequest(oldPassword, "DifferentPass123!"))
		if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), errAccountUnavailable) {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("rejects incorrect current password", func(t *testing.T) {
		db, script := openTestDB(t, queryStep("SELECT password_hash,active FROM users", []string{"password_hash", "active"}, row(string(hash), true)))
		w := httptest.NewRecorder()
		(&app{db: db, secret: secret}).changePassword(w, newRequest("WrongPassword12!", "DifferentPass123!"))
		if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), errCurrentPasswordIncorrect) {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("masks update database failure", func(t *testing.T) {
		boom := errors.New("update failed")
		db, script := openTestDB(t,
			queryStep("SELECT password_hash,active FROM users", []string{"password_hash", "active"}, row(string(hash), true)),
			execErrorStep("UPDATE users SET password_hash", boom),
		)
		w := httptest.NewRecorder()
		(&app{db: db, secret: secret}).changePassword(w, newRequest(oldPassword, "DifferentPass123!"))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
}

func TestUserMutationDatabaseErrorsAreHandled(t *testing.T) {
	t.Run("create user database error", func(t *testing.T) {
		boom := errors.New("insert user failed")
		db, script := openTestDB(t, execErrorStep("INSERT INTO users", boom))
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.usersCreate(w, httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"Alice","email":"alice@example.com","password":"LongPassword12!","role":"staff"}`)))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("update user database error", func(t *testing.T) {
		boom := errors.New("update user failed")
		db, script := openTestDB(t, execErrorStep("active=? WHERE id=?", boom))
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.usersUpdate(w, httptest.NewRequest(http.MethodPatch, "/api/users", strings.NewReader(`{"id":8,"name":"Alice","role":"auditor","active":true}`)))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
}
