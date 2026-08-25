package dbx

import (
	"os"
	"testing"
)

func TestEnvFallback(t *testing.T) {
	key := "TEST_DB_HOST_ENV"
	os.Unsetenv(key)

	val := os.Getenv(key)
	if val != "" {
		t.Errorf("expected empty string, got %s", val)
	}
}
