package main

import (
	"testing"
	"time"
)

func TestValidRole(t *testing.T) {
	valid := []string{"admin", "auditor", "staff"}
	for _, role := range valid {
		if !validRole(role) {
			t.Fatalf("expected role %q to be valid", role)
		}
	}
	invalid := []string{"", "manager", "ADMIN", "owner"}
	for _, role := range invalid {
		if validRole(role) {
			t.Fatalf("expected role %q to be invalid", role)
		}
	}
}

func TestSessionTTL(t *testing.T) {
	if sessionTTL != 30*time.Minute {
		t.Fatalf("expected 30 minute session TTL, got %s", sessionTTL)
	}
}
