package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"twinstar-bosskills/internal/api"
)

// ItemDisk caches /item/{id} responses on disk as JSON. One file per item.
//
// Memoizes in-flight lookups so the bosskill detail page (which can request
// 20+ items in one render) only fires a single upstream call per unique ID
// even under concurrent rendering.
type ItemDisk struct {
	Dir       string
	API       *api.Client
	Expansion int // when >0, GetItemTooltip is also fetched on cache miss

	mu       sync.Mutex
	inflight map[int]chan struct{}
	results  map[int]itemResult
}

type itemResult struct {
	item *api.Item
	err  error
}

func NewItemDisk(dir string, cli *api.Client) (*ItemDisk, error) {
	if dir == "" {
		dir = "./var/items"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return &ItemDisk{
		Dir:      dir,
		API:      cli,
		inflight: map[int]chan struct{}{},
		results:  map[int]itemResult{},
	}, nil
}

// Get returns the cached item or fetches it. Concurrent calls for the same ID
// block on a single in-flight request.
func (c *ItemDisk) Get(ctx context.Context, id int) (*api.Item, error) {
	if it, ok := c.readDisk(id); ok {
		// Backfill tooltip for cache entries written before tooltip support.
		// One-shot best-effort: failure is non-fatal and we still return the
		// cached item.
		if c.Expansion > 0 && it.Tooltip == "" {
			if tip, terr := c.API.GetItemTooltip(ctx, id, c.Expansion); terr == nil && tip != "" {
				it.Tooltip = tip
				c.writeDisk(id, it)
			}
		}
		return it, nil
	}

	c.mu.Lock()
	if ch, ok := c.inflight[id]; ok {
		c.mu.Unlock()
		<-ch
		c.mu.Lock()
		res := c.results[id]
		c.mu.Unlock()
		return res.item, res.err
	}
	ch := make(chan struct{})
	c.inflight[id] = ch
	c.mu.Unlock()

	it, err := c.API.GetItem(ctx, id)
	if err == nil && it != nil && c.Expansion > 0 {
		// Tooltip is part of the same cache entry so concurrent callers
		// only do one upstream round trip per item.
		if tip, terr := c.API.GetItemTooltip(ctx, id, c.Expansion); terr == nil && tip != "" {
			it.Tooltip = tip
		}
	}

	c.mu.Lock()
	c.results[id] = itemResult{item: it, err: err}
	delete(c.inflight, id)
	close(ch)
	c.mu.Unlock()

	if err == nil && it != nil {
		c.writeDisk(id, it)
	}
	return it, err
}

func (c *ItemDisk) path(id int) string {
	return filepath.Join(c.Dir, strconv.Itoa(id)+".json")
}

func (c *ItemDisk) readDisk(id int) (*api.Item, bool) {
	data, err := os.ReadFile(c.path(id))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			// Surface IO errors silently — caller will refetch.
		}
		return nil, false
	}
	var it api.Item
	if err := json.Unmarshal(data, &it); err != nil {
		return nil, false
	}
	return &it, true
}

func (c *ItemDisk) writeDisk(id int, it *api.Item) {
	data, err := json.Marshal(it)
	if err != nil {
		return
	}
	_ = os.WriteFile(c.path(id), data, 0o644)
}

// GetMany resolves a batch of items in parallel, populating a map keyed by
// item ID. Missing items (not found / errors) are simply omitted.
func (c *ItemDisk) GetMany(ctx context.Context, ids []int) map[int]*api.Item {
	out := map[int]*api.Item{}
	if len(ids) == 0 {
		return out
	}
	type res struct {
		id   int
		item *api.Item
	}
	results := make(chan res, len(ids))
	for _, id := range ids {
		go func(id int) {
			it, _ := c.Get(ctx, id)
			results <- res{id: id, item: it}
		}(id)
	}
	for range ids {
		r := <-results
		if r.item != nil {
			out[r.id] = r.item
		}
	}
	return out
}
