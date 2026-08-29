package main

import (
	"errors"
	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/httpx"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthzHandler(t *testing.T) {
	w := httptest.NewRecorder()
	httpx.HealthHandler(serviceName)(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), serviceName) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestIssueValidation(t *testing.T) {
	for _, severity := range []string{severityLow, severityMedium, severityHigh, severityCritical} {
		if !validSeverity(severity) {
			t.Fatalf("expected severity %q to be valid", severity)
		}
	}
	for _, status := range []string{statusOpen, statusInProgress, statusResolved, statusVerified, statusClosed} {
		if !validIssueStatus(status) {
			t.Fatalf("expected status %q to be valid", status)
		}
	}
	if validSeverity("urgent") || validIssueStatus("done") {
		t.Fatal("unexpected validation acceptance")
	}
}

func TestValidOptionalDate(t *testing.T) {
	for _, value := range []string{"", "2026-08-29", " 2026-08-29 "} {
		if !validOptionalDate(value) {
			t.Fatalf("expected date %q to be valid", value)
		}
	}
	for _, value := range []string{"29-08-2026", "2026-02-30", "2026/08/29"} {
		if validOptionalDate(value) {
			t.Fatalf("expected date %q to be invalid", value)
		}
	}
}

func TestValidateAudit(t *testing.T) {
	valid := audit{Restaurant: "Tropical", Auditor: "Dita", Cleanliness: 90, SOP: 80, FoodQuality: 85}
	if got := validateAudit(valid); got != "" {
		t.Fatalf("valid audit rejected: %q", got)
	}
	cases := []audit{
		{Auditor: "Dita"},
		{Restaurant: "Tropical", Auditor: "Dita", Cleanliness: -1},
		{Restaurant: "Tropical", Auditor: "Dita", SOP: 101},
		{Restaurant: "Tropical", Auditor: "Dita", FoodQuality: 101},
	}
	for _, x := range cases {
		if got := validateAudit(x); got == "" {
			t.Fatalf("invalid audit accepted: %+v", x)
		}
	}
}

func TestIssueDefaultsAndValidation(t *testing.T) {
	x := issue{Title: "  issue  ", DueDate: "2026-09-01"}
	x.Title = strings.TrimSpace(x.Title)
	applyIssueDefaults(&x)
	if x.Severity != severityMedium || x.Status != statusOpen || validateIssue(x) != "" {
		t.Fatalf("unexpected issue after defaults: %+v", x)
	}

	x.DueDate = "not-a-date"
	if got := validateIssue(x); got != errInvalidDueDate {
		t.Fatalf("invalid due date result=%q", got)
	}
	x.DueDate = ""
	x.Severity = "urgent"
	if got := validateIssue(x); got != errInvalidSeverityStatus {
		t.Fatalf("invalid severity result=%q", got)
	}
}

func TestAuditHandlersRejectInvalidRequestsBeforeDatabaseAccess(t *testing.T) {
	a := &app{}

	w := httptest.NewRecorder()
	a.audits(w, httptest.NewRequest(http.MethodDelete, "/api/audits", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("audits delete status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.createAudit(w, httptest.NewRequest(http.MethodPost, "/api/audits", strings.NewReader(`{"restaurant":"","auditor":""}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid audit status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.issues(w, httptest.NewRequest(http.MethodDelete, "/api/issues", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("issues delete status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.createIssue(w, httptest.NewRequest(http.MethodPost, "/api/issues", strings.NewReader(`{"title":"","severity":"high"}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid issue status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.updateIssue(w, httptest.NewRequest(http.MethodPatch, "/api/issues", strings.NewReader(`{"id":1,"status":"open","due_date":"bad"}`)))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), errInvalidDueDate) {
		t.Fatalf("invalid update status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestAuditDatabasePaths(t *testing.T) {
	now := time.Date(2026, 8, 29, 9, 30, 0, 0, time.UTC)
	db, script := openTestDB(t,
		execStep("CREATE TABLE IF NOT EXISTS audits", 0, 0),
		execStep("CREATE TABLE IF NOT EXISTS issues", 0, 0),
		execStep("ALTER TABLE issues ADD COLUMN due_date", 0, 0),
		execStep("ALTER TABLE issues ADD COLUMN corrective_action", 0, 0),
		execStep("ALTER TABLE issues ADD COLUMN updated_at", 0, 0),
		execStep("ALTER TABLE issues ADD INDEX idx_issue_due_date", 0, 0),
		queryStep("FROM audits ORDER BY id DESC", []string{"id", "restaurant", "auditor", "cleanliness", "sop", "food_quality", "score", "notes", "created_at"},
			row(int64(1), "Tropical", "Dita", int64(90), int64(80), int64(85), float64(85), "ok", now)),
		execStep("INSERT INTO audits", 2, 1),
		queryStep("FROM issues", []string{"id", "audit_id", "title", "severity", "status", "assigned_to", "due_date", "corrective_action", "created_at", "updated_at"},
			row(int64(5), int64(1), "Fridge", severityHigh, statusOpen, "Ops", "2026-09-01", "repair", now, now)),
		execStep("INSERT INTO issues", 6, 1),
		execStep("UPDATE issues SET status", 0, 1),
		queryStep("AVG(score)", []string{"avg"}, row(float64(88.5))),
		queryStep("status <> 'closed'", []string{"count"}, row(int64(3))),
		queryStep("due_date < CURDATE()", []string{"count"}, row(int64(1))),
		queryStep("severity = 'critical'", []string{"count"}, row(int64(2))),
	)
	a := &app{db: db}

	if err := a.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	w := httptest.NewRecorder()
	a.audits(w, httptest.NewRequest(http.MethodGet, "/api/audits", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"score":85`) {
		t.Fatalf("audits status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.audits(w, httptest.NewRequest(http.MethodPost, "/api/audits", strings.NewReader(`{"restaurant":" Tropical ","auditor":" Dita ","cleanliness":90,"sop":80,"food_quality":85,"notes":" ok "}`)))
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":2`) {
		t.Fatalf("create audit status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.issues(w, httptest.NewRequest(http.MethodGet, "/api/issues", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"title":"Fridge"`) {
		t.Fatalf("issues status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.issues(w, httptest.NewRequest(http.MethodPost, "/api/issues", strings.NewReader(`{"audit_id":1,"title":" Fix sink ","severity":"high","assigned_to":" Ops ","due_date":"2026-09-02","corrective_action":" Repair "}`)))
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":6`) {
		t.Fatalf("create issue status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.issues(w, httptest.NewRequest(http.MethodPatch, "/api/issues", strings.NewReader(`{"id":6,"status":"resolved","assigned_to":" QA ","due_date":"2026-09-03","corrective_action":" Done "}`)))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"resolved"`) {
		t.Fatalf("update issue status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"audit_score":88.5`) || !strings.Contains(w.Body.String(), `"critical_issues":2`) {
		t.Fatalf("summary status=%d body=%q", w.Code, w.Body.String())
	}
	script.assertDone(t)
}

func TestAuditDatabaseErrorsAreHandled(t *testing.T) {
	boom := errors.New("db unavailable")
	db, script := openTestDB(t,
		execErrorStep("CREATE TABLE IF NOT EXISTS audits", boom),
		queryErrorStep("FROM audits ORDER BY id DESC", boom),
		execErrorStep("INSERT INTO audits", boom),
		queryErrorStep("AVG(score)", boom),
	)
	a := &app{db: db}

	if err := a.migrate(); err == nil {
		t.Fatal("expected migration error")
	}

	w := httptest.NewRecorder()
	a.getAudits(w, httptest.NewRequest(http.MethodGet, "/api/audits", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("get audits error status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.createAudit(w, httptest.NewRequest(http.MethodPost, "/api/audits", strings.NewReader(`{"restaurant":"Tropical","auditor":"Dita","cleanliness":90,"sop":90,"food_quality":90}`)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("create audit error status=%d", w.Code)
	}

	w = httptest.NewRecorder()
	a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("summary error status=%d", w.Code)
	}
	script.assertDone(t)
}

func TestAuditSummaryRemainingBranches(t *testing.T) {
	t.Run("rejects unsupported method", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&app{}).summary(w, httptest.NewRequest(http.MethodPost, "/internal/summary", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("open issues error", func(t *testing.T) {
		boom := errors.New("open issues failed")
		db, script := openTestDB(t,
			queryStep("AVG(score)", []string{"score"}, row(float64(90))),
			queryErrorStep("status <> 'closed'", boom),
		)
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("overdue issues error", func(t *testing.T) {
		boom := errors.New("overdue issues failed")
		db, script := openTestDB(t,
			queryStep("AVG(score)", []string{"score"}, row(float64(90))),
			queryStep("status <> 'closed'", []string{"count"}, row(int64(3))),
			queryErrorStep("due_date < CURDATE()", boom),
		)
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})

	t.Run("critical issues error", func(t *testing.T) {
		boom := errors.New("critical issues failed")
		db, script := openTestDB(t,
			queryStep("AVG(score)", []string{"score"}, row(float64(90))),
			queryStep("status <> 'closed'", []string{"count"}, row(int64(3))),
			queryStep("due_date < CURDATE()", []string{"count"}, row(int64(1))),
			queryErrorStep("severity = 'critical'", boom),
		)
		a := &app{db: db}
		w := httptest.NewRecorder()
		a.summary(w, httptest.NewRequest(http.MethodGet, "/internal/summary", nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		script.assertDone(t)
	})
}

func TestAuditIssueDatabaseErrorsAreHandled(t *testing.T) {
	boom := errors.New("issue database failure")
	db, script := openTestDB(t,
		queryErrorStep("FROM issues ORDER BY id DESC", boom),
		execErrorStep("INSERT INTO issues", boom),
		execErrorStep("UPDATE issues SET", boom),
	)
	a := &app{db: db}

	w := httptest.NewRecorder()
	a.getIssues(w, httptest.NewRequest(http.MethodGet, "/api/issues", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("get issues status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.createIssue(w, httptest.NewRequest(http.MethodPost, "/api/issues", strings.NewReader(`{"audit_id":1,"title":"Fix sink","severity":"high","assigned_to":"Ops","due_date":"2026-09-02","corrective_action":"Repair"}`)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("create issue status=%d body=%q", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	a.updateIssue(w, httptest.NewRequest(http.MethodPatch, "/api/issues", strings.NewReader(`{"id":6,"status":"resolved","assigned_to":"QA","due_date":"2026-09-03","corrective_action":"Done"}`)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("update issue status=%d body=%q", w.Code, w.Body.String())
	}
	script.assertDone(t)
}
