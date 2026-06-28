package changelog

import (
	"net/http"

	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/layouts"
)

type Deps struct {
	CSSHash string
	JSHash  string
}

// entries are hard-coded for now. When the rewrite reaches parity the user can
// replace this with whatever source they prefer (CMS, embedded markdown, etc).
var entries = []Entry{
	{
		Date:  "2026-06-27",
		Title: "Go + htmx rewrite, phase 1",
		Notes: []string{
			"New backend built in Go with ClickHouse storage.",
			"Templates rendered server-side with templ + Tailwind v4.",
			"Interactivity via htmx — no SPA bundle.",
			"Existing SvelteKit app preserved during transition.",
		},
	},
	{
		Date:  "2026-04-01",
		Title: "Kronos realm added",
		Notes: []string{
			"Initial vanilla-era ranking data.",
		},
	},
}

func Handler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title:   "Changelog",
				CSSHash: deps.CSSHash,
				JSHash:  deps.JSHash,
			},
			Entries: entries,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

func Mount(mux *http.ServeMux, deps Deps) {
	mux.Handle("GET /changelog", Handler(deps))
	mux.Handle("GET /changelog/{$}", Handler(deps))
}
