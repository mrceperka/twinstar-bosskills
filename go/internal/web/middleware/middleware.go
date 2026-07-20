// Package middleware holds the standard chain wrapped around every request.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// ctxKey is a private type so values stored in request context can't collide
// with other packages.
type ctxKey int

const (
	keyRequestID ctxKey = iota + 1
	keyRealm
)

// RequestID returns the value attached by Recover/RequestID. Empty if missing.
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}

// Realm returns the realm attached by Realm middleware. Empty if route is
// realm-independent.
func Realm(ctx context.Context) string {
	v, _ := ctx.Value(keyRealm).(string)
	return v
}

// WithRequestID attaches an X-Request-Id (or a fresh one) to ctx + response.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), keyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Recover logs panics with stack trace + request id and returns 500.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic",
						"err", rec,
						"request_id", RequestID(r.Context()),
						"path", r.URL.Path,
						"stack", string(debug.Stack()),
					)
					http.Error(w, "internal error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Logger wraps each request with a method/path/status/duration line.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(sw, r)
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"bytes", sw.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestID(r.Context()),
			)
		})
	}
}

// SecurityHeaders sets a small, sane default header set. CSP is intentionally
// strict - htmx and inline-script-free templ make this easy.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Allow self + inline styles (Tailwind inlines nothing at runtime, but
		// echarts may inject a <style>; revisit if it bites).
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' https://eu.umami.is; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https://twinstar-api.twinstar-wow.com; "+
				"connect-src 'self' https://eu.umami.is; "+
				"frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// statusWriter captures status + bytes for Logger.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Chain composes the standard request middleware in a sensible order.
func Chain(log *slog.Logger, h http.Handler) http.Handler {
	return WithRequestID(
		Recover(log)(
			Logger(log)(
				SecurityHeaders(
					AttachLocale(h),
				),
			),
		),
	)
}

// IsHTMX reports whether the request came from htmx.
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
