package middleware

import (
	"context"
	"net/http"
	"strings"

	"twinstar-bosskills/internal/realm"
)

// RequireRealm extracts the realm from URL.Path's first segment,
// canonicalises case, redirects merged realms (e.g. Apollo -> Athena), and
// rejects unknown realms with 404.
//
// Why URL.Path and not PathValue("realm"): the mux uses literal per-realm
// patterns ("/Helios/...", "/helios/...") rather than a wildcard, because Go's
// ServeMux can't resolve a "/{realm}" wildcard against concrete prefixes like
// "/static/". See internal/web/router/realms.go for context.
func RequireRealm(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := firstPathSegment(r.URL.Path)
		if raw == "" || !realm.IsKnown(raw) {
			http.NotFound(w, r)
			return
		}

		canonical := realm.Normalize(raw)
		if target := realm.MergedTo(canonical); target != "" {
			canonical = target
		}

		if canonical != raw {
			redirectURL := *r.URL
			redirectURL.Path = "/" + canonical + r.URL.Path[len(raw)+1:]
			http.Redirect(w, r, redirectURL.String(), http.StatusMovedPermanently)
			return
		}

		// Remember the realm so future / requests skip the picker.
		http.SetCookie(w, &http.Cookie{
			Name:     "last-realm",
			Value:    canonical,
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 365,
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
		})

		ctx := context.WithValue(r.Context(), keyRealm, canonical)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// firstPathSegment returns the first segment of an absolute URL path
// (without leading or trailing slashes). "/" returns "".
func firstPathSegment(path string) string {
	path = strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return path
}
