package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// JSON keeps every service response consistent and avoids repeating headers.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// DecodeJSON rejects unknown fields so API mistakes fail fast during development.
func DecodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func Env(key, fallback string) string {
	// Prefer file-based configuration when KEY_FILE is set. This is intended
	// for secret injectors such as HashiCorp Vault Agent, Docker secrets, and
	// Kubernetes-mounted secrets. If a file was explicitly configured, fail
	// closed when it cannot be read instead of silently using a fallback.
	if file := strings.TrimSpace(os.Getenv(key + "_FILE")); file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			panic(fmt.Sprintf("httpx.Env: read %s_FILE %q: %v", key, file, err))
		}

		value := strings.TrimSpace(string(data))
		if value == "" {
			panic(fmt.Sprintf("httpx.Env: %s_FILE %q is empty", key, file))
		}

		return value
	}

	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func BearerToken(r *http.Request) (string, error) {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("missing bearer token")
	}
	return strings.TrimSpace(parts[1]), nil
}
