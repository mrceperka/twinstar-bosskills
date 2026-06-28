// Package cache holds in-process + on-disk cache backends.
//
// Two distinct caches live here so they can be tuned independently:
//
//   - icons.Disk        — large, binary, persists across restarts
//   - (future) memory   — small, fast, query-result cache
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// IconDisk caches icon blobs on local disk. One blob = two files:
//
//	<sha256>.bin   — raw bytes
//	<sha256>.meta  — Content-Type string
//
// Cache keys are derived from the upstream URL so the same icon shared across
// realms is stored once.
type IconDisk struct {
	Dir  string
	HTTP *http.Client
}

func NewIconDisk(dir string) (*IconDisk, error) {
	if dir == "" {
		dir = "./var/icons"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return &IconDisk{
		Dir:  dir,
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// Blob is one cached icon ready to be served.
type Blob struct {
	Data        []byte
	ContentType string
	Hit         bool
}

// Get returns the blob, either from disk or by fetching upstream and caching.
// If the upstream fetch returns a non-allowed content type, the function
// returns (nil, nil) — the caller should serve a placeholder.
func (c *IconDisk) Get(ctx context.Context, upstreamURL string) (*Blob, error) {
	key := hashKey(upstreamURL)
	dataPath := filepath.Join(c.Dir, key+".bin")
	metaPath := filepath.Join(c.Dir, key+".meta")

	if data, err := os.ReadFile(dataPath); err == nil {
		ct, _ := os.ReadFile(metaPath)
		return &Blob{Data: data, ContentType: string(ct), Hit: true}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", dataPath, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "twinstar-bosskills-go/0.1 (icons)")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", upstreamURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("upstream %s: status %d", upstreamURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5MB cap per icon
	if err != nil {
		return nil, err
	}
	ct := resp.Header.Get("Content-Type")
	if !allowedContentType(ct) {
		return nil, nil
	}

	if err := os.WriteFile(dataPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", dataPath, err)
	}
	if err := os.WriteFile(metaPath, []byte(ct), 0o644); err != nil {
		// Best-effort: data already wrote; meta is recoverable.
		_ = err
	}

	return &Blob{Data: data, ContentType: ct, Hit: false}, nil
}

func hashKey(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:32]
}

func allowedContentType(ct string) bool {
	switch {
	case ct == "image/png",
		ct == "image/jpeg",
		ct == "image/webp",
		ct == "image/avif",
		ct == "image/svg+xml",
		ct == "application/octet-stream":
		return true
	}
	return false
}
