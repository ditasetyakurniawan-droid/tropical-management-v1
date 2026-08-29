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
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ditasetyakurniawan-droid/tropical-management-v1/internal/configx"
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

// WriteError writes the standard JSON error envelope used by all HTTP services.
func WriteError(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// SetRetryAfter emits an RFC-compatible Retry-After delay in whole seconds.
// Sub-second values are rounded up so clients never receive a zero-second retry.
func SetRetryAfter(w http.ResponseWriter, delay time.Duration) {
	if delay <= 0 {
		delay = time.Second
	}
	seconds := int64((delay + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
}

// InternalError logs the implementation detail server-side and returns a stable,
// non-sensitive message to the client.
func InternalError(w http.ResponseWriter, err error) {
	if err != nil {
		log.Printf("event=internal_error error=%q", err)
	}
	WriteError(w, http.StatusInternalServerError, internalErrorMsg)
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
	return configx.Value(key, fallback)
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

// ReadinessCheck represents one dependency or initialization check required
// before a service can safely receive traffic.
type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

// DBReadinessCheck creates a readiness check backed by database/sql PingContext.
// Database reachability belongs in readiness, not liveness, so a temporary DB
// outage removes the pod from Service endpoints without causing restart loops.
func DBReadinessCheck(name string, db interface {
	PingContext(context.Context) error
}) ReadinessCheck {
	return ReadinessCheck{
		Name: name,
		Check: func(ctx context.Context) error {
			if db == nil {
				return errors.New("database handle is nil")
			}
			return db.PingContext(ctx)
		},
	}
}

// HealthHandler preserves the legacy /healthz compatibility contract.
func HealthHandler(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": service,
		})
	}
}

// RegisterHealthRoutes registers the shared compatibility, liveness, and
// readiness endpoints. Dependency checks belong only to readiness.
func RegisterHealthRoutes(mux *http.ServeMux, service string, checks ...ReadinessCheck) {
	mux.HandleFunc("/healthz", HealthHandler(service))
	mux.HandleFunc("/livez", LivenessHandler(service))
	mux.HandleFunc("/readyz", ReadinessHandler(service, 0, checks...))
}

// LivenessHandler reports only process health. It intentionally does not check
// databases or downstream services because external failures must not trigger
// Kubernetes restart loops.
func LivenessHandler(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, map[string]string{
			"status":  "alive",
			"service": service,
		})
	}
}

// ReadinessHandler reports whether the service is currently safe to receive
// traffic. Dependency checks are bounded by timeout and failures return 503.
func ReadinessHandler(service string, timeout time.Duration, checks ...ReadinessCheck) http.HandlerFunc {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		for _, check := range checks {
			if check.Check == nil {
				continue
			}
			if err := check.Check(ctx); err != nil {
				name := strings.TrimSpace(check.Name)
				if name == "" {
					name = "unnamed"
				}
				log.Printf("event=readiness_failed service=%q check=%q error=%q", service, name, err)
				JSON(w, http.StatusServiceUnavailable, map[string]string{
					"status":  "not_ready",
					"service": service,
				})
				return
			}
		}

		JSON(w, http.StatusOK, map[string]string{
			"status":  "ready",
			"service": service,
		})
	}
}

// RunServer starts an HTTP server and handles SIGINT/SIGTERM with a bounded
// graceful shutdown. Kubernetes sends SIGTERM during normal rollouts and pod
// termination, so in-flight requests get a chance to complete cleanly.
func RunServer(server *http.Server, service string, shutdownTimeout time.Duration) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return ServeServer(ctx, server, service, shutdownTimeout)
}

// ServeServer is the context-driven form of RunServer and is kept separate so
// graceful shutdown behaviour can be exercised in tests without OS signals.
func ServeServer(ctx context.Context, server *http.Server, service string, shutdownTimeout time.Duration) error {
	if server == nil {
		return errors.New("http server is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if shutdownTimeout <= 0 {
		shutdownTimeout = 20 * time.Second
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", service, err)
	}

	errCh := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Printf("event=shutdown_started service=%q timeout_ms=%d", service, shutdownTimeout.Milliseconds())

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("graceful shutdown %s: %w", service, err)
		}

		err := <-errCh
		log.Printf("event=shutdown_completed service=%q", service)
		return err
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
