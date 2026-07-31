package server

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"twinstar-bosskills/internal/cache"
	"twinstar-bosskills/internal/ch"
	"twinstar-bosskills/internal/config"
	"twinstar-bosskills/internal/domain"
	"twinstar-bosskills/internal/wow"
)

const (
	testBossID   uint32 = 990001
	testBossMode        = wow.DifficultyMoP10Normal
)

var (
	serverFixtureOnce     sync.Once
	serverFixtureErr      error
	serverFixturePrepared bool
)

type testServer struct {
	URL    string
	client *http.Client
	db     *sql.DB
}

type handlerTransport struct {
	handler http.Handler
}

type testLogWriter struct {
	t *testing.T
}

func (w testLogWriter) Write(p []byte) (int, error) {
	w.t.Helper()
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

func (t handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func TestMain(m *testing.M) {
	code := m.Run()
	if serverFixturePrepared {
		if err := cleanupServerFixture(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup server fixture: %v\n", err)
			if code == 0 {
				code = 1
			}
		}
	}
	os.Exit(code)
}

// openTestDB returns a CH connection for server integration tests.
//
// Tests hit a real ClickHouse so they double as parity checks against the
// schema + MVs. The configured ClickHouse DSN must point at a running
// ClickHouse.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	cfg := config.FromEnv()
	if cfg.ClickHouse.DSN == "" {
		t.Fatal(config.EnvClickHouseDSN + " not set")
	}
	db, err := ch.Open(ch.Options{DSN: cfg.ClickHouse.DSN})
	if err != nil {
		t.Fatalf("ch.Open: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatalf("ClickHouse unavailable at %s: %v", config.EnvClickHouseDSN, err)
	}
	serverFixtureOnce.Do(func() {
		serverFixtureErr = insertServerFixture(db)
	})
	if serverFixtureErr != nil {
		_ = db.Close()
		t.Fatalf("insert server fixture: %v", serverFixtureErr)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertServerFixture(db *sql.DB) error {
	lock := domain.RaidLock(time.Now().UTC(), 0)
	var existing uint64
	if err := db.QueryRow(`
		SELECT count()
		FROM boss_kill
		WHERE realm = ? AND boss_remote_id = ? AND mode = ?
		  AND kill_time >= ? AND kill_time < ?
	`, "Helios", testBossID, uint8(testBossMode), lock.Start, lock.End).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		serverFixturePrepared = true
		return nil
	}

	killTime := lock.Start.Add(2 * time.Hour)
	remoteID := fmt.Sprintf("server_test_%s", lock.Start.Format("20060102T150405Z"))
	if _, err := db.Exec(`
		INSERT INTO boss_kill (
			remote_id, realm, raid_name, boss_remote_id, boss_name, mode, guild,
			kill_time, length, wipes, deaths, ress_used,
			players.guid, players.talent_spec, players.avg_item_lvl, players.dmg_done,
			players.healing_done, players.overhealing_done, players.absorb_done,
			players.dmg_taken, players.dmg_absorbed, players.healing_taken,
			players.dispels, players.interrupts, players.name, players.race,
			players.class, players.gender, players.level,
			deaths_detail.remote_id, deaths_detail.guid, deaths_detail.time,
			loot.remote_id, loot.item_id, loot.count,
			timeline.time, timeline.encounter_damage, timeline.encounter_heal,
			timeline.raid_damage, timeline.raid_heal
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		remoteID, "Helios", "Server Test Raid", testBossID, "Server Test Boss", uint8(testBossMode), "Server Test Guild",
		killTime, uint32(300000), uint32(1), uint32(0), uint32(0),
		[]uint64{99000101, 99000102},
		[]uint16{62, 71},
		[]float32{540, 535},
		[]uint64{90000000, 75000000},
		[]uint64{12000000, 6000000},
		[]uint64{1000000, 500000},
		[]uint64{3000000, 1500000},
		[]uint64{1000000, 1200000},
		[]uint64{200000, 250000},
		[]uint64{15000000, 12000000},
		[]uint32{0, 1},
		[]uint32{0, 0},
		[]string{"Servermage", "Serverwar"},
		[]uint8{1, 1},
		[]uint8{wow.ClassMage, wow.ClassWarrior},
		[]uint8{0, 0},
		[]uint8{90, 90},
		[]uint32{},
		[]uint64{},
		[]int32{},
		[]uint32{},
		[]uint32{},
		[]uint8{},
		[]int32{},
		[]uint64{},
		[]uint64{},
		[]uint64{},
		[]uint64{},
	); err != nil {
		return err
	}
	serverFixturePrepared = true
	return nil
}

func cleanupServerFixture() error {
	cfg := config.FromEnv()
	if cfg.ClickHouse.DSN == "" {
		return fmt.Errorf("%s not set", config.EnvClickHouseDSN)
	}
	db, err := ch.Open(ch.Options{DSN: cfg.ClickHouse.DSN})
	if err != nil {
		return err
	}
	defer db.Close()

	lock := domain.RaidLock(time.Now().UTC(), 0)
	_, err = db.Exec(`
		ALTER TABLE boss_kill
		DELETE WHERE realm = ? AND boss_remote_id = ? AND guild = ?
		  AND kill_time >= ? AND kill_time < ?
		SETTINGS mutations_sync = 1
	`, "Helios", testBossID, "Server Test Guild", lock.Start, lock.End)
	return err
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	db := openTestDB(t)
	icons, err := cache.NewIconDisk(t.TempDir())
	if err != nil {
		t.Fatalf("icons: %v", err)
	}
	cfg := Config{
		DB:          db,
		Logger:      slog.New(slog.NewTextHandler(testLogWriter{t: t}, nil)),
		Icons:       icons,
		SecretGuild: "test-guild-secret",
	}
	handler := New(cfg)
	return &testServer{
		URL: "http://twinstar-bosskills.test",
		client: &http.Client{
			Transport: handlerTransport{handler: handler},
		},
		db: db,
	}
}

// noRedirectClient is needed for the redirect tests since the default
// http.Client follows 30x.
func (s *testServer) noRedirectClient() *http.Client {
	return &http.Client{
		Transport: s.client.Transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func requireStatus(t *testing.T, resp *http.Response, want int) string {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("expected status %d, got %d; body: %s", want, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body)
}

func (s *testServer) latestBossRaidLockParam(t *testing.T, realmName string, bossID uint32, mode int) string {
	t.Helper()
	const q = `
		SELECT max(kill_time)
		FROM boss_kill
		WHERE realm = ? AND boss_remote_id = ? AND mode = ? AND length > 0
	`
	var latest time.Time
	if err := s.db.QueryRow(q, realmName, bossID, uint8(mode)).Scan(&latest); err != nil {
		t.Fatalf("load latest boss kill: %v", err)
	}
	if latest.IsZero() {
		t.Fatalf("no boss kills found for %s boss=%d mode=%d", realmName, bossID, mode)
	}
	currentLock := domain.RaidLock(time.Now().UTC(), 0).Start
	dataLock := domain.RaidLock(latest, 0).Start
	offset := int(currentLock.Sub(dataLock).Hours() / (24 * 7))
	if offset < 0 {
		offset = 0
	}
	return fmt.Sprintf("raidlock=%d", offset)
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := requireStatus(t, resp, http.StatusOK)
	if body != "ok" {
		t.Errorf("body: %s", body)
	}
}

func TestRootRedirectsToHelios(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.noRedirectClient().Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusFound)
	if loc := resp.Header.Get("Location"); !strings.HasSuffix(loc, "/Helios/") {
		t.Errorf("Location: %s", loc)
	}
}

func TestLowercaseRealmRedirectsToCanonical(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.noRedirectClient().Get(srv.URL + "/helios/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusMovedPermanently)
	if loc := resp.Header.Get("Location"); !strings.HasSuffix(loc, "/Helios/") {
		t.Errorf("Location: %s", loc)
	}
}

func TestInactiveAthena404(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/Athena/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusNotFound)
}

func TestUnknownRealm404(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/Divnej/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusNotFound)
}

func TestPrivateRealmBlocksWithoutCookie(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/MoPPvE/boss-kills")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := requireStatus(t, resp, http.StatusForbidden)
	if !strings.Contains(body, "guild-token") {
		t.Errorf("body missing guide: %s", body)
	}
}

func TestPublicRealmDashboard(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/Helios/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
}

func TestRaidsPage(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/Helios/raids")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
}

func TestBossPage(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(fmt.Sprintf("%s/Helios/boss/%d?mode=%d", srv.URL, testBossID, testBossMode))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
}

func TestBossPagePercentileFilter(t *testing.T) {
	srv := newTestServer(t)
	lock := srv.latestBossRaidLockParam(t, "Helios", testBossID, testBossMode)
	resp, err := srv.client.Get(fmt.Sprintf("%s/Helios/boss/%d?mode=%d&p=85&%s", srv.URL, testBossID, testBossMode, lock))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := requireStatus(t, resp, http.StatusOK)
	if !strings.Contains(body, "DPS @ p85") {
		t.Errorf("body missing p85 label: snippet=%q", snippet(body, "@ p"))
	}
}

func TestBossPageHTMXFragment(t *testing.T) {
	srv := newTestServer(t)
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/Helios/boss/%d?mode=%d", srv.URL, testBossID, testBossMode), nil)
	req.Header.Set("HX-Request", "true")
	resp, err := srv.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := requireStatus(t, resp, http.StatusOK)
	if strings.Contains(body, "<html") {
		t.Errorf("htmx fragment should not include <html>; got snippet=%q", body[:min(200, len(body))])
	}
	if !strings.Contains(body, "DPS by Talent Spec") && !strings.Contains(body, "Top DPS") {
		t.Errorf("htmx fragment missing expected chart panel")
	}
}

func TestBossKillsList(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/Helios/boss-kills")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
}

func TestUnknownBoss404(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/Helios/boss/99999999")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusNotFound)
}

func TestIconProxyBadType(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/img/icon?type=evil&id=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusBadRequest)
}

func TestStaticAsset(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.client.Get(srv.URL + "/static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK)
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
