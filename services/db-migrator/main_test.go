package main

import (
	"os"
	"strings"
	"testing"
)

func TestRequiredConfigFromEnvironment(t *testing.T) {
	const key = "TEST_MIGRATION_DSN"
	t.Setenv(key, " user:pass@tcp(db:3306)/schema ")
	got, err := requiredConfig(key)
	if err != nil {
		t.Fatal(err)
	}
	if got != "user:pass@tcp(db:3306)/schema" {
		t.Fatalf("got %q", got)
	}
}

func TestRequiredConfigFromFile(t *testing.T) {
	const key = "TEST_MIGRATION_DSN_FILE"
	file, err := os.CreateTemp(t.TempDir(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("u:p@tcp(db:3306)/schema\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(key+"_FILE", file.Name())
	got, err := requiredConfig(key)
	if err != nil || got != "u:p@tcp(db:3306)/schema" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestRequiredConfigMissing(t *testing.T) {
	const key = "TEST_MISSING_MIGRATION_DSN"
	t.Setenv(key, "")
	t.Setenv(key+"_FILE", "")
	_, err := requiredConfig(key)
	if err == nil || !strings.Contains(err.Error(), key) {
		t.Fatalf("unexpected err=%v", err)
	}
}
