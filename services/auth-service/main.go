package main

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/dbx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const sessionTTL = 30 * time.Minute

type app struct {
	db     *sql.DB
	secret []byte
}

type user struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Active bool   `json:"active"`
}

func main() {
	db, err := dbx.Open(httpx.Env("AUTH_DB_DSN", "tropical:tropical@tcp(mysql:3306)/tropical_auth?parseTime=true&charset=utf8mb4"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	a := &app{db: db, secret: []byte(httpx.Env("JWT_SECRET", "local-dev-secret-change-this-value"))}
	if err := a.migrate(); err != nil {
		log.Fatal(err)
	}
	if err := a.bootstrapAdmin(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "auth-service"})
	})
	mux.HandleFunc("/api/auth/login", a.login)
	mux.HandleFunc("/api/auth/me", a.me)
	mux.HandleFunc("/api/auth/change-password", a.changePassword)
	mux.HandleFunc("/api/users", a.users)

	log.Println("auth-service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (a *app) migrate() error {
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(120) NOT NULL,
		email VARCHAR(190) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		role VARCHAR(30) NOT NULL DEFAULT 'staff',
		active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	if err != nil {
		return err
	}
	// Existing local databases from Phase 2 do not yet have the active flag.
	// Duplicate-column errors are intentionally ignored so this remains restart-safe.
	_, _ = a.db.Exec(`ALTER TABLE users ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE`)
	return nil
}

func (a *app) bootstrapAdmin() error {
	email := httpx.Env("BOOTSTRAP_ADMIN_EMAIL", "admin@tropical.local")
	password := httpx.Env("BOOTSTRAP_ADMIN_PASSWORD", "ChangeThis123!")
	var count int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM users WHERE email=?", email).Scan(&count); err != nil || count > 0 {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = a.db.Exec("INSERT INTO users(name,email,password_hash,role,active) VALUES(?,?,?,?,TRUE)", "Tropical Admin", email, string(hash), "admin")
	return err
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var input struct{ Email, Password string }
	if err := httpx.DecodeJSON(r, &input); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	var u user
	var hash string
	if err := a.db.QueryRow("SELECT id,name,email,password_hash,role,active FROM users WHERE email=?", strings.ToLower(strings.TrimSpace(input.Email))).Scan(&u.ID, &u.Name, &u.Email, &hash, &u.Role, &u.Active); err != nil || !u.Active || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials or inactive user"})
		return
	}
	now := time.Now()
	claims := jwt.MapClaims{"sub": u.ID, "name": u.Name, "email": u.Email, "role": u.Role, "iat": now.Unix(), "exp": now.Add(sessionTTL).Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.secret)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"token": signed, "user": u})
}

func (a *app) parseClaims(r *http.Request) (jwt.MapClaims, error) {
	raw, err := httpx.BearerToken(r)
	if err != nil {
		return nil, err
	}
	token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) { return a.secret, nil }, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func (a *app) me(w http.ResponseWriter, r *http.Request) {
	claims, err := a.parseClaims(r)
	if err != nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	httpx.JSON(w, http.StatusOK, claims)
}

func (a *app) changePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	claims, err := a.parseClaims(r)
	if err != nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	email, _ := claims["email"].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid session"})
		return
	}

	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httpx.DecodeJSON(r, &input); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if len(input.NewPassword) < 12 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "new password must be at least 12 characters"})
		return
	}
	if input.CurrentPassword == input.NewPassword {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "new password must be different from current password"})
		return
	}

	var hash string
	var active bool
	if err := a.db.QueryRow("SELECT password_hash,active FROM users WHERE email=?", email).Scan(&hash, &active); err != nil || !active {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "account unavailable"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.CurrentPassword)) != nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "password hashing failed"})
		return
	}
	if _, err := a.db.Exec("UPDATE users SET password_hash=? WHERE email=?", string(newHash), email); err != nil {
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "password update failed"})
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "password updated; sign in again"})
}

func validRole(role string) bool {
	return role == "admin" || role == "auditor" || role == "staff"
}

func (a *app) users(w http.ResponseWriter, r *http.Request) {
	claims, err := a.parseClaims(r)
	if err != nil || claims["role"] != "admin" {
		httpx.JSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query("SELECT id,name,email,role,active FROM users ORDER BY id DESC")
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		result := []user{}
		for rows.Next() {
			var u user
			if rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active) == nil {
				result = append(result, u)
			}
		}
		httpx.JSON(w, 200, result)

	case http.MethodPost:
		var input struct{ Name, Email, Password, Role string }
		if err := httpx.DecodeJSON(r, &input); err != nil {
			httpx.JSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		role := strings.ToLower(strings.TrimSpace(input.Role))
		if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Email) == "" || len(input.Password) < 8 || !validRole(role) {
			httpx.JSON(w, 400, map[string]string{"error": "name/email required, password min 8 chars, and role must be admin/auditor/staff"})
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": "password hashing failed"})
			return
		}
		email := strings.ToLower(strings.TrimSpace(input.Email))
		res, err := a.db.Exec("INSERT INTO users(name,email,password_hash,role,active) VALUES(?,?,?,?,TRUE)", strings.TrimSpace(input.Name), email, string(hash), role)
		if err != nil {
			httpx.JSON(w, 409, map[string]string{"error": "user already exists or invalid"})
			return
		}
		id, _ := res.LastInsertId()
		httpx.JSON(w, 201, user{ID: id, Name: strings.TrimSpace(input.Name), Email: email, Role: role, Active: true})

	case http.MethodPatch:
		var input struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Role     string `json:"role"`
			Active   bool   `json:"active"`
			Password string `json:"password"`
		}
		if err := httpx.DecodeJSON(r, &input); err != nil || input.ID == 0 {
			httpx.JSON(w, 400, map[string]string{"error": "id and valid json required"})
			return
		}
		role := strings.ToLower(strings.TrimSpace(input.Role))
		if strings.TrimSpace(input.Name) == "" || !validRole(role) {
			httpx.JSON(w, 400, map[string]string{"error": "name and valid role required"})
			return
		}
		if input.Password != "" && len(input.Password) < 8 {
			httpx.JSON(w, 400, map[string]string{"error": "new password must be at least 8 characters"})
			return
		}
		if input.Password != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
			if err != nil {
				httpx.JSON(w, 500, map[string]string{"error": "password hashing failed"})
				return
			}
			_, err = a.db.Exec("UPDATE users SET name=?,role=?,active=?,password_hash=? WHERE id=?", strings.TrimSpace(input.Name), role, input.Active, string(hash), input.ID)
			if err != nil {
				httpx.JSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		} else {
			_, err = a.db.Exec("UPDATE users SET name=?,role=?,active=? WHERE id=?", strings.TrimSpace(input.Name), role, input.Active, input.ID)
			if err != nil {
				httpx.JSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
		}
		httpx.JSON(w, 200, map[string]any{"id": input.ID, "name": strings.TrimSpace(input.Name), "role": role, "active": input.Active})

	default:
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}
