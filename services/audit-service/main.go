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
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": serviceName})
	})
	mux.HandleFunc("/api/audits", a.audits)
	mux.HandleFunc("/api/issues", a.issues)
	mux.HandleFunc("/internal/summary", a.summary)

	log.Println(serviceName + " listening on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}

// writeError is a helper to send consistent JSON error responses.
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

func (a *app) audits(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
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

	case http.MethodPost:
		var x audit
		if err := httpx.DecodeJSON(r, &x); err != nil {
			writeError(w, http.StatusBadRequest, errInvalidJSON)
			return
		}
		x.Restaurant = strings.TrimSpace(x.Restaurant)
		x.Auditor = strings.TrimSpace(x.Auditor)
		x.Notes = strings.TrimSpace(x.Notes)

		if x.Restaurant == "" || x.Auditor == "" {
			writeError(w, http.StatusBadRequest, errRestaurantAuditor)
			return
		}
		if x.Cleanliness < 0 || x.Cleanliness > 100 || x.SOP < 0 || x.SOP > 100 || x.FoodQuality < 0 || x.FoodQuality > 100 {
			writeError(w, http.StatusBadRequest, errScoreRange)
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

	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) issues(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
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

	case http.MethodPost:
		var x issue
		if err := httpx.DecodeJSON(r, &x); err != nil {
			writeError(w, http.StatusBadRequest, errInvalidJSON)
			return
		}
		x.Title = strings.TrimSpace(x.Title)
		x.AssignedTo = strings.TrimSpace(x.AssignedTo)
		x.CorrectiveAction = strings.TrimSpace(x.CorrectiveAction)

		if x.Title == "" {
			writeError(w, http.StatusBadRequest, errIssueTitleRequired)
			return
		}
		if x.Severity == "" {
			x.Severity = severityMedium
		}
		if x.Status == "" {
			x.Status = statusOpen
		}
		if !validSeverity(x.Severity) || !validIssueStatus(x.Status) {
			writeError(w, http.StatusBadRequest, errInvalidSeverityStatus)
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

	case http.MethodPatch:
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

	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (a *app) summary(w http.ResponseWriter, _ *http.Request) {
	var score float64
	var open, overdue, critical int

	// Query 1: rata-rata skor 30 hari terakhir
	err := a.db.QueryRow(`
		SELECT COALESCE(AVG(score),0) 
		FROM audits 
		WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY)
	`).Scan(&score)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Query 2: jumlah issue yang masih terbuka
	err = a.db.QueryRow(`
		SELECT COUNT(*) 
		FROM issues 
		WHERE status <> 'closed'
	`).Scan(&open)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Query 3: jumlah issue yang lewat jatuh tempo
	err = a.db.QueryRow(`
		SELECT COUNT(*) 
		FROM issues 
		WHERE due_date < CURDATE() 
		  AND status NOT IN ('closed','verified')
	`).Scan(&overdue)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Query 4: jumlah issue kritikal yang belum ditutup
	err = a.db.QueryRow(`
		SELECT COUNT(*) 
		FROM issues 
		WHERE severity = 'critical' 
		  AND status <> 'closed'
	`).Scan(&critical)
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
