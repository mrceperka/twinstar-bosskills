package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Item is the trimmed shape used by the bosskill detail page.
type Item struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Quality int    `json:"quality"`
	// Tooltip is the HTML body served by /item/tooltip - used by the
	// bosskill detail page to render rich-hover popovers.
	Tooltip string `json:"tooltip,omitempty"`
}

// rawItem mirrors the upstream /item/{id} response.
type rawItem struct {
	Item       struct{ ID int } `json:"item"`
	ItemSparse struct {
		Name    string `json:"Name"`
		Quality int    `json:"Quality"`
	} `json:"itemSparse"`
}

// ErrItemNotFound signals a 404 or unparseable upstream response.
var ErrItemNotFound = errors.New("item not found")

// rawTooltip mirrors /item/tooltip's `{data: {tooltip: "..."}}` shape.
type rawTooltip struct {
	Data struct {
		Tooltip string `json:"tooltip"`
	} `json:"data"`
}

// GetItemTooltip returns a sanitized upstream HTML tooltip body. Empty string
// + nil error means upstream had no tooltip (e.g. removed item).
func (c *Client) GetItemTooltip(ctx context.Context, id, expansion int) (string, error) {
	if id <= 0 {
		return "", ErrItemNotFound
	}
	u := c.BaseURL + "/item/tooltip?id=" + strconv.Itoa(id) +
		"&expansion=" + strconv.Itoa(expansion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}
	var raw rawTooltip
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", err
	}
	return SanitizeTooltipHTML(raw.Data.Tooltip), nil
}

// GetItem fetches one item by ID.
func (c *Client) GetItem(ctx context.Context, id int) (*Item, error) {
	if id <= 0 {
		return nil, ErrItemNotFound
	}
	u := c.BaseURL + "/item/" + strconv.Itoa(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrItemNotFound
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("GET %s: status %d: %s", u, resp.StatusCode, string(body))
	}
	var raw rawItem
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode %s: %w", u, err)
	}
	if raw.Item.ID == 0 && raw.ItemSparse.Name == "" {
		return nil, ErrItemNotFound
	}
	return &Item{
		ID:      raw.Item.ID,
		Name:    raw.ItemSparse.Name,
		Quality: raw.ItemSparse.Quality,
	}, nil
}

// QualityColor returns a hex color matching WoW item rarity.
func QualityColor(q int) string {
	switch q {
	case 0:
		return "#9d9d9d" // Poor
	case 1:
		return "#ffffff" // Common
	case 2:
		return "#1eff00" // Uncommon
	case 3:
		return "#0070dd" // Rare
	case 4:
		return "#a335ee" // Epic
	case 5:
		return "#ff8000" // Legendary
	case 6:
		return "#e6cc80" // Artifact
	case 7:
		return "#00ccff" // Heirloom
	}
	return "#d8dde6"
}
