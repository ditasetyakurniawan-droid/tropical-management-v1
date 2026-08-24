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

const (
	sessionTTL = 30 * time.Minute

	defaultDSN                 = "tropical:tropical@tcp(mysql:3306)/tropical_auth?parseTime=true&charset=utf8mb4"
	defaultJWTSecret           = "local-dev-secret-change-this-value"
	defaultBootstrapAdminEmail = "admin@tropical.local"
	defaultBootstrapAdminPass  = "ChangeThis123!"
	serviceName                = "auth-service"
	listenAddr                 = ":8080"

	// Role constants
	roleAdmin   = "admin"
	roleAuditor = "auditor"
	roleStaff   = "staff"

	// Error messages
	errMethodNotAllowed         = "method not allowed"
	errInvalidJSON              = "invalid json"
	errUnauthorized             = "unauthorized"
	errInvalidCredentials       = "invalid credentials or inactive user"
	errTokenGenerationFailed    = "token generation failed"
	errInvalidSession           = "invalid session"
	errAccountUnavailable       = "account unavailable"
	errCurrentPasswordIncorrect = "current password is incorrect"
	errPasswordHashingFailed    = "password hashing failed"
	errPasswordUpdateFailed     = "password update failed"
	errNewPasswordTooShort      = "new password must be at least 12 characters"
	errNewPasswordSameAsCurrent = "new password must be different from current password"
	errAdminRoleRequired        = "admin role required"
	errInvalidUserInput         = "name/email required, password min 8 chars, and role must be admin/auditor/staff"
	errUserAlreadyExists        = "user already exists or invalid"
	errIDAndJSONRequired        = "id and valid json required"
	errNameAndRoleRequired      = "name and valid role required"
	errNewPasswordTooShort8     = "new password must be at least 8 characters"

	// SQL queries
	createUsersTable = `CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(120) NOT NULL,
		email VARCHAR(190) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		role VARCHAR(30) NOT NULL DEFAULT 'staff',
		active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	alterUsersAddActive = `ALTER TABLE users ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE`

	selectUserForLogin = `SELECT id,name,email,password_hash,role,active FROM users WHERE email=?`

	insertUser = `INSERT INTO users(name,email,password_hash,role,active) VALUES(?,?,?,?,TRUE)`

	selectUserPasswordByEmail = `SELECT password_hash,active FROM users WHERE email=?`

	updateUserPassword = `UPDATE users SET password_hash=? WHERE email=?`

	selectUsersList = `SELECT id,name,email,role,active FROM users ORDER BY id DESC`

	updateUserWithPassword = `UPDATE users SET name=?,role=?,active=?,password_hash=? WHERE id=?`

	updateUserWithoutPassword = `UPDATE users SET name=?,role=?,active=? WHERE id=?`

	countUserByEmail = `SELECT COUNT(*) FROM users WHERE email=?`
)

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
	db, err := dbx.Open(httpx.Env("AUTH_DB_DSN", defaultDSN))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	a := &app{
		db:     db,
		secret: []byte(httpx.Env("JWT_SECRET", defaultJWTSecret)),
	}
	if err := a.migrate(); err != nil {
		log.Fatal(err)
	}
	if err := a.bootstrapAdmin(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": serviceName})
	})
	mux.HandleFunc("/api/auth/login", a.login)
	mux.HandleFunc("/api/auth/me", a.me)
	mux.HandleFunc("/api/auth/change-password", a.changePassword)
	mux.HandleFunc("/api/users", a.users)

	log.Println(serviceName + " listening on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}

// writeError mengirimkan JSON error yang konsisten.
func writeError(w http.ResponseWriter, status int, msg string) {
	httpx.JSON(w, status, map[string]string{"error": msg})
}

// normalizeEmail membersihkan dan menyeragamkan format email.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (a *app) migrate() error {
	if _, err := a.db.Exec(createUsersTable); err != nil {
		return err
	}
	// Existing local databases from Phase 2 do not yet have the active flag.
	// Duplicate-column errors are intentionally ignored so this remains restart-safe.
	if _, err := a.db.Exec(alterUsersAddActive); err != nil {
		log.Printf("migrate warning: %v", err)
	}
	return nil
}

func (a *app) bootstrapAdmin() error {
	email := normalizeEmail(httpx.Env("BOOTSTRAP_ADMIN_EMAIL", defaultBootstrapAdminEmail))
	password := httpx.Env("BOOTSTRAP_ADMIN_PASSWORD", defaultBootstrapAdminPass)

	var count int
	if err := a.db.QueryRow(countUserByEmail, email).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // admin already exists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(insertUser, "Tropical Admin", email, string(hash), roleAdmin)
	return err
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	email := normalizeEmail(input.Email)
	var u user
	var hash string
	err := a.db.QueryRow(selectUserForLogin, email).Scan(
		&u.ID, &u.Name, &u.Email, &hash, &u.Role, &u.Active,
	)
	if err != nil || !u.Active || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil {
		writeError(w, http.StatusUnauthorized, errInvalidCredentials)
		return
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"sub":   u.ID,
		"name":  u.Name,
		"email": u.Email,
		"role":  u.Role,
		"iat":   now.Unix(),
		"exp":   now.Add(sessionTTL).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errTokenGenerationFailed)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"token": signed, "user": u})
}

func (a *app) parseClaims(r *http.Request) (jwt.MapClaims, error) {
	raw, err := httpx.BearerToken(r)
	if err != nil {
		return nil, err
	}
	token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) { return a.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}))
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
		writeError(w, http.StatusUnauthorized, errUnauthorized)
		return
	}
	httpx.JSON(w, http.StatusOK, claims)
}

func (a *app) changePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	claims, err := a.parseClaims(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, errUnauthorized)
		return
	}

	email, _ := claims["email"].(string)
	email = normalizeEmail(email)
	if email == "" {
		writeError(w, http.StatusUnauthorized, errInvalidSession)
		return
	}

	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := httpx.DecodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}
	if len(input.NewPassword) < 12 {
		writeError(w, http.StatusBadRequest, errNewPasswordTooShort)
		return
	}
	if input.CurrentPassword == input.NewPassword {
		writeError(w, http.StatusBadRequest, errNewPasswordSameAsCurrent)
		return
	}

	var hash string
	var active bool
	if err := a.db.QueryRow(selectUserPasswordByEmail, email).Scan(&hash, &active); err != nil || !active {
		writeError(w, http.StatusUnauthorized, errAccountUnavailable)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.CurrentPassword)) != nil {
		writeError(w, http.StatusUnauthorized, errCurrentPasswordIncorrect)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errPasswordHashingFailed)
		return
	}
	if _, err := a.db.Exec(updateUserPassword, string(newHash), email); err != nil {
		writeError(w, http.StatusInternalServerError, errPasswordUpdateFailed)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"status": "password updated; sign in again"})
}

func validRole(role string) bool {
	switch role {
	case roleAdmin, roleAuditor, roleStaff:
		return true
	default:
		return false
	}
}

// users adalah entry point untuk /api/users. Setiap HTTP method didelegasikan
// ke handler-nya sendiri supaya function ini tetap sederhana (cognitive complexity rendah).
func (a *app) users(w http.ResponseWriter, r *http.Request) {
	claims, err := a.parseClaims(r)
	if err != nil || claims["role"] != roleAdmin {
		writeError(w, http.StatusForbidden, errAdminRoleRequired)
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.usersList(w)
	case http.MethodPost:
		a.usersCreate(w, r)
	case http.MethodPatch:
		a.usersUpdate(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

// usersList menangani GET /api/users.
func (a *app) usersList(w http.ResponseWriter) {
	rows, err := a.db.Query(selectUsersList)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	result := []user{}
	for rows.Next() {
		var u user
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result = append(result, u)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// usersCreateInput adalah payload untuk POST /api/users.
type usersCreateInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// sanitize membersihkan dan menyeragamkan field input.
func (in *usersCreateInput) sanitize() {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = normalizeEmail(in.Email)
	in.Role = strings.ToLower(strings.TrimSpace(in.Role))
}

// valid memeriksa apakah input POST sudah lengkap dan sah.
func (in usersCreateInput) valid() bool {
	return in.Name != "" && in.Email != "" && len(in.Password) >= 8 && validRole(in.Role)
}

// usersCreate menangani POST /api/users.
func (a *app) usersCreate(w http.ResponseWriter, r *http.Request) {
	var input usersCreateInput
	if err := httpx.DecodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}
	input.sanitize()
	if !input.valid() {
		writeError(w, http.StatusBadRequest, errInvalidUserInput)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, errPasswordHashingFailed)
		return
	}

	res, err := a.db.Exec(insertUser, input.Name, input.Email, string(hash), input.Role)
	if err != nil {
		writeError(w, http.StatusConflict, errUserAlreadyExists)
		return
	}
	id, _ := res.LastInsertId()
	httpx.JSON(w, http.StatusCreated, user{
		ID:     id,
		Name:   input.Name,
		Email:  input.Email,
		Role:   input.Role,
		Active: true,
	})
}

// usersUpdateInput adalah payload untuk PATCH /api/users.
type usersUpdateInput struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Active   bool   `json:"active"`
	Password string `json:"password"`
}

// sanitize membersihkan dan menyeragamkan field input.
func (in *usersUpdateInput) sanitize() {
	in.Name = strings.TrimSpace(in.Name)
	in.Role = strings.ToLower(strings.TrimSpace(in.Role))
}

// usersUpdate menangani PATCH /api/users.
func (a *app) usersUpdate(w http.ResponseWriter, r *http.Request) {
	var input usersUpdateInput
	if err := httpx.DecodeJSON(r, &input); err != nil || input.ID == 0 {
		writeError(w, http.StatusBadRequest, errIDAndJSONRequired)
		return
	}
	input.sanitize()

	if input.Name == "" || !validRole(input.Role) {
		writeError(w, http.StatusBadRequest, errNameAndRoleRequired)
		return
	}
	if input.Password != "" && len(input.Password) < 8 {
		writeError(w, http.StatusBadRequest, errNewPasswordTooShort8)
		return
	}

	if err := a.applyUserUpdate(input); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":     input.ID,
		"name":   input.Name,
		"role":   input.Role,
		"active": input.Active,
	})
}

// applyUserUpdate menjalankan query UPDATE yang sesuai, tergantung ada tidaknya
// password baru.
func (a *app) applyUserUpdate(input usersUpdateInput) error {
	if input.Password == "" {
		_, err := a.db.Exec(updateUserWithoutPassword, input.Name, input.Role, input.Active, input.ID)
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(updateUserWithPassword, input.Name, input.Role, input.Active, string(hash), input.ID)
	return err
}
