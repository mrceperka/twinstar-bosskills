package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// noopOK responds with 200 OK so we can distinguish "passed through" from
// "redirected" and "404'd".
func noopOK(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok " + Realm(r.Context())))
}

func TestRequireRealm_CanonicalCaseRedirect(t *testing.T) {
	h := RequireRealm(http.HandlerFunc(noopOK))

	req := httptest.NewRequest("GET", "/helios/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("status: %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/Helios/x" {
		t.Errorf("Location: %s", got)
	}
}

func TestRequireRealm_InactiveAthena404(t *testing.T) {
	h := RequireRealm(http.HandlerFunc(noopOK))

	req := httptest.NewRequest("GET", "/Athena/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestRequireRealm_CanonicalPassesThrough(t *testing.T) {
	called := false
	h := RequireRealm(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if Realm(r.Context()) != "Helios" {
			t.Errorf("Realm(ctx) = %q", Realm(r.Context()))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/Helios/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	if !called {
		t.Fatal("downstream not called")
	}
}

func TestRequireRealm_UnknownRealm404(t *testing.T) {
	h := RequireRealm(http.HandlerFunc(noopOK))

	req := httptest.NewRequest("GET", "/nope/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestRequireRealm_DeepPathPreserved(t *testing.T) {
	h := RequireRealm(http.HandlerFunc(noopOK))

	req := httptest.NewRequest("GET", "/helios/boss/123/history", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("status: %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "/Helios/boss/123/history" {
		t.Errorf("Location: %s", got)
	}
}
