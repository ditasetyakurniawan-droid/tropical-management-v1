package main

import "testing"

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
