package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mrceperka/twinstar-bosskills/go/internal/domain"
)

func newReqWithRealm(method, path, realm string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r = r.WithContext(context.WithValue(r.Context(), keyRealm, realm))
	return r
}

func TestAttachGuildAuth_PublicRealmAlwaysVerified(t *testing.T) {
	h := AttachGuildAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := Auth(r.Context())
		if !a.Verified || !a.IsPublic {
			t.Errorf("public realm should be verified+public, got %#v", a)
		}
	}))
	h.ServeHTTP(httptest.NewRecorder(), newReqWithRealm("GET", "/Helios/x", "Helios"))
}

func TestAttachGuildAuth_PrivateRealmRequiresMatch(t *testing.T) {
	secret := "secret"
	realm := "MoPPvE"
	guild := "Acme"
	good := domain.GuildToken(secret, realm, guild)

	h := AttachGuildAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := Auth(r.Context())
		if !a.Verified {
			t.Errorf("expected verified, got %#v", a)
		}
	}))
	r := newReqWithRealm("GET", "/MoPPvE/x", realm)
	r.AddCookie(&http.Cookie{Name: cookieGuildName, Value: guild})
	r.AddCookie(&http.Cookie{Name: cookieGuildToken, Value: good})
	h.ServeHTTP(httptest.NewRecorder(), r)
}
