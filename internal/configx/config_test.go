package configx

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSensitiveAllowsLocalFallback(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	if got := Sensitive("CONFIGX_LOCAL_SECRET", "local-secret"); got != "local-secret" {
		t.Fatalf("Sensitive()=%q", got)
	}
}

func TestSensitiveFailsClosedOutsideLocal(t *testing.T) {
	t.Setenv("APP_ENV", "test-app")
	defer func() {
		if recover() == nil {
			t.Fatal("expected missing sensitive config to panic")
		}
	}()
	_ = Sensitive("CONFIGX_REQUIRED_SECRET", "local-secret")
}

func TestSensitiveUsesFileOutsideLocal(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	path := filepath.Join(t.TempDir(), "secret")
	if err := osWriteFile(path, []byte("from-file\n")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIGX_FILE_SECRET_FILE", path)
	if got := Sensitive("CONFIGX_FILE_SECRET", "local-secret"); got != "from-file" {
		t.Fatalf("Sensitive()=%q", got)
	}
}

func TestIntAndDuration(t *testing.T) {
	t.Setenv("CONFIGX_INT", "7")
	if got := Int("CONFIGX_INT", 3); got != 7 {
		t.Fatalf("Int()=%d", got)
	}
	t.Setenv("CONFIGX_DURATION", "1500ms")
	if got := Duration("CONFIGX_DURATION", time.Second); got != 1500*time.Millisecond {
		t.Fatalf("Duration()=%s", got)
	}
}

func TestInvalidEnvironmentPanics(t *testing.T) {
	t.Setenv("APP_ENV", "banana")
	defer func() {
		if recover() == nil {
			t.Fatal("expected invalid APP_ENV to panic")
		}
	}()
	_ = CurrentEnvironment()
}

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
