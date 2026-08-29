package logx

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeServiceName(t *testing.T) {
	tests := map[string]string{
		"auth-service":      "auth-service",
		" chat/service ":    "chat-service",
		"../../bad service": "bad-service",
		"":                  "",
	}
	for input, want := range tests {
		if got := sanitizeServiceName(input); got != want {
			t.Fatalf("sanitizeServiceName(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestRotatingWriterRotatesAndRetainsBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	w, err := newRotatingWriter(path, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if _, err := w.Write([]byte("12345678")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("ABCDEFGH")); err != nil {
		t.Fatal(err)
	}

	active, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatal(err)
	}
	if string(active) != "ABCDEFGH" || string(backup) != "12345678" {
		t.Fatalf("unexpected rotation: active=%q backup=%q", active, backup)
	}
}

func TestConfigureWritesServiceLog(t *testing.T) {
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	oldPrefix := log.Prefix()
	t.Cleanup(func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
		log.SetPrefix(oldPrefix)
	})

	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	t.Setenv("LOG_STDOUT", "false")
	closeLog, err := Configure("test-service")
	if err != nil {
		t.Fatal(err)
	}
	log.Print("trace-me")
	if err := closeLog(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "test-service.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "service=test-service") || !strings.Contains(string(content), "trace-me") {
		t.Fatalf("log file does not contain expected fields: %q", content)
	}
}

func TestEnvParsersFallbackOnInvalidValues(t *testing.T) {
	t.Setenv("LOG_TEST_INT", "invalid")
	if got := envInt("LOG_TEST_INT", 5, 1, 10); got != 5 {
		t.Fatalf("envInt fallback=%d, want 5", got)
	}
	t.Setenv("LOG_TEST_BOOL", "invalid")
	if got := envBool("LOG_TEST_BOOL", true); !got {
		t.Fatal("envBool should use fallback")
	}
}

func TestConfigureValidationAndFilesystemErrors(t *testing.T) {
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	oldPrefix := log.Prefix()
	t.Cleanup(func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
		log.SetPrefix(oldPrefix)
	})

	if closeLog, err := Configure("   "); err == nil || closeLog != nil {
		t.Fatalf("blank service should fail with nil cleanup: err=%v", err)
	}

	t.Run("log directory cannot be created", func(t *testing.T) {
		root := t.TempDir()
		blocked := filepath.Join(root, "not-a-directory")
		if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("LOG_DIR", blocked)
		if closeLog, err := Configure("service"); err == nil || closeLog != nil {
			t.Fatalf("expected mkdir failure with nil cleanup: err=%v", err)
		}
	})

	t.Run("log file cannot be opened", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "service.log"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("LOG_DIR", dir)
		if closeLog, err := Configure("service"); err == nil || closeLog != nil {
			t.Fatalf("expected open failure with nil cleanup: err=%v", err)
		}
	})

	t.Run("stdout mirroring branch", func(t *testing.T) {
		t.Setenv("LOG_DIR", t.TempDir())
		t.Setenv("LOG_STDOUT", "true")
		closeLog, err := Configure("stdout-service")
		if err != nil {
			t.Fatal(err)
		}
		if err := closeLog(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestEnvParsersValidAndEmptyValues(t *testing.T) {
	t.Setenv("LOG_TEST_INT_EMPTY", "")
	if got := envInt("LOG_TEST_INT_EMPTY", 7, 1, 10); got != 7 {
		t.Fatalf("empty envInt=%d, want 7", got)
	}
	t.Setenv("LOG_TEST_INT_VALID", " 8 ")
	if got := envInt("LOG_TEST_INT_VALID", 7, 1, 10); got != 8 {
		t.Fatalf("valid envInt=%d, want 8", got)
	}

	t.Setenv("LOG_TEST_BOOL_EMPTY", "")
	if got := envBool("LOG_TEST_BOOL_EMPTY", false); got {
		t.Fatal("empty envBool should use false fallback")
	}
	t.Setenv("LOG_TEST_BOOL_VALID", " true ")
	if got := envBool("LOG_TEST_BOOL_VALID", false); !got {
		t.Fatal("valid envBool should parse true")
	}
}

func TestRotatingWriterValidationAndClosedBehavior(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	if _, err := newRotatingWriter(path, 0, 1); err == nil {
		t.Fatal("expected maxBytes validation error")
	}
	if _, err := newRotatingWriter(path, 1, 0); err == nil {
		t.Fatal("expected maxBackups validation error")
	}

	dirPath := filepath.Join(t.TempDir(), "service.log")
	if err := os.Mkdir(dirPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := newRotatingWriter(dirPath, 10, 1); err == nil {
		t.Fatal("expected open error for directory path")
	}

	w, err := newRotatingWriter(path, 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second close should be harmless: %v", err)
	}
	if _, err := w.Write([]byte("closed")); err == nil {
		t.Fatal("write after close should fail")
	}
}

func TestRotatingWriterReportsBackupRemovalFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	w, err := newRotatingWriter(path, 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	if _, err := w.Write([]byte("12345678")); err != nil {
		t.Fatal(err)
	}

	backupDir := path + ".1"
	if err := os.Mkdir(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := w.Write([]byte("ABCDEFGH")); err == nil || !strings.Contains(err.Error(), "remove oldest log backup") {
		t.Fatalf("expected backup removal error, got %v", err)
	}
}

func TestConfigureBestEffort(t *testing.T) {
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	oldPrefix := log.Prefix()
	t.Cleanup(func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
		log.SetPrefix(oldPrefix)
	})

	t.Run("returns real cleanup when configuration succeeds", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("LOG_DIR", dir)
		t.Setenv("LOG_STDOUT", "false")
		cleanup := ConfigureBestEffort("best-effort-service")
		if cleanup == nil {
			t.Fatal("cleanup must never be nil")
		}
		log.Print("best-effort-line")
		cleanup()
		content, err := os.ReadFile(filepath.Join(dir, "best-effort-service.log"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "best-effort-line") {
			t.Fatalf("log content=%q", content)
		}
	})

	t.Run("falls back when file logging cannot initialize", func(t *testing.T) {
		root := t.TempDir()
		blocked := filepath.Join(root, "not-a-directory")
		if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("LOG_DIR", blocked)
		cleanup := ConfigureBestEffort("fallback-service")
		if cleanup == nil {
			t.Fatal("fallback cleanup must never be nil")
		}
		cleanup()
	})
}
