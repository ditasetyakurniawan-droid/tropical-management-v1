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
	db, err := dbx.Open(httpx.Env("AUDIT_DB_DSN", "tropical:tropical@tcp(mysql:3306)/tropical_audit?parseTime=true&charset=utf8mb4"))
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
		httpx.JSON(w, 200, map[string]string{"status": "ok", "service": "audit-service"})
	})
	mux.HandleFunc("/api/audits", a.audits)
	mux.HandleFunc("/api/issues", a.issues)
	mux.HandleFunc("/internal/summary", a.summary)
	log.Println("audit-service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (a *app) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS audits (id BIGINT PRIMARY KEY AUTO_INCREMENT, restaurant VARCHAR(150) NOT NULL, auditor VARCHAR(150) NOT NULL, cleanliness INT NOT NULL, sop INT NOT NULL, food_quality INT NOT NULL, score DECIMAL(5,2) NOT NULL, notes TEXT, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS issues (id BIGINT PRIMARY KEY AUTO_INCREMENT, audit_id BIGINT NULL, title VARCHAR(220) NOT NULL, severity VARCHAR(20) NOT NULL, status VARCHAR(30) NOT NULL DEFAULT 'open', assigned_to VARCHAR(150), due_date DATE NULL, corrective_action TEXT, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, INDEX idx_issue_status(status), INDEX idx_issue_due_date(due_date)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, q := range queries {
		if _, err := a.db.Exec(q); err != nil {
			return err
		}
	}
	// Phase-2 databases already have issues. These ALTERs are restart-safe because
	// duplicate-column/index errors are intentionally ignored during local migration.
	_, _ = a.db.Exec(`ALTER TABLE issues ADD COLUMN due_date DATE NULL`)
	_, _ = a.db.Exec(`ALTER TABLE issues ADD COLUMN corrective_action TEXT`)
	_, _ = a.db.Exec(`ALTER TABLE issues ADD COLUMN updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP`)
	_, _ = a.db.Exec(`ALTER TABLE issues ADD INDEX idx_issue_due_date(due_date)`)
	return nil
}

func validSeverity(v string) bool {
	switch v {
	case "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
}

func validIssueStatus(v string) bool {
	switch v {
	case "open", "in_progress", "resolved", "verified", "closed":
		return true
	default:
		return false
	}
}

func (a *app) audits(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query("SELECT id,restaurant,auditor,cleanliness,sop,food_quality,score,COALESCE(notes,''),created_at FROM audits ORDER BY id DESC LIMIT 100")
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		result := []audit{}
		for rows.Next() {
			var x audit
			if rows.Scan(&x.ID, &x.Restaurant, &x.Auditor, &x.Cleanliness, &x.SOP, &x.FoodQuality, &x.Score, &x.Notes, &x.CreatedAt) == nil {
				result = append(result, x)
			}
		}
		httpx.JSON(w, 200, result)

	case http.MethodPost:
		var x audit
		if err := httpx.DecodeJSON(r, &x); err != nil {
			httpx.JSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		x.Restaurant = strings.TrimSpace(x.Restaurant)
		x.Auditor = strings.TrimSpace(x.Auditor)
		if x.Restaurant == "" || x.Auditor == "" {
			httpx.JSON(w, 400, map[string]string{"error": "restaurant and auditor are required"})
			return
		}
		if x.Cleanliness < 0 || x.Cleanliness > 100 || x.SOP < 0 || x.SOP > 100 || x.FoodQuality < 0 || x.FoodQuality > 100 {
			httpx.JSON(w, 400, map[string]string{"error": "checklist scores must be 0-100"})
			return
		}
		x.Score = float64(x.Cleanliness+x.SOP+x.FoodQuality) / 3
		res, err := a.db.Exec("INSERT INTO audits(restaurant,auditor,cleanliness,sop,food_quality,score,notes) VALUES(?,?,?,?,?,?,?)", x.Restaurant, x.Auditor, x.Cleanliness, x.SOP, x.FoodQuality, x.Score, strings.TrimSpace(x.Notes))
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		x.ID, _ = res.LastInsertId()
		x.CreatedAt = time.Now()
		httpx.JSON(w, 201, x)

	default:
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (a *app) issues(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,COALESCE(audit_id,0),title,severity,status,COALESCE(assigned_to,''),COALESCE(DATE_FORMAT(due_date,'%Y-%m-%d'),''),COALESCE(corrective_action,''),created_at,updated_at FROM issues ORDER BY CASE severity WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END, id DESC LIMIT 200`)
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer rows.Close()
		result := []issue{}
		for rows.Next() {
			var x issue
			if rows.Scan(&x.ID, &x.AuditID, &x.Title, &x.Severity, &x.Status, &x.AssignedTo, &x.DueDate, &x.CorrectiveAction, &x.CreatedAt, &x.UpdatedAt) == nil {
				result = append(result, x)
			}
		}
		httpx.JSON(w, 200, result)

	case http.MethodPost:
		var x issue
		if err := httpx.DecodeJSON(r, &x); err != nil {
			httpx.JSON(w, 400, map[string]string{"error": "invalid json"})
			return
		}
		x.Title = strings.TrimSpace(x.Title)
		if x.Title == "" {
			httpx.JSON(w, 400, map[string]string{"error": "issue title is required"})
			return
		}
		if x.Severity == "" {
			x.Severity = "medium"
		}
		if x.Status == "" {
			x.Status = "open"
		}
		if !validSeverity(x.Severity) || !validIssueStatus(x.Status) {
			httpx.JSON(w, 400, map[string]string{"error": "invalid severity or status"})
			return
		}
		res, err := a.db.Exec("INSERT INTO issues(audit_id,title,severity,status,assigned_to,due_date,corrective_action) VALUES(NULLIF(?,0),?,?,?,?,NULLIF(?,''),?)", x.AuditID, x.Title, x.Severity, x.Status, strings.TrimSpace(x.AssignedTo), x.DueDate, strings.TrimSpace(x.CorrectiveAction))
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		x.ID, _ = res.LastInsertId()
		x.CreatedAt = time.Now()
		x.UpdatedAt = x.CreatedAt
		httpx.JSON(w, 201, x)

	case http.MethodPatch:
		var x issue
		if err := httpx.DecodeJSON(r, &x); err != nil || x.ID == 0 {
			httpx.JSON(w, 400, map[string]string{"error": "id and valid json required"})
			return
		}
		if !validIssueStatus(x.Status) {
			httpx.JSON(w, 400, map[string]string{"error": "invalid issue status"})
			return
		}
		_, err := a.db.Exec("UPDATE issues SET status=?,assigned_to=?,due_date=NULLIF(?,''),corrective_action=? WHERE id=?", x.Status, strings.TrimSpace(x.AssignedTo), x.DueDate, strings.TrimSpace(x.CorrectiveAction), x.ID)
		if err != nil {
			httpx.JSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		httpx.JSON(w, 200, map[string]any{"id": x.ID, "status": x.Status, "assigned_to": strings.TrimSpace(x.AssignedTo), "due_date": x.DueDate, "corrective_action": strings.TrimSpace(x.CorrectiveAction)})

	default:
		httpx.JSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func (a *app) summary(w http.ResponseWriter, _ *http.Request) {
	var score float64
	var open, overdue, critical int
	_ = a.db.QueryRow("SELECT COALESCE(AVG(score),0) FROM audits WHERE created_at>=DATE_SUB(NOW(),INTERVAL 30 DAY)").Scan(&score)
	_ = a.db.QueryRow("SELECT COUNT(*) FROM issues WHERE status<>'closed'").Scan(&open)
	_ = a.db.QueryRow("SELECT COUNT(*) FROM issues WHERE due_date<CURDATE() AND status NOT IN ('closed','verified')").Scan(&overdue)
	_ = a.db.QueryRow("SELECT COUNT(*) FROM issues WHERE severity='critical' AND status<>'closed'").Scan(&critical)
	httpx.JSON(w, 200, map[string]any{"audit_score": score, "open_issues": open, "overdue_issues": overdue, "critical_issues": critical})
}
