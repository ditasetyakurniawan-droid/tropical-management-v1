package configx

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultEnvironment = "local"

// Environment identifies the runtime trust level used to decide whether local
// development fallbacks are allowed.
type Environment string

const (
	EnvironmentLocal       Environment = "local"
	EnvironmentDevelopment Environment = "development"
	EnvironmentTest        Environment = "test"
	EnvironmentTestApp     Environment = "test-app"
	EnvironmentStaging     Environment = "staging"
	EnvironmentProduction  Environment = "production"
)

// CurrentEnvironment returns APP_ENV after validation. APP_ENV intentionally
// defaults to local so CLI/unit-test workflows remain backwards compatible.
// Kubernetes overlays must set APP_ENV explicitly.
func CurrentEnvironment() Environment {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if raw == "" {
		raw = defaultEnvironment
	}

	env := Environment(raw)
	switch env {
	case EnvironmentLocal, EnvironmentDevelopment, EnvironmentTest, EnvironmentTestApp, EnvironmentStaging, EnvironmentProduction:
		return env
	default:
		panic(fmt.Sprintf("configx: unsupported APP_ENV %q", raw))
	}
}

// AllowsLocalFallback reports whether hard-coded local development defaults are
// permitted. Test/test-app/staging/production deliberately fail closed.
func AllowsLocalFallback() bool {
	switch CurrentEnvironment() {
	case EnvironmentLocal, EnvironmentDevelopment:
		return true
	default:
		return false
	}
}

// Value reads KEY_FILE before KEY, then returns fallback when neither is set.
// Explicitly configured files fail closed when unreadable or empty.
func Value(key, fallback string) string {
	if file := strings.TrimSpace(os.Getenv(key + "_FILE")); file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			panic(fmt.Sprintf("configx: read %s_FILE %q: %v", key, file, err))
		}
		value := strings.TrimSpace(string(data))
		if value == "" {
			panic(fmt.Sprintf("configx: %s_FILE %q is empty", key, file))
		}
		return value
	}

	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// Sensitive behaves like Value for local development, but non-local runtimes
// must provide the value through KEY or KEY_FILE. This prevents Kubernetes
// workloads from silently starting with development credentials.
func Sensitive(key, localFallback string) string {
	value := Value(key, "")
	if value != "" {
		return value
	}
	if AllowsLocalFallback() && strings.TrimSpace(localFallback) != "" {
		return strings.TrimSpace(localFallback)
	}
	panic(fmt.Sprintf("configx: %s or %s_FILE is required when APP_ENV=%s", key, key, CurrentEnvironment()))
}

// SensitiveSecret reads a sensitive value and enforces a minimum byte length.
func SensitiveSecret(key, localFallback string, minBytes int) string {
	value := Sensitive(key, localFallback)
	if len([]byte(value)) < minBytes {
		panic(fmt.Sprintf("configx: %s must be at least %d bytes", key, minBytes))
	}
	return value
}

// Int returns a positive integer configuration value. Empty values use the
// supplied default. Invalid or non-positive values fail fast during startup.
func Int(key string, defaultValue int) int {
	if defaultValue <= 0 {
		panic(fmt.Sprintf("configx: default for %s must be positive", key))
	}
	raw := Value(key, "")
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		panic(fmt.Sprintf("configx: %s must be a positive integer, got %q", key, raw))
	}
	return value
}

// Duration returns a positive time.Duration configuration value. Values use Go
// duration syntax such as 750ms, 3s, or 2m.
func Duration(key string, defaultValue time.Duration) time.Duration {
	if defaultValue <= 0 {
		panic(fmt.Sprintf("configx: default for %s must be positive", key))
	}
	raw := Value(key, "")
	if raw == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		panic(fmt.Sprintf("configx: %s must be a positive duration, got %q", key, raw))
	}
	return value
}
