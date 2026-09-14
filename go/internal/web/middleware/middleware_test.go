package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersCSPAllowsAnalytics(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	SecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(recorder, request)

	csp := recorder.Header().Get("Content-Security-Policy")
	for _, directive := range []string{
		"script-src 'self' 'unsafe-inline' https://*.umami.is https://static.cloudflareinsights.com",
		"script-src-attr 'none'",
		"connect-src 'self' https://*.umami.is https://cloudflareinsights.com",
	} {
		if !strings.Contains(csp, directive) {
			t.Errorf("Content-Security-Policy does not contain %q: %q", directive, csp)
		}
	}
}
