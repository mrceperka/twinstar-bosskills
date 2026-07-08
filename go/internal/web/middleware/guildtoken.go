package middleware

import (
	"context"
	"net/http"

	"twinstar-bosskills/internal/domain"
	"twinstar-bosskills/internal/realm"
)

const (
	cookieGuildName  = "guild-name"
	cookieGuildToken = "guild-token"
)

// GuildAuth carries the request's guild-token verification state, derived
// from cookies + the SECRET_TOKEN_GUILD.
type GuildAuth struct {
	// Realm name from URL — already canonicalised by RequireRealm.
	Realm string
	// IsPublic mirrors realm.IsPublic.
	IsPublic bool
	// Guild from cookie, if any.
	Guild string
	// Verified is true on public realms unconditionally, or on private realms
	// when the cookie's token matches HMAC(secret + realm + guild).
	Verified bool
}

const keyGuildAuth ctxKey = 100

// Auth returns the guild verification state attached by AttachGuildAuth.
//
// For routes that don't run AttachGuildAuth (everything outside /{realm}/...)
// this returns the zero value with Verified=false.
func Auth(ctx context.Context) GuildAuth {
	v, _ := ctx.Value(keyGuildAuth).(GuildAuth)
	return v
}

// AttachGuildAuth reads the guild cookie, verifies the token for the realm in
// URL.Path's first segment, and stores the result on the context.
//
// Reads URL.Path directly (rather than ctx) so it can run anywhere in the
// chain — including before RequireRealm. For non-realm routes (e.g. /changelog,
// /static) the auth simply stays zero-valued.
//
// Does NOT itself 403 — handlers that gate on private-realm data should call
// Auth(ctx).Verified and decide what to do. This keeps the middleware generic.
func AttachGuildAuth(secretGuild string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			realmName := Realm(r.Context())
			if realmName == "" {
				raw := firstPathSegment(r.URL.Path)
				if realm.IsKnown(raw) {
					realmName = realm.Normalize(raw)
				}
			}
			auth := GuildAuth{
				Realm:    realmName,
				IsPublic: realm.IsPublic(realmName),
				Guild:    cookieValueRaw(r, cookieGuildName),
			}
			if realmName != "" {
				token := cookieValueRaw(r, cookieGuildToken)
				auth.Verified = domain.VerifyGuildToken(secretGuild, realmName, auth.Guild, token)
			}
			ctx := context.WithValue(r.Context(), keyGuildAuth, auth)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func cookieValueRaw(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil || c == nil {
		return ""
	}
	return c.Value
}
