package middleware

import (
	"context"
	"net/http"

	"twinstar-bosskills/internal/web/query"
)

// raidLockValue is the parsed ?raidlock=N (or ?offset=N) result, cached on the
// request context so every page resolves the raid-lock the same way.
type raidLockValue struct {
	offset int
	ok     bool
}

// AttachRaidLock parses the raid-lock offset query param once per request and
// stores it on the context. Handlers read it via middleware.RaidLock.
func AttachRaidLock(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, ok := query.RaidLock(r.URL.Query())
		v := raidLockValue{offset: offset, ok: ok}
		ctx := context.WithValue(r.Context(), keyRaidLock, v)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RaidLock returns the raid-lock offset attached by AttachRaidLock. `ok` is
// false when no raidlock param was present, in which case offset is 0 (the
// current lock). Mirrors query.RaidLock so pages can switch to it directly.
func RaidLock(ctx context.Context) (offset int, ok bool) {
	v, _ := ctx.Value(keyRaidLock).(raidLockValue)
	return v.offset, v.ok
}
