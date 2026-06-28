package icon

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/api"
	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
)

// tooltipHandler serves GET /img/tooltip?id=N&realm=R.
//
// It proxies the upstream item tooltip API using the realm's expansion so
// the correct version of the tooltip is returned per realm. Responses are
// cached by the browser for 15 minutes.
func tooltipHandler(cli *api.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		id, err := strconv.Atoi(q.Get("id"))
		if err != nil || id <= 0 {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		realmName := realm.Normalize(q.Get("realm"))
		exp := realm.Expansion(realmName)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		html, err := cli.GetItemTooltip(ctx, id, exp)
		if err != nil {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		if html == "" {
			http.NotFound(w, r)
			return
		}

		h := w.Header()
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Cache-Control", "public, max-age=900") // 15 min browser cache
		_, _ = io.WriteString(w, html)
	}
}

// MountTooltip registers GET /img/tooltip on mux.
func MountTooltip(mux *http.ServeMux, apiBase string) {
	if apiBase == "" {
		apiBase = api.DefaultBaseURL
	}
	cli := api.NewClient(apiBase)
	mux.Handle("GET /img/tooltip", tooltipHandler(cli))
}
