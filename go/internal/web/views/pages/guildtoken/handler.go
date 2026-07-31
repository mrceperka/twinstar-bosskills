package guildtoken

import (
	"net/http"
	"strings"
	"time"

	"twinstar-bosskills/internal/domain"
	"twinstar-bosskills/internal/web/middleware"
	"twinstar-bosskills/internal/web/router"
	"twinstar-bosskills/internal/web/views/layouts"
)

type Deps struct {
	SecretGuild string
	SecretAdmin string
	CSSHash     string
	JSHash      string
}

const (
	cookieGuildName  = "guild-name"
	cookieGuildToken = "guild-token"
	cookieAdminToken = "admin-token"
	cookieMaxAge     = int(time.Hour * 24 * 365 / time.Second) // 1y
)

func TokenHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())

		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:   realmName + " / Guild token",
				Realm:   realmName,
				CSSHash: deps.CSSHash,
				JSHash:  deps.JSHash,
			},
			Realm:        realmName,
			CurrentGuild: cookieValue(r, cookieGuildName),
			CurrentToken: cookieValue(r, cookieGuildToken),
		}

		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			guild := strings.TrimSpace(r.FormValue("guild"))
			token := strings.TrimSpace(r.FormValue("token"))
			if guild == "" || token == "" {
				vm.StatusError = "Guild and token are required."
			} else if !domain.VerifyGuildToken(deps.SecretGuild, realmName, guild, token) {
				vm.StatusError = "Token does not match this guild."
			} else {
				setCookie(w, cookieGuildName, guild)
				setCookie(w, cookieGuildToken, token)
				vm.CurrentGuild = guild
				vm.CurrentToken = token
				vm.StatusOK = true
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

func GenerateHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		realmName := middleware.Realm(r.Context())
		vm := GenerateViewModel{
			Meta: layouts.PageMeta{
				Title:   realmName + " / Generate token",
				Realm:   realmName,
				CSSHash: deps.CSSHash,
				JSHash:  deps.JSHash,
			},
			Realm:    realmName,
			HasAdmin: cookieValue(r, cookieAdminToken) != "",
		}

		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			guild := strings.TrimSpace(r.FormValue("guild"))
			adminToken := strings.TrimSpace(r.FormValue("admin_token"))
			if adminToken == "" {
				adminToken = cookieValue(r, cookieAdminToken)
			}
			vm.GeneratedGuild = guild

			if guild == "" {
				vm.StatusError = "Guild is required."
			} else if deps.SecretAdmin == "" {
				vm.StatusError = "Server has no admin secret configured."
			} else if adminToken != deps.SecretAdmin {
				vm.StatusError = "Admin token invalid."
			} else {
				setCookie(w, cookieAdminToken, adminToken)
				vm.HasAdmin = true
				vm.GeneratedToken = domain.GuildToken(deps.SecretGuild, realmName, guild)
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = GeneratePage(vm).Render(r.Context(), w)
	}
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil || c == nil {
		return ""
	}
	return c.Value
}

func setCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure is left unset; ops can rely on a reverse proxy to force HTTPS.
	})
}

func Mount(mux *http.ServeMux, deps Deps) {
	tokH := middleware.RequireRealm(TokenHandler(deps))
	genH := middleware.RequireRealm(GenerateHandler(deps))
	router.ForEachRealmPrefix(func(prefix string) {
		mux.Handle("GET "+prefix+"/guild-token", tokH)
		mux.Handle("POST "+prefix+"/guild-token", tokH)
		mux.Handle("GET "+prefix+"/guild-token/generate", genH)
		mux.Handle("POST "+prefix+"/guild-token/generate", genH)
	})
}
