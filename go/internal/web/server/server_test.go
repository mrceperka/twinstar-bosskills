package server

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mrceperka/twinstar-bosskills/go/internal/cache"
	"github.com/mrceperka/twinstar-bosskills/go/internal/ch"
)

// openTestDB returns a CH connection if BK_CH_DSN is set, otherwise t.Skip's.
//
// Tests hit a real ClickHouse so they double as parity checks against the
// schema + MVs. CI environments without CH skip these — unit tests for pure
// logic live next to their packages and don't need this.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BK_CH_DSN")
	if dsn == "" {
		t.Skip("BK_CH_DSN not set; skipping integration tests")
	}
	db, err := ch.Open(ch.Options{DSN: dsn})
	if err != nil {
		t.Fatalf("ch.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	db := openTestDB(t)
	icons, err := cache.NewIconDisk(t.TempDir())
	if err != nil {
		t.Fatalf("icons: %v", err)
	}
	cfg := Config{
		DB:          db,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Icons:       icons,
		SecretGuild: "test-guild-secret",
	}
	srv := httptest.NewServer(New(cfg))
	t.Cleanup(srv.Close)
	return srv
}

// noRedirectClient is needed for the redirect tests since the default
// http.Client follows 30x.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body: %s", body)
	}
}

func TestRootRedirectsToHelios(t *testing.T) {
	srv := newTestServer(t)
	resp, err := noRedirectClient().Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasSuffix(loc, "/Helios/") {
		t.Errorf("Location: %s", loc)
	}
}

func TestLowercaseRealmRedirectsToCanonical(t *testing.T) {
	srv := newTestServer(t)
	resp, err := noRedirectClient().Get(srv.URL + "/helios/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasSuffix(loc, "/Helios/") {
		t.Errorf("Location: %s", loc)
	}
}

func TestApolloRedirectsToAthena(t *testing.T) {
	srv := newTestServer(t)
	resp, err := noRedirectClient().Get(srv.URL + "/Apollo/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasSuffix(loc, "/Athena/") {
		t.Errorf("Location: %s", loc)
	}
}

func TestUnknownRealm404(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/Atlantis/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func TestPrivateRealmBlocksWithoutCookie(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/MoPPvE/boss-kills")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "guild-token") {
		t.Errorf("body missing guide: %s", body)
	}
}

func TestPublicRealmDashboard(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/Helios/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func TestRaidsPage(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/Helios/raids")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func TestBossPage(t *testing.T) {
	srv := newTestServer(t)
	// 71865 = Garrosh Hellscream — known to exist in seed data
	resp, err := http.Get(srv.URL + "/Helios/boss/71865")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func TestBossPagePercentileFilter(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/Helios/boss/71865?mode=7&p=85")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "DPS @ p85") {
		t.Errorf("body missing p85 label: snippet=%q", snippet(string(body), "@ p"))
	}
}

func TestBossPageHTMXFragment(t *testing.T) {
	srv := newTestServer(t)
	req, _ := http.NewRequest("GET", srv.URL+"/Helios/boss/71865?mode=5", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "<html") {
		t.Errorf("htmx fragment should not include <html>; got snippet=%q", string(body[:min(200, len(body))]))
	}
	if !strings.Contains(string(body), "DPS by Talent Spec") && !strings.Contains(string(body), "Top DPS") {
		t.Errorf("htmx fragment missing expected chart panel")
	}
}

func TestBossKillsList(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/Helios/boss-kills")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func TestUnknownBoss404(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/Helios/boss/99999999")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func TestIconProxyBadType(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/img/icon?type=evil&id=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func TestStaticAsset(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type: %s", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "max-age") {
		t.Errorf("Cache-Control missing: %s", cc)
	}
}

// snippet returns a window around the first occurrence of needle in s.
func snippet(s, needle string) string {
	i := strings.Index(s, needle)
	if i < 0 {
		return "(not found)"
	}
	lo := max(0, i-40)
	hi := min(len(s), i+80)
	return s[lo:hi]
}
