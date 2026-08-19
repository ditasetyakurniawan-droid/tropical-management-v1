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

type app struct {
	db     *sql.DB
	secret []byte
}

type user struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
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
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "auth-service"}) })
	mux.HandleFunc("/api/auth/login", a.login)
	mux.HandleFunc("/api/auth/me", a.me)
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
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	return err
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
	_, err = a.db.Exec("INSERT INTO users(name,email,password_hash,role) VALUES(?,?,?,?)", "Tropical Admin", email, string(hash), "admin")
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
	if err := a.db.QueryRow("SELECT id,name,email,password_hash,role FROM users WHERE email=?", strings.ToLower(strings.TrimSpace(input.Email))).Scan(&u.ID, &u.Name, &u.Email, &hash, &u.Role); err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil {
		httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	now := time.Now()
	claims := jwt.MapClaims{"sub": u.ID, "name": u.Name, "email": u.Email, "role": u.Role, "iat": now.Unix(), "exp": now.Add(8 * time.Hour).Unix()}
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

func (a *app) users(w http.ResponseWriter, r *http.Request) {
	claims, err := a.parseClaims(r)
	if err != nil || claims["role"] != "admin" {
		httpx.JSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query("SELECT id,name,email,role FROM users ORDER BY id DESC")
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		result := []user{}
		for rows.Next() {
			var u user
			if rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role) == nil {
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
		role := strings.ToLower(input.Role)
		if role != "admin" && role != "auditor" && role != "staff" {
			httpx.JSON(w, 400, map[string]string{"error": "role must be admin, auditor, or staff"})
			return
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		res, err := a.db.Exec("INSERT INTO users(name,email,password_hash,role) VALUES(?,?,?,?)", input.Name, strings.ToLower(input.Email), string(hash), role)
		if err != nil {
			httpx.JSON(w, 409, map[string]string{"error": "user already exists or invalid"})
			return
		}
		id, _ := res.LastInsertId()
		httpx.JSON(w, 201, user{ID: id, Name: input.Name, Email: strings.ToLower(input.Email), Role: role})
	default:
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}
