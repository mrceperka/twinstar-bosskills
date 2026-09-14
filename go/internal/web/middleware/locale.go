package middleware

import (
	"net/http"

	"twinstar-bosskills/internal/format"
)

// AttachLocale parses Accept-Language once and stores the resulting Locale
// on the request context. Templates read it via format.LocaleFromContext.
func AttachLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loc := format.FromAcceptLanguage(r.Header.Get("Accept-Language"))
		ctx := format.WithLocale(r.Context(), loc)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
