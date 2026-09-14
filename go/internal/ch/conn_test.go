package ch

import (
	"net/url"
	"testing"
)

func TestQueryCacheDSN(t *testing.T) {
	const in = "clickhouse://default:@127.0.0.1:9000/bosskills?dial_timeout=10s&max_execution_time=60"
	got, err := queryCacheDSN(in)
	if err != nil {
		t.Fatalf("queryCacheDSN: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}
	q := u.Query()

	// Cache settings applied.
	for k, want := range map[string]string{
		"use_query_cache":                "1",
		"query_cache_ttl":                "3600",
		"query_cache_min_query_duration": "100",
	} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
	// Pre-existing settings survive - dropping dial_timeout would silently
	// restore the 1s default and break startup on a slow box.
	if q.Get("dial_timeout") != "10s" {
		t.Errorf("dial_timeout = %q, want 10s", q.Get("dial_timeout"))
	}
	if q.Get("max_execution_time") != "60" {
		t.Errorf("max_execution_time = %q, want 60", q.Get("max_execution_time"))
	}
	// Credentials and database survive.
	if u.Path != "/bosskills" {
		t.Errorf("path = %q, want /bosskills", u.Path)
	}
	if u.User == nil || u.User.Username() != "default" {
		t.Errorf("user = %v, want default", u.User)
	}
	if u.Host != "127.0.0.1:9000" {
		t.Errorf("host = %q", u.Host)
	}
}

func TestQueryCacheDSNNoExistingParams(t *testing.T) {
	got, err := queryCacheDSN("clickhouse://127.0.0.1:9000/bosskills")
	if err != nil {
		t.Fatalf("queryCacheDSN: %v", err)
	}
	u, _ := url.Parse(got)
	if u.Query().Get("use_query_cache") != "1" {
		t.Errorf("use_query_cache not set: %q", got)
	}
}
