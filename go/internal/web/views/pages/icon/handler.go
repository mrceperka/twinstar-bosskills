// Package icon is the /img/icon proxy.
//
// URL: GET /img/icon?type=<class|race|raid|item|talent>&id=<id>&realm=<realm>
//
// The proxy:
//   1. validates `type` against an allowlist
//   2. validates `id` shape (integer for class/talent/item; string for race/raid)
//   3. computes the upstream URL using the ported TS rules
//   4. serves from disk cache if present, otherwise fetches + caches
//   5. sets Content-Type + 14-day Cache-Control
package icon

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mrceperka/twinstar-bosskills/go/internal/api"
	"github.com/mrceperka/twinstar-bosskills/go/internal/cache"
	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
)

type Deps struct {
	APIBase string // e.g. https://twinstar-api.twinstar-wow.com
	Icons   *cache.IconDisk
}

const armoryBase = "https://armory.twinstar-wow.com"

func Handler(deps Deps) http.HandlerFunc {
	if deps.APIBase == "" {
		deps.APIBase = api.DefaultBaseURL
	}
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		typ := q.Get("type")
		id := q.Get("id")
		realmName := q.Get("realm")
		if realmName == "" {
			realmName = realm.Helios
		}

		upstream, err := buildUpstreamURL(deps.APIBase, typ, id, realmName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		blob, err := deps.Icons.Get(r.Context(), upstream)
		if err != nil {
			http.Error(w, "upstream fetch failed", http.StatusBadGateway)
			return
		}
		if blob == nil {
			http.NotFound(w, r)
			return
		}

		h := w.Header()
		if blob.ContentType != "" {
			h.Set("Content-Type", blob.ContentType)
		}
		h.Set("Cache-Control", "public, max-age=1209600, immutable")
		if blob.Hit {
			h.Set("X-Cache", "HIT")
		} else {
			h.Set("X-Cache", "MISS")
		}
		_, _ = w.Write(blob.Data)
	}
}

func buildUpstreamURL(apiBase, typ, id, realmName string) (string, error) {
	switch typ {
	case "item":
		n, err := strconv.Atoi(id)
		if err != nil || n <= 0 {
			return "", errBadID
		}
		return apiBase + "/item/icon/" + strconv.Itoa(n), nil
	case "talent":
		n, err := strconv.Atoi(id)
		if err != nil || n <= 0 {
			return "", errBadID
		}
		exp := realm.Expansion(realmName)
		return apiBase + "/talent/icon/" + strconv.Itoa(exp) + "/" + strconv.Itoa(n), nil
	case "class":
		n, err := strconv.Atoi(id)
		if err != nil || n <= 0 {
			return "", errBadID
		}
		return armoryBase + "/img/Classes/" + strconv.Itoa(n) + ".webp", nil
	case "race":
		if !isRaceID(id) {
			return "", errBadID
		}
		return armoryBase + "/img/Races/" + url.PathEscape(id) + ".webp", nil
	case "raid":
		if id == "" {
			return "", errBadID
		}
		lc := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(id, "'", ""), " ", "-"))
		return apiBase + "/img/raids/" + url.PathEscape(lc) + "-small.avif", nil
	default:
		return "", errBadType
	}
}

// isRaceID accepts "race-gender" strings like "1-0", refusing anything with
// path separators or whitespace.
func isRaceID(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, r := range s {
		if !(r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

var (
	errBadID   = httpErr{code: http.StatusBadRequest, msg: "bad id"}
	errBadType = httpErr{code: http.StatusBadRequest, msg: "unknown type"}
)

type httpErr struct {
	code int
	msg  string
}

func (e httpErr) Error() string { return e.msg }

func Mount(mux *http.ServeMux, deps Deps) {
	mux.Handle("GET /img/icon", Handler(deps))
}
