package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const DefaultBaseURL = "https://twinstar-api.twinstar-wow.com"

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) getJSON(ctx context.Context, path string, query string, out any) error {
	u := c.BaseURL + path
	if query != "" {
		u += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "twinstar-bosskills-go/0.1")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("GET %s: status %d: %s", u, resp.StatusCode, string(body))
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	return dec.Decode(out)
}

func (c *Client) getRawJSON(ctx context.Context, path string, query string) ([]byte, error) {
	u := c.BaseURL + path
	if query != "" {
		u += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "twinstar-bosskills-go/0.1")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("GET %s: status %d: %s", u, resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// GetRaids ports packages/api/src/raid.ts:getRaids, including the boss
// filter/rename fixups for MoP, ToT, ToES, SoO, and Cata raids.
func (c *Client) GetRaids(ctx context.Context, realmName string, expansion int) ([]Raid, error) {
	q := url.Values{}
	q.Set("expansion", fmt.Sprintf("%d", expansion))
	var raids []Raid
	if err := c.getJSON(ctx, "/bosskills/raids", q.Encode(), &raids); err != nil {
		return nil, err
	}
	for i := range raids {
		raids[i].Bosses = sanitizeBosses(raids[i].Bosses)
		raids[i].Bosses = renameBosses(raids[i].Bosses)
	}
	if realmName == "Kronos" {
		raids = filterKronosVanillaRaids(raids)
	}
	return raids, nil
}

// GetLatestBossKills calls /bosskills?sorter=time desc. Page is 0-indexed.
func (c *Client) GetLatestBossKills(ctx context.Context, q Query) (PaginatedBossKills, error) {
	if q.Sorter == nil {
		q.Sorter = &Sorter{Column: "time", Order: "desc"}
	}
	var out PaginatedBossKills
	if err := c.getJSON(ctx, "/bosskills", q.Encode(), &out); err != nil {
		return PaginatedBossKills{}, err
	}
	return out, nil
}

// ListAllLatestBossKills pages through /bosskills, mirroring listAll in
// packages/api/src/pagination.ts. Fires concurrent requests after the first
// page returns total/pageSize.
func (c *Client) ListAllLatestBossKills(ctx context.Context, q Query, concurrency int) ([]BossKill, error) {
	if concurrency < 1 {
		concurrency = 4
	}
	if q.PageSize <= 0 {
		q.PageSize = 50
	}

	first := q
	first.Page = 0
	page0, err := c.GetLatestBossKills(ctx, first)
	if err != nil {
		return nil, err
	}
	items := append([]BossKill(nil), page0.Data...)
	pageSize := q.PageSize
	if len(page0.Data) > 0 && len(page0.Data) < pageSize {
		pageSize = len(page0.Data)
	}
	if page0.Total <= pageSize {
		return items, nil
	}
	totalPages := (page0.Total + pageSize - 1) / pageSize

	type job struct {
		page int
		out  []BossKill
		err  error
	}
	jobs := make(chan int)
	results := make(chan job)
	go func() {
		for p := 1; p <= totalPages; p++ {
			jobs <- p
		}
		close(jobs)
	}()
	workers := concurrency
	if workers > totalPages {
		workers = totalPages
	}
	for w := 0; w < workers; w++ {
		go func() {
			for p := range jobs {
				qq := q
				qq.Page = p
				qq.PageSize = pageSize
				res, err := c.GetLatestBossKills(ctx, qq)
				results <- job{page: p, out: res.Data, err: err}
			}
		}()
	}
	for i := 0; i < totalPages; i++ {
		r := <-results
		if r.err != nil {
			return nil, fmt.Errorf("page %d: %w", r.page, r.err)
		}
		items = append(items, r.out...)
	}
	return items, nil
}

// GetBossKillDetail returns nil if the upstream replies with no record.
func (c *Client) GetBossKillDetail(ctx context.Context, realmName, id string) (*BossKillDetail, error) {
	q := url.Values{}
	q.Set("realm", realmName)
	var out BossKillDetail
	if err := c.getJSON(ctx, "/bosskills/"+url.PathEscape(id), q.Encode(), &out); err != nil {
		return nil, err
	}
	// Treat empty ID as "no result" — upstream sometimes returns {}.
	if out.ID == "" {
		return nil, nil
	}
	return &out, nil
}

func (c *Client) GetCharacterActivityFeed(ctx context.Context, realmName, characterName string, page, pageSize int) (PaginatedCharacterActivityFeed, error) {
	q := url.Values{}
	q.Set("realm", realmName)
	q.Set("name", characterName)
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
	var out PaginatedCharacterActivityFeed
	if err := c.getJSON(ctx, "/character/activity-feed", q.Encode(), &out); err != nil {
		return PaginatedCharacterActivityFeed{}, err
	}
	return out, nil
}

func (c *Client) GetCharacterStats(ctx context.Context, realmName, characterName string) (CharacterStatsPayload, error) {
	q := url.Values{}
	q.Set("realm", realmName)
	q.Set("name", characterName)
	raw, err := c.getRawJSON(ctx, "/character/stats", q.Encode())
	if err != nil {
		return CharacterStatsPayload{}, err
	}
	var stats CharacterStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		return CharacterStatsPayload{}, err
	}
	return CharacterStatsPayload{Raw: raw, Stats: stats}, nil
}
