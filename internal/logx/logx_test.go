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
