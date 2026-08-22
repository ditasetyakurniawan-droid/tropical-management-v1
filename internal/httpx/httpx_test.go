package httpx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvUsesFileBeforeEnvironment(t *testing.T) {
	key := "HTTPX_TEST_SECRET"
	secretFile := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secretFile, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(key, "from-env")
	t.Setenv(key+"_FILE", secretFile)

	if got := Env(key, "fallback"); got != "from-file" {
		t.Fatalf("Env() = %q, want %q", got, "from-file")
	}
}

func TestEnvUsesEnvironmentWhenFileIsUnset(t *testing.T) {
	key := "HTTPX_TEST_ENV"
	t.Setenv(key, " from-env ")
	t.Setenv(key+"_FILE", "")

	if got := Env(key, "fallback"); got != "from-env" {
		t.Fatalf("Env() = %q, want %q", got, "from-env")
	}
}

func TestEnvUsesFallbackWhenNoConfigurationExists(t *testing.T) {
	key := "HTTPX_TEST_FALLBACK"
	t.Setenv(key, "")
	t.Setenv(key+"_FILE", "")

	if got := Env(key, "fallback"); got != "fallback" {
		t.Fatalf("Env() = %q, want %q", got, "fallback")
	}
}

func TestEnvPanicsWhenConfiguredFileCannotBeRead(t *testing.T) {
	key := "HTTPX_TEST_MISSING_FILE"
	t.Setenv(key, "from-env")
	t.Setenv(key+"_FILE", filepath.Join(t.TempDir(), "missing"))

	defer func() {
		if recover() == nil {
			t.Fatal("Env() did not panic for an unreadable configured file")
		}
	}()

	_ = Env(key, "fallback")
}

func TestEnvPanicsWhenConfiguredFileIsEmpty(t *testing.T) {
	key := "HTTPX_TEST_EMPTY_FILE"
	secretFile := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secretFile, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(key+"_FILE", secretFile)

	defer func() {
		if recover() == nil {
			t.Fatal("Env() did not panic for an empty configured file")
		}
	}()

	_ = Env(key, "fallback")
}
