package httpx

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	fallbackValue = "fallback"
	fromFileValue = "from-file"
	fromEnvValue  = "from-env"
	fromEnvPadded = " from-env "
	envErrFormat  = "Env() = %q, want %q"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnv(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		envVal  string
		fileVal string // kosong berarti tidak membuat file
		want    string
	}{
		{
			name:    "file before environment",
			key:     "HTTPX_TEST_SECRET",
			envVal:  fromEnvValue,
			fileVal: fromFileValue + "\n",
			want:    fromFileValue,
		},
		{
			name:    "environment when file unset",
			key:     "HTTPX_TEST_ENV",
			envVal:  fromEnvPadded,
			fileVal: "",
			want:    fromEnvValue,
		},
		{
			name:    "fallback when no configuration",
			key:     "HTTPX_TEST_FALLBACK",
			envVal:  "",
			fileVal: "",
			want:    fallbackValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var secretFile string
			if tt.fileVal != "" {
				secretFile = writeTempFile(t, tt.fileVal)
			}

			t.Setenv(tt.key, tt.envVal)
			t.Setenv(tt.key+"_FILE", secretFile)

			if got := Env(tt.key, fallbackValue); got != tt.want {
				t.Fatalf(envErrFormat, got, tt.want)
			}
		})
	}
}

func TestEnvPanicsWhenConfiguredFileCannotBeRead(t *testing.T) {
	key := "HTTPX_TEST_MISSING_FILE"
	t.Setenv(key, fromEnvValue)
	t.Setenv(key+"_FILE", filepath.Join(t.TempDir(), "missing"))

	defer func() {
		if recover() == nil {
			t.Fatal("Env() did not panic for an unreadable configured file")
		}
	}()

	_ = Env(key, fallbackValue)
}

func TestEnvPanicsWhenConfiguredFileIsEmpty(t *testing.T) {
	key := "HTTPX_TEST_EMPTY_FILE"
	secretFile := writeTempFile(t, "\n")
	t.Setenv(key+"_FILE", secretFile)

	defer func() {
		if recover() == nil {
			t.Fatal("Env() did not panic for an empty configured file")
		}
	}()

	_ = Env(key, fallbackValue)
}
