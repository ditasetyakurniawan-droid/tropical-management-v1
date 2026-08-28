package logx

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultLogDir      = "/var/log/tropical"
	defaultMaxSizeMB   = 25
	defaultMaxBackups  = 5
	minimumMaxSizeMB   = 1
	maximumMaxSizeMB   = 1024
	maximumBackupCount = 20
)

// Configure routes the process-wide standard logger to a rotating service log file
// and, by default, stdout. It returns a cleanup function that should be deferred.
//
// Environment variables:
//
//	LOG_DIR          directory for service log files (default /var/log/tropical)
//	LOG_MAX_SIZE_MB  maximum active log size before rotation (default 25)
//	LOG_MAX_BACKUPS  number of rotated files to retain (default 5)
//	LOG_STDOUT       true/false; also mirror logs to stdout (default true)
func Configure(service string) (func() error, error) {
	service = sanitizeServiceName(service)
	if service == "" {
		return nil, fmt.Errorf("logx: service name is required")
	}

	dir := strings.TrimSpace(os.Getenv("LOG_DIR"))
	if dir == "" {
		dir = defaultLogDir
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("logx: create log directory %q: %w", dir, err)
	}

	maxSizeMB := envInt("LOG_MAX_SIZE_MB", defaultMaxSizeMB, minimumMaxSizeMB, maximumMaxSizeMB)
	maxBackups := envInt("LOG_MAX_BACKUPS", defaultMaxBackups, 1, maximumBackupCount)
	path := filepath.Join(dir, service+".log")

	rotator, err := newRotatingWriter(path, int64(maxSizeMB)*1024*1024, maxBackups)
	if err != nil {
		return nil, err
	}

	writers := []io.Writer{rotator}
	if envBool("LOG_STDOUT", true) {
		writers = append(writers, os.Stdout)
	}

	log.SetOutput(io.MultiWriter(writers...))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.LUTC)
	log.SetPrefix("service=" + service + " ")

	return rotator.Close, nil
}

func sanitizeServiceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-.")
}

func envInt(key string, fallback, minValue, maxValue int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minValue || value > maxValue {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

type rotatingWriter struct {
	mu         sync.Mutex
	path       string
	maxBytes   int64
	maxBackups int
	file       *os.File
	size       int64
}

func newRotatingWriter(path string, maxBytes int64, maxBackups int) (*rotatingWriter, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("logx: maxBytes must be positive")
	}
	if maxBackups < 1 {
		return nil, fmt.Errorf("logx: maxBackups must be at least 1")
	}

	w := &rotatingWriter{path: path, maxBytes: maxBytes, maxBackups: maxBackups}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("logx: open log file %q: %w", w.path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("logx: stat log file %q: %w", w.path, err)
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return 0, fmt.Errorf("logx: writer is closed")
	}
	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("logx: close active log before rotation: %w", err)
	}
	w.file = nil

	oldest := fmt.Sprintf("%s.%d", w.path, w.maxBackups)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("logx: remove oldest log backup: %w", err)
	}

	for i := w.maxBackups - 1; i >= 1; i-- {
		from := fmt.Sprintf("%s.%d", w.path, i)
		to := fmt.Sprintf("%s.%d", w.path, i+1)
		if err := os.Rename(from, to); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("logx: rotate %q to %q: %w", from, to, err)
		}
	}

	if err := os.Rename(w.path, w.path+".1"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("logx: rotate active log: %w", err)
	}
	return w.open()
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
