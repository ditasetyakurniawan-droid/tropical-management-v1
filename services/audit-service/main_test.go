package main

import "testing"

func TestIssueValidation(t *testing.T) {
	for _, severity := range []string{"low", "medium", "high", "critical"} {
		if !validSeverity(severity) {
			t.Fatalf("expected severity %q to be valid", severity)
		}
	}
	for _, status := range []string{"open", "in_progress", "resolved", "verified", "closed"} {
		if !validIssueStatus(status) {
			t.Fatalf("expected status %q to be valid", status)
		}
	}
	if validSeverity("urgent") || validIssueStatus("done") {
		t.Fatal("unexpected validation acceptance")
	}
}
