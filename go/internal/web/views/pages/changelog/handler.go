package changelog

import (
	"net/http"

	"twinstar-bosskills/internal/links"
	"twinstar-bosskills/internal/realm"
	"twinstar-bosskills/internal/web/views/layouts"
)

type Deps struct {
	CSSHash string
	JSHash  string
}

// entries mirrors packages/sveltekit/src/routes/changelog/+page.svelte so the
// content stays in sync between the two apps. Newest first.
func newEntries() []Entry {
	ranksHref := links.Ranks(realm.Helios)
	return []Entry{
		{
			Date:      "2025-10-07",
			TitleHTML: `<a href="` + ranksHref + `" class="text-bk-accent underline">Ranks</a> introduced`,
			Paragraphs: []string{
				`Calculated for the current (each day at 5AM) and previous raid locks (each WED at 6AM). Cached for 1 day.`,
			},
			Hint: &Hint{
				Title:      `Why bother? Crossic asked nicely`,
				Paragraphs: []string{`Apparently this feature is useful to someone :D`},
			},
		},
		{
			Date:      "2024-06-30",
			TitleHTML: `Cache expiration set to 1 day for stats in boss detail`,
		},
		{
			Date:      "2024-06-27",
			TitleHTML: `Cache expiration set to 30 minutes for stats in boss detail`,
		},
		{
			Date:      "2024-05-19",
			TitleHTML: `Cache expiration increased to 1 day for stats in character detail`,
			Hint: &Hint{
				Title:      `But why?!`,
				Paragraphs: []string{`Skill issues and sever went crazy 🔥.`},
			},
		},
		{
			Date:      "2024-05-18",
			TitleHTML: `Character details with overall rankings`,
			Paragraphs: []string{
				`Overall DPS and HPS rankings are now available in character detail. Talent spec and link to the bosskill detail is also there.`,
			},
			Hint: &Hint{
				Paragraphs: []string{
					`You will get 🔥 if you rank is under 200, ⚠️ if under 1000 and 🥦 otherwise.`,
				},
			},
		},
		{
			Date:       "2024-03-17",
			TitleHTML:  `More detailed landing page`,
			Paragraphs: []string{`Previous raid lock charts added to landing page`},
		},
		{
			Date:      "2024-03-11",
			TitleHTML: `Percentiles`,
			Paragraphs: []string{
				`Percentiles (someone calls them parses or parsers) have been added to bosskill details.`,
				`Now you can see how <span class="text-emerald-400">good</span> or <span class="text-yellow-400">bad</span> you are compared to the same spec 😉`,
			},
			Hint: &Hint{
				Title: `WTF is percentile anyway?`,
				Paragraphs: []string{
					`A percentile is a measure indicating the value below which a given percentage of observations in a group of observations falls.`,
					`For example, the 50th percentile is the value below which 50% of the observations may be found, often referred to as the median.`,
					`<a href="https://en.wikipedia.org/wiki/Percentile" target="_blank" rel="noopener" class="text-bk-accent underline">Wikipedia percentile ↗</a>`,
				},
			},
		},
		{
			Date:      "2024-01-01",
			TitleHTML: `DPS and HPS`,
			Paragraphs: []string{
				`DPS and HPS numbers might be slightly different from the ones you can see on Twinhead.`,
			},
			Hint: &Hint{
				Paragraphs: []string{
					`We are dividing the value by <code class="line-through">usefullTime</code> <code>bosskill.length</code> when calculating the average.`,
				},
			},
		},
	}
}

func Handler(deps Deps) http.HandlerFunc {
	entries := newEntries()
	return func(w http.ResponseWriter, r *http.Request) {
		vm := ViewModel{
			Meta: layouts.PageMeta{
				Title: "Changelog",
				// The changelog isn't realm-scoped, but the top nav is - surface
				// the last visited realm so nav links keep working. Fall back
				// to Helios, matching the home-page auto-redirect convention.
				Realm:   navRealm(r),
				CSSHash: deps.CSSHash,
				JSHash:  deps.JSHash,
			},
			Entries: entries,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = Page(vm).Render(r.Context(), w)
	}
}

func navRealm(r *http.Request) string {
	if c, err := r.Cookie("last-realm"); err == nil && c != nil && realm.IsKnown(c.Value) {
		return realm.Normalize(c.Value)
	}
	return realm.Helios
}

func Mount(mux *http.ServeMux, deps Deps) {
	mux.Handle("GET /changelog", Handler(deps))
	mux.Handle("GET /changelog/{$}", Handler(deps))
}
