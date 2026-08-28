package httpx

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

const (
	maxJSONBodyBytes = 1 << 20
	internalErrorMsg = "internal server error"
)

type requestIDContextKey struct{}

// JSON keeps every service response consistent and avoids repeating headers.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("event=json_encode_error error=%q", err)
	}
}

// InternalError logs the implementation detail server-side and returns a stable,
// non-sensitive message to the client.
func InternalError(w http.ResponseWriter, err error) {
	if err != nil {
		log.Printf("event=internal_error error=%q", err)
	}
	JSON(w, http.StatusInternalServerError, map[string]string{"error": internalErrorMsg})
}

// DecodeJSON rejects unknown fields, payloads larger than 1 MiB, and trailing
// JSON values so malformed API requests fail closed.
func DecodeJSON(r *http.Request, dst any) error {
	if r == nil || r.Body == nil {
		return errors.New("request body is required")
	}
	defer r.Body.Close()

	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxJSONBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain a single JSON value")
		}
		return fmt.Errorf("invalid trailing json: %w", err)
	}
	return nil
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

// Secret reads a secret through Env and enforces a minimum byte length.
// This prevents accidentally starting with trivially weak signing keys.
func Secret(key, fallback string, minBytes int) string {
	value := Env(key, fallback)
	if len([]byte(value)) < minBytes {
		panic(fmt.Sprintf("httpx.Secret: %s must be at least %d bytes", key, minBytes))
	}
	return value
}

func BearerToken(r *http.Request) (string, error) {
	if r == nil {
		return "", errors.New("missing bearer token")
	}
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("missing bearer token")
	}
	return strings.TrimSpace(parts[1]), nil
}

// RequestID returns the correlation identifier attached by RequestLogger.
func RequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

// RequestLogger adds/propagates X-Request-ID, logs request completion, and
// recovers panics without exposing stack traces to clients.
func RequestLogger(service string, next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := incomingRequestID(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		r = r.WithContext(ctx)
		r.Header.Set("X-Request-ID", requestID)
		w.Header().Set("X-Request-ID", requestID)

		recorder := &statusWriter{ResponseWriter: w}
		started := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("event=panic request_id=%q panic=%q stack=%q", requestID, fmt.Sprint(recovered), debug.Stack())
				if !recorder.wroteHeader {
					JSON(recorder, http.StatusInternalServerError, map[string]string{"error": internalErrorMsg})
				}
			}
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			log.Printf(
				"event=http_request request_id=%q service=%q method=%q path=%q status=%d bytes=%d duration_ms=%d",
				requestID,
				service,
				r.Method,
				r.URL.Path,
				status,
				recorder.bytes,
				time.Since(started).Milliseconds(),
			)
		}()

		next.ServeHTTP(recorder, r)
	})
}

// NewServer applies conservative HTTP server defaults. WriteTimeout is left at
// zero because the API gateway and chat service support long-lived SSE streams.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func incomingRequestID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 0 || len(value) > 128 {
		return ""
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.', r == ':':
		default:
			return ""
		}
	}
	return value
}

func newRequestID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data[:])
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

func (w *statusWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("http hijacking is not supported")
	}
	return hijacker.Hijack()
}

func (w *statusWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		n, err := readerFrom.ReadFrom(r)
		w.bytes += n
		return n, err
	}
	n, err := io.Copy(struct{ io.Writer }{w.ResponseWriter}, r)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
