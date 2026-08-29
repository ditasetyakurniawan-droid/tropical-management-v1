package httpx

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusCreated, map[string]string{"status": "ok"})
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d, want %d", w.Code, http.StatusCreated)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type=%q", got)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options=%q", got)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestInternalErrorDoesNotLeakDetails(t *testing.T) {
	oldWriter := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(oldWriter) })

	w := httptest.NewRecorder()
	InternalError(w, errors.New("mysql password=secret"))
	if strings.Contains(w.Body.String(), "secret") || !strings.Contains(w.Body.String(), internalErrorMsg) {
		t.Fatalf("unexpected body=%q", w.Body.String())
	}
}

func TestDecodeJSON(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		var body struct {
			Name string `json:"name"`
		}
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Tropical"}`))
		if err := DecodeJSON(r, &body); err != nil {
			t.Fatal(err)
		}
		if body.Name != "Tropical" {
			t.Fatalf("name=%q", body.Name)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		var body struct {
			Name string `json:"name"`
		}
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Tropical","unknown":true}`))
		if err := DecodeJSON(r, &body); err == nil {
			t.Fatal("expected unknown field to fail")
		}
	})

	t.Run("trailing json", func(t *testing.T) {
		var body map[string]any
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true} {"second":true}`))
		if err := DecodeJSON(r, &body); err == nil {
			t.Fatal("expected trailing JSON to fail")
		}
	})

	t.Run("empty", func(t *testing.T) {
		var body map[string]any
		r := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
		if err := DecodeJSON(r, &body); err == nil {
			t.Fatal("expected empty body to fail")
		}
	})
}

func TestEnv(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		envVal  string
		fileVal string
		want    string
	}{
		{name: "file before environment", key: "HTTPX_TEST_SECRET", envVal: fromEnvValue, fileVal: fromFileValue + "\n", want: fromFileValue},
		{name: "environment when file unset", key: "HTTPX_TEST_ENV", envVal: fromEnvPadded, want: fromEnvValue},
		{name: "fallback when no configuration", key: "HTTPX_TEST_FALLBACK", want: fallbackValue},
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

func TestSecretEnforcesMinimumLength(t *testing.T) {
	t.Setenv("HTTPX_TEST_SECRET_LENGTH", "01234567890123456789012345678901")
	if got := Secret("HTTPX_TEST_SECRET_LENGTH", "", 32); len(got) != 32 {
		t.Fatalf("secret length=%d", len(got))
	}

	t.Setenv("HTTPX_TEST_SECRET_SHORT", "short")
	defer func() {
		if recover() == nil {
			t.Fatal("short secret should panic")
		}
	}()
	_ = Secret("HTTPX_TEST_SECRET_SHORT", "", 32)
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
		ok     bool
	}{
		{name: "valid", header: "Bearer abc.def", want: "abc.def", ok: true},
		{name: "case insensitive scheme", header: "bearer token", want: "token", ok: true},
		{name: "missing", ok: false},
		{name: "wrong scheme", header: "Basic abc", ok: false},
		{name: "empty token", header: "Bearer   ", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Authorization", tt.header)
			got, err := BearerToken(r)
			if (err == nil) != tt.ok || got != tt.want {
				t.Fatalf("BearerToken()=(%q,%v), want (%q, ok=%v)", got, err, tt.want, tt.ok)
			}
		})
	}
	if _, err := BearerToken(nil); err == nil {
		t.Fatal("nil request should fail")
	}
}

func TestRequestLoggerPropagatesRequestIDAndRecoversPanic(t *testing.T) {
	oldWriter := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(oldWriter) })

	t.Run("propagates valid request id", func(t *testing.T) {
		h := RequestLogger("test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := RequestID(r.Context()); got != "trace-123" {
				t.Fatalf("request id in context=%q", got)
			}
			if got := r.Header.Get("X-Request-ID"); got != "trace-123" {
				t.Fatalf("request id header=%q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		r.Header.Set("X-Request-ID", "trace-123")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusNoContent || w.Header().Get("X-Request-ID") != "trace-123" {
			t.Fatalf("status=%d response request id=%q", w.Code, w.Header().Get("X-Request-ID"))
		}
	})

	t.Run("replaces invalid request id", func(t *testing.T) {
		h := RequestLogger("test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Request-ID", "bad\nvalue")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := w.Header().Get("X-Request-ID"); got == "" || got == "bad\nvalue" {
			t.Fatalf("generated request id=%q", got)
		}
	})

	t.Run("recovers panic", func(t *testing.T) {
		h := RequestLogger("test", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), internalErrorMsg) {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})
}

func TestIncomingRequestID(t *testing.T) {
	if got := incomingRequestID("abc-123:xyz"); got != "abc-123:xyz" {
		t.Fatalf("valid id rejected: %q", got)
	}
	for _, invalid := range []string{"contains space", "../bad/request", strings.Repeat("a", 129)} {
		if got := incomingRequestID(invalid); got != "" {
			t.Fatalf("invalid id accepted: %q", got)
		}
	}
}

func TestNewServerDefaults(t *testing.T) {
	s := NewServer(":8080", http.NotFoundHandler())
	if s.Addr != ":8080" || s.ReadHeaderTimeout != 5*time.Second || s.ReadTimeout != 15*time.Second || s.IdleTimeout != 60*time.Second {
		t.Fatalf("unexpected server defaults: %+v", s)
	}
}

func TestRequestIDNilContextAndNilHandler(t *testing.T) {
	if got := RequestID(nil); got != "" {
		t.Fatalf("RequestID(nil)=%q", got)
	}

	oldWriter := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(oldWriter) })

	h := RequestLogger("test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if w.Code != http.StatusNotFound || w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status=%d request-id=%q", w.Code, w.Header().Get("X-Request-ID"))
	}
}

func TestStatusWriterOptionalInterfacesAndAccounting(t *testing.T) {
	t.Run("flush and unwrap", func(t *testing.T) {
		base := httptest.NewRecorder()
		w := &statusWriter{ResponseWriter: base}
		w.Flush()
		if !w.wroteHeader || w.status != http.StatusOK || !base.Flushed {
			t.Fatalf("wroteHeader=%v status=%d flushed=%v", w.wroteHeader, w.status, base.Flushed)
		}
		if w.Unwrap() != base {
			t.Fatal("Unwrap did not return the underlying writer")
		}
	})

	t.Run("unsupported hijack and push", func(t *testing.T) {
		w := &statusWriter{ResponseWriter: httptest.NewRecorder()}
		if _, _, err := w.Hijack(); err == nil {
			t.Fatal("Hijack should fail for a writer without http.Hijacker")
		}
		if err := w.Push("/asset.js", nil); !errors.Is(err, http.ErrNotSupported) {
			t.Fatalf("Push error=%v", err)
		}
	})

	t.Run("read from fallback counts bytes", func(t *testing.T) {
		base := httptest.NewRecorder()
		w := &statusWriter{ResponseWriter: base}
		n, err := w.ReadFrom(strings.NewReader("hello"))
		if err != nil || n != 5 || w.bytes != 5 || w.status != http.StatusOK {
			t.Fatalf("n=%d err=%v bytes=%d status=%d", n, err, w.bytes, w.status)
		}
		if base.Body.String() != "hello" {
			t.Fatalf("body=%q", base.Body.String())
		}
	})

	t.Run("duplicate write header is ignored", func(t *testing.T) {
		base := httptest.NewRecorder()
		w := &statusWriter{ResponseWriter: base}
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusTeapot)
		if w.status != http.StatusCreated || base.Code != http.StatusCreated {
			t.Fatalf("status=%d base=%d", w.status, base.Code)
		}
	})
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	var body map[string]any
	payload := `{"value":"` + strings.Repeat("x", maxJSONBodyBytes) + `"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	if err := DecodeJSON(r, &body); err == nil {
		t.Fatal("expected oversized JSON body to fail")
	}
}

type fakeContextPinger struct {
	err error
}

func (f fakeContextPinger) PingContext(context.Context) error {
	return f.err
}

func TestLivenessHandler(t *testing.T) {
	w := httptest.NewRecorder()
	LivenessHandler("test-service").ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), `"status":"alive"`) || !strings.Contains(w.Body.String(), `"service":"test-service"`) {
		t.Fatalf("unexpected body=%q", w.Body.String())
	}
}

func TestReadinessHandler(t *testing.T) {
	oldWriter := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(oldWriter) })

	t.Run("ready", func(t *testing.T) {
		check := DBReadinessCheck("mysql", fakeContextPinger{})
		w := httptest.NewRecorder()
		ReadinessHandler("test-service", time.Second, check).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"ready"`) {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("dependency unavailable", func(t *testing.T) {
		check := DBReadinessCheck("mysql", fakeContextPinger{err: errors.New("db unavailable")})
		w := httptest.NewRecorder()
		ReadinessHandler("test-service", time.Second, check).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), `"status":"not_ready"`) {
			t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "db unavailable") {
			t.Fatalf("readiness response leaked dependency error: %q", w.Body.String())
		}
	})

	t.Run("nil database handle", func(t *testing.T) {
		var pinger interface {
			PingContext(context.Context) error
		}
		check := DBReadinessCheck("mysql", pinger)
		w := httptest.NewRecorder()
		ReadinessHandler("test-service", 0, check).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d, want %d", w.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("custom check failure", func(t *testing.T) {
		check := ReadinessCheck{Name: "config", Check: func(context.Context) error { return errors.New("not initialized") }}
		w := httptest.NewRecorder()
		ReadinessHandler("test-service", time.Second, check).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d, want %d", w.Code, http.StatusServiceUnavailable)
		}
	})
}

func TestServeServerRejectsNilServer(t *testing.T) {
	if err := ServeServer(context.Background(), nil, "test-service", time.Second); err == nil {
		t.Fatal("expected nil server to fail")
	}
}

func TestServeServerGracefulShutdown(t *testing.T) {
	oldWriter := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(oldWriter) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := NewServer("127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := ServeServer(ctx, server, "test-service", 250*time.Millisecond); err != nil {
		t.Fatalf("ServeServer() error=%v", err)
	}
}

func TestRunServerReturnsListenError(t *testing.T) {
	server := NewServer("bad-address", http.NotFoundHandler())
	if err := RunServer(server, "test-service", time.Second); err == nil {
		t.Fatal("expected invalid listen address to fail")
	}
}

func TestSetRetryAfter(t *testing.T) {
	w := httptest.NewRecorder()
	SetRetryAfter(w, 1500*time.Millisecond)
	if got := w.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("Retry-After=%q want 2", got)
	}
}

func TestWriteErrorUsesStandardEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusTooManyRequests, "too many requests")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type=%q", got)
	}
	if !strings.Contains(w.Body.String(), `"error":"too many requests"`) {
		t.Fatalf("body=%q", w.Body.String())
	}
}

func TestHealthHandlerAndRegistration(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHealthRoutes(mux, "quality-service")

	for path, wantStatus := range map[string]string{
		"/healthz": `"status":"ok"`,
		"/livez":   `"status":"alive"`,
		"/readyz":  `"status":"ready"`,
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), wantStatus) || !strings.Contains(w.Body.String(), `"service":"quality-service"`) {
			t.Fatalf("path=%s status=%d body=%q", path, w.Code, w.Body.String())
		}
	}
}
