package home

import (
	"net/http"

	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
)

// Mount registers the home routes.
//
// `/` redirects straight to the default realm — the realm picker is one
// click away (Phase 4 made the realm panel on `/realms` instead). This makes
// the most common task (look up data on Helios) a one-click op.
func Mount(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		// If a previous session set a realm cookie, honour it.
		if c, err := r.Cookie("last-realm"); err == nil && c != nil && realm.IsKnown(c.Value) {
			http.Redirect(w, r, "/"+realm.Normalize(c.Value)+"/", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/"+realm.Helios+"/", http.StatusFound)
	})
	// Keep the realm picker available behind /realms for users who want the overview.
	mux.Handle("GET /realms", Handler(deps))
	mux.Handle("GET /realms/{$}", Handler(deps))
}
