package main

import (
	"database/sql"
	"log"
	"net/http"
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
	ID         int64     `json:"id"`
	AuditID    int64     `json:"audit_id"`
	Title      string    `json:"title"`
	Severity   string    `json:"severity"`
	Status     string    `json:"status"`
	AssignedTo string    `json:"assigned_to"`
	CreatedAt  time.Time `json:"created_at"`
}

func main() {
	db, err := dbx.Open(httpx.Env("AUDIT_DB_DSN", "tropical:tropical@tcp(mysql:3306)/tropical_audit?parseTime=true&charset=utf8mb4"))
	if err != nil { log.Fatal(err) }
	defer db.Close()
	a := &app{db: db}
	if err := a.migrate(); err != nil { log.Fatal(err) }
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { httpx.JSON(w, 200, map[string]string{"status":"ok","service":"audit-service"}) })
	mux.HandleFunc("/api/audits", a.audits)
	mux.HandleFunc("/api/issues", a.issues)
	mux.HandleFunc("/internal/summary", a.summary)
	log.Println("audit-service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (a *app) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS audits (id BIGINT PRIMARY KEY AUTO_INCREMENT, restaurant VARCHAR(150) NOT NULL, auditor VARCHAR(150) NOT NULL, cleanliness INT NOT NULL, sop INT NOT NULL, food_quality INT NOT NULL, score DECIMAL(5,2) NOT NULL, notes TEXT, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS issues (id BIGINT PRIMARY KEY AUTO_INCREMENT, audit_id BIGINT NULL, title VARCHAR(220) NOT NULL, severity VARCHAR(20) NOT NULL, status VARCHAR(30) NOT NULL DEFAULT 'open', assigned_to VARCHAR(150), created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, INDEX idx_issue_status(status)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, q := range queries { if _, err := a.db.Exec(q); err != nil { return err } }
	return nil
}

func (a *app) audits(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query("SELECT id,restaurant,auditor,cleanliness,sop,food_quality,score,notes,created_at FROM audits ORDER BY id DESC LIMIT 100")
		if err != nil { httpx.JSON(w,500,map[string]string{"error":err.Error()}); return }
		defer rows.Close(); result := []audit{}
		for rows.Next(){ var x audit; if rows.Scan(&x.ID,&x.Restaurant,&x.Auditor,&x.Cleanliness,&x.SOP,&x.FoodQuality,&x.Score,&x.Notes,&x.CreatedAt)==nil { result=append(result,x) } }
		httpx.JSON(w,200,result)
	case http.MethodPost:
		var x audit
		if err:=httpx.DecodeJSON(r,&x); err!=nil { httpx.JSON(w,400,map[string]string{"error":"invalid json"}); return }
		if x.Cleanliness<0||x.Cleanliness>100||x.SOP<0||x.SOP>100||x.FoodQuality<0||x.FoodQuality>100 { httpx.JSON(w,400,map[string]string{"error":"checklist scores must be 0-100"}); return }
		x.Score=float64(x.Cleanliness+x.SOP+x.FoodQuality)/3
		res,err:=a.db.Exec("INSERT INTO audits(restaurant,auditor,cleanliness,sop,food_quality,score,notes) VALUES(?,?,?,?,?,?,?)",x.Restaurant,x.Auditor,x.Cleanliness,x.SOP,x.FoodQuality,x.Score,x.Notes)
		if err!=nil { httpx.JSON(w,500,map[string]string{"error":err.Error()}); return }; x.ID,_=res.LastInsertId(); x.CreatedAt=time.Now(); httpx.JSON(w,201,x)
	default: httpx.JSON(w,405,map[string]string{"error":"method not allowed"})
	}
}

func (a *app) issues(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows,err:=a.db.Query("SELECT id,COALESCE(audit_id,0),title,severity,status,COALESCE(assigned_to,''),created_at FROM issues ORDER BY id DESC LIMIT 100")
		if err!=nil { httpx.JSON(w,500,map[string]string{"error":err.Error()}); return }; defer rows.Close(); result:=[]issue{}
		for rows.Next(){ var x issue; if rows.Scan(&x.ID,&x.AuditID,&x.Title,&x.Severity,&x.Status,&x.AssignedTo,&x.CreatedAt)==nil { result=append(result,x) } }; httpx.JSON(w,200,result)
	case http.MethodPost:
		var x issue; if err:=httpx.DecodeJSON(r,&x); err!=nil { httpx.JSON(w,400,map[string]string{"error":"invalid json"}); return }
		if x.Severity=="" { x.Severity="medium" }; if x.Status=="" { x.Status="open" }
		res,err:=a.db.Exec("INSERT INTO issues(audit_id,title,severity,status,assigned_to) VALUES(NULLIF(?,0),?,?,?,?)",x.AuditID,x.Title,x.Severity,x.Status,x.AssignedTo)
		if err!=nil { httpx.JSON(w,500,map[string]string{"error":err.Error()}); return }; x.ID,_=res.LastInsertId(); x.CreatedAt=time.Now(); httpx.JSON(w,201,x)
	case http.MethodPatch:
		var x issue; if err:=httpx.DecodeJSON(r,&x); err!=nil || x.ID==0 { httpx.JSON(w,400,map[string]string{"error":"id and valid json required"}); return }
		_,err:=a.db.Exec("UPDATE issues SET status=?,assigned_to=? WHERE id=?",x.Status,x.AssignedTo,x.ID); if err!=nil { httpx.JSON(w,500,map[string]string{"error":err.Error()}); return }; httpx.JSON(w,200,map[string]any{"id":x.ID,"status":x.Status,"assigned_to":x.AssignedTo})
	default: httpx.JSON(w,405,map[string]string{"error":"method not allowed"})
	}
}

func (a *app) summary(w http.ResponseWriter,_ *http.Request){ var score float64; var open int; _=a.db.QueryRow("SELECT COALESCE(AVG(score),0) FROM audits WHERE created_at>=DATE_SUB(NOW(),INTERVAL 30 DAY)").Scan(&score); _=a.db.QueryRow("SELECT COUNT(*) FROM issues WHERE status<>'closed'").Scan(&open); httpx.JSON(w,200,map[string]any{"audit_score":score,"open_issues":open}) }
