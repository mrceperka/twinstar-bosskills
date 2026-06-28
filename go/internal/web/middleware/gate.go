package middleware

import "net/http"

// GatePrivateRealm writes a 403 and returns true if the request targets a
// private realm and has no verified guild token. Handlers should `if
// middleware.GatePrivateRealm(w, r) { return }` near the top of any function
// that exposes per-guild data.
//
// Public realms always pass through.
func GatePrivateRealm(w http.ResponseWriter, r *http.Request) bool {
	auth := Auth(r.Context())
	if auth.IsPublic || auth.Verified {
		return false
	}
	http.Error(w,
		"Guild token required for this realm. Set one at /"+auth.Realm+"/guild-token.",
		http.StatusForbidden)
	return true
}

// PrivateRealmGuildFilter returns the guild the request should be filtered
// by, or empty string if no filter applies (public realm). Used by list-style
// handlers that should restrict per-guild data.
func PrivateRealmGuildFilter(r *http.Request) string {
	auth := Auth(r.Context())
	if auth.IsPublic {
		return ""
	}
	if auth.Verified {
		return auth.Guild
	}
	// Caller should already have GatePrivateRealm'd; defensive empty value
	// here would still leak — return a sentinel that no real guild can match.
	return "\x00impossible-guild\x00"
}
