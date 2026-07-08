package home

import (
	"net/http"

	"twinstar-bosskills/internal/realm"
)

func Mount(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		// If a previous session set a realm cookie, honour it.
		if c, err := r.Cookie("last-realm"); err == nil && c != nil && realm.IsKnown(c.Value) {
			http.Redirect(w, r, "/"+realm.Normalize(c.Value)+"/", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/"+realm.Helios+"/", http.StatusFound)
	})
}
