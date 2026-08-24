package main

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/dbx"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
)

const (
	defaultDSN  = "tropical:tropical@tcp(mysql:3306)/tropical_audit?parseTime=true&charset=utf8mb4"
	serviceName = "audit-service"
	listenAddr  = ":8080"

	// Error messages
	errInvalidJSON           = "invalid json"
	errMethodNotAllowed      = "method not allowed"
	errRestaurantAuditor     = "restaurant and auditor are required"
	errScoreRange            = "checklist scores must be 0-100"
	errIssueTitleRequired    = "issue title is required"
	errInvalidSeverityStatus = "invalid severity or status"
	errInvalidIssueStatus    = "invalid issue status"
	errIDAndJSONRequired     = "id and valid json required"

	// Severity values
	severityLow      = "low"
	severityMedium   = "medium"
	severityHigh     = "high"
	severityCritical = "critical"

	// Issue status values
	statusOpen       = "open"
	statusInProgress = "in_progress"
	statusResolved   = "resolved"
	statusVerified   = "verified"
	statusClosed     = "closed"

	// SQL queries
	createAuditsTable = `CREATE TABLE IF NOT EXISTS audits (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		restaurant VARCHAR(150) NOT NULL,
		auditor VARCHAR(150) NOT NULL,
		cleanliness INT NOT NULL,
		sop INT NOT NULL,
		food_quality INT NOT NULL,
		score DECIMAL(5,2) NOT NULL,
		notes TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	createIssuesTable = `CREATE TABLE IF NOT EXISTS issues (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		audit_id BIGINT NULL,
		title VARCHAR(220) NOT NULL,
		severity VARCHAR(20) NOT NULL,
		status VARCHAR(30) NOT NULL DEFAULT 'open',
		assigned_to VARCHAR(150),
		due_date DATE NULL,
		corrective_action TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_issue_status(status),
		INDEX idx_issue_due_date(due_date)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	selectAuditsQuery = `SELECT id,restaurant,auditor,cleanliness,sop,food_quality,score,COALESCE(notes,''),created_at
		FROM audits ORDER BY id DESC LIMIT 100`

	insertAuditQuery = `INSERT INTO audits(restaurant,auditor,cleanliness,sop,food_quality,score,notes)
		VALUES(?,?,?,?,?,?,?)`

	selectIssuesQuery = `SELECT id,COALESCE(audit_id,0),title,severity,status,
		COALESCE(assigned_to,''),COALESCE(DATE_FORMAT(due_date,'%Y-%m-%d'),''),
		COALESCE(corrective_action,''),created_at,updated_at
		FROM issues
		ORDER BY CASE severity
			WHEN 'critical' THEN 1
			WHEN 'high' THEN 2
			WHEN 'medium' THEN 3
			ELSE 4
		END, id DESC LIMIT 200`

	insertIssueQuery = `INSERT INTO issues(audit_id,title,severity,status,assigned_to,due_date,corrective_action)
		VALUES(NULLIF(?,0),?,?,?,?,NULLIF(?,''),?)`

	updateIssueQuery = `UPDATE issues SET status=?,assigned_to=?,due_date=NULLIF(?,''),corrective_action=?
		WHERE id=?`
)

type app struct{ db *sql.DB }

type audit struct {
	ID          int64     `json:"id"`
	Restaurant  string    `json:"restaurant"`
	Auditor     string    `json:"auditor"`
	Cleanliness int       `json:"cleanliness"`
	SOP         int       `json:"sop"`
	FoodQuality int       `json:"food_quality"`
	Score       float64   `json:"score"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
}

type issue struct {
	ID               int64     `json:"id"`
	AuditID          int64     `json:"audit_id"`
	Title            string    `json:"title"`
	Severity         string    `json:"severity"`
	Status           string    `json:"status"`
	AssignedTo       string    `json:"assigned_to"`
	DueDate          string    `json:"due_date"`
	CorrectiveAction string    `json:"corrective_action"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func main() {
	db, err := dbx.Open(httpx.Env("AUDIT_DB_DSN", defaultDSN))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	a := &app{db: db}
	if err := a.migrate(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/api/audits", a.audits)
	mux.HandleFunc("/api/issues", a.issues)
	mux.HandleFunc("/internal/summary", a.summary)

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
	queries := []string{createAuditsTable, createIssuesTable}
	for _, q := range queries {
		if _, err := a.db.Exec(q); err != nil {
			return err
		}
	}

	// Phase-2 databases already have issues. These ALTERs are restart-safe because
	// duplicate-column/index errors are intentionally ignored during local migration.
	alterQueries := []string{
		`ALTER TABLE issues ADD COLUMN due_date DATE NULL`,
		`ALTER TABLE issues ADD COLUMN corrective_action TEXT`,
		`ALTER TABLE issues ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP`,
		`ALTER TABLE issues ADD INDEX idx_issue_due_date(due_date)`,
	}
	for _, q := range alterQueries {
		if _, err := a.db.Exec(q); err != nil {
			log.Printf("migrate warning: %v", err)
		}
	}
	return nil
}

func validSeverity(v string) bool {
	switch v {
	case severityLow, severityMedium, severityHigh, severityCritical:
		return true
	default:
		return false
	}
}

func validIssueStatus(v string) bool {
	switch v {
	case statusOpen, statusInProgress, statusResolved, statusVerified, statusClosed:
		return true
	default:
		return false
	}
}

// ============================================================
// AUDITS
// ============================================================

func (a *app) audits(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getAudits(w)
	case http.MethodPost:
		a.createAudit(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) getAudits(w http.ResponseWriter) {
	rows, err := a.db.Query(selectAuditsQuery)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	result := []audit{}
	for rows.Next() {
		var x audit
		if err := rows.Scan(&x.ID, &x.Restaurant, &x.Auditor, &x.Cleanliness, &x.SOP, &x.FoodQuality, &x.Score, &x.Notes, &x.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result = append(result, x)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (a *app) createAudit(w http.ResponseWriter, r *http.Request) {
	var x audit
	if err := httpx.DecodeJSON(r, &x); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	x.Restaurant = strings.TrimSpace(x.Restaurant)
	x.Auditor = strings.TrimSpace(x.Auditor)
	x.Notes = strings.TrimSpace(x.Notes)

	if errMsg := validateAudit(x); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	x.Score = float64(x.Cleanliness+x.SOP+x.FoodQuality) / 3

	res, err := a.db.Exec(insertAuditQuery, x.Restaurant, x.Auditor, x.Cleanliness, x.SOP, x.FoodQuality, x.Score, x.Notes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	x.ID, _ = res.LastInsertId()
	x.CreatedAt = time.Now()
	httpx.JSON(w, http.StatusCreated, x)
}

func validateAudit(x audit) string {
	if x.Restaurant == "" || x.Auditor == "" {
		return errRestaurantAuditor
	}
	if x.Cleanliness < 0 || x.Cleanliness > 100 {
		return errScoreRange
	}
	if x.SOP < 0 || x.SOP > 100 {
		return errScoreRange
	}
	if x.FoodQuality < 0 || x.FoodQuality > 100 {
		return errScoreRange
	}
	return ""
}

// ============================================================
// ISSUES
// ============================================================

func (a *app) issues(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.getIssues(w)
	case http.MethodPost:
		a.createIssue(w, r)
	case http.MethodPatch:
		a.updateIssue(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) getIssues(w http.ResponseWriter) {
	rows, err := a.db.Query(selectIssuesQuery)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	result := []issue{}
	for rows.Next() {
		var x issue
		if err := rows.Scan(&x.ID, &x.AuditID, &x.Title, &x.Severity, &x.Status, &x.AssignedTo, &x.DueDate, &x.CorrectiveAction, &x.CreatedAt, &x.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result = append(result, x)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (a *app) createIssue(w http.ResponseWriter, r *http.Request) {
	var x issue
	if err := httpx.DecodeJSON(r, &x); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidJSON)
		return
	}

	x.Title = strings.TrimSpace(x.Title)
	x.AssignedTo = strings.TrimSpace(x.AssignedTo)
	x.CorrectiveAction = strings.TrimSpace(x.CorrectiveAction)

	applyIssueDefaults(&x)

	if errMsg := validateIssue(x); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	res, err := a.db.Exec(insertIssueQuery, x.AuditID, x.Title, x.Severity, x.Status, x.AssignedTo, x.DueDate, x.CorrectiveAction)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	x.ID, _ = res.LastInsertId()
	x.CreatedAt = time.Now()
	x.UpdatedAt = x.CreatedAt
	httpx.JSON(w, http.StatusCreated, x)
}

func applyIssueDefaults(x *issue) {
	if x.Severity == "" {
		x.Severity = severityMedium
	}
	if x.Status == "" {
		x.Status = statusOpen
	}
}

func validateIssue(x issue) string {
	if x.Title == "" {
		return errIssueTitleRequired
	}
	if !validSeverity(x.Severity) {
		return errInvalidSeverityStatus
	}
	if !validIssueStatus(x.Status) {
		return errInvalidSeverityStatus
	}
	return ""
}

func (a *app) updateIssue(w http.ResponseWriter, r *http.Request) {
	var x issue
	if err := httpx.DecodeJSON(r, &x); err != nil || x.ID == 0 {
		writeError(w, http.StatusBadRequest, errIDAndJSONRequired)
		return
	}

	x.Status = strings.TrimSpace(x.Status)
	x.AssignedTo = strings.TrimSpace(x.AssignedTo)
	x.CorrectiveAction = strings.TrimSpace(x.CorrectiveAction)

	if !validIssueStatus(x.Status) {
		writeError(w, http.StatusBadRequest, errInvalidIssueStatus)
		return
	}

	_, err := a.db.Exec(updateIssueQuery, x.Status, x.AssignedTo, x.DueDate, x.CorrectiveAction, x.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":                x.ID,
		"status":            x.Status,
		"assigned_to":       x.AssignedTo,
		"due_date":          x.DueDate,
		"corrective_action": x.CorrectiveAction,
	})
}

// ============================================================
// SUMMARY
// ============================================================

func (a *app) summary(w http.ResponseWriter, _ *http.Request) {
	score, err := a.avgScore30Days()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	open, err := a.countOpenIssues()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	overdue, err := a.countOverdueIssues()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	critical, err := a.countCriticalIssues()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"audit_score":     score,
		"open_issues":     open,
		"overdue_issues":  overdue,
		"critical_issues": critical,
	})
}

func (a *app) avgScore30Days() (float64, error) {
	var score float64
	err := a.db.QueryRow(`
		SELECT COALESCE(AVG(score),0) 
		FROM audits 
		WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)
	`).Scan(&score)
	return score, err
}

func (a *app) countOpenIssues() (int, error) {
	var count int
	err := a.db.QueryRow(`SELECT COUNT(*) FROM issues WHERE status <> 'closed'`).Scan(&count)
	return count, err
}

func (a *app) countOverdueIssues() (int, error) {
	var count int
	err := a.db.QueryRow(`
		SELECT COUNT(*) FROM issues 
		WHERE due_date < CURDATE() AND status NOT IN ('closed','verified')
	`).Scan(&count)
	return count, err
}

func (a *app) countCriticalIssues() (int, error) {
	var count int
	err := a.db.QueryRow(`
		SELECT COUNT(*) FROM issues 
		WHERE severity = 'critical' AND status <> 'closed'
	`).Scan(&count)
	return count, err
}
