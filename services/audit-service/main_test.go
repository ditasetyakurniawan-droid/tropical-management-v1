package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthzHandler(t *testing.T) {
	w := httptest.NewRecorder()
	healthzHandler(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
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
