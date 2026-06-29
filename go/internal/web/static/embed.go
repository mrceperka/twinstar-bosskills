// Package static serves built assets via embed.FS.
package static

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
)

//go:embed app.css htmx.min.js echarts.min.js bk-chart.js bk-ui.js logos/32x32/*.png
var assetsFS embed.FS

// FS exposes the embedded files for http.FileServer.
func FS() fs.FS {
	return assetsFS
}

// Handler returns an http.Handler rooted at the embedded asset directory.
// Mount under /static/, e.g. mux.Handle("GET /static/", http.StripPrefix("/static", static.Handler())).
func Handler() http.Handler {
	return http.FileServer(http.FS(assetsFS))
}

// Hash returns a short content hash for the given asset name. Used for
// cache-busting URL query params.
func Hash(name string) string {
	b, err := assetsFS.ReadFile(name)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:8]
}
