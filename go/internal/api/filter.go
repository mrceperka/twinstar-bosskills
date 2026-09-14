package api

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

type FilterOperator string

const (
	OpEquals FilterOperator = "equals"
	OpILike  FilterOperator = "ilike"
	OpIn     FilterOperator = "in"
	OpGT     FilterOperator = "gt"
	OpGTE    FilterOperator = "gte"
	OpLT     FilterOperator = "lt"
	OpLTE    FilterOperator = "lte"
)

// Filter is JSON-marshalled into the `filters` query parameter.
type Filter struct {
	Column   string         `json:"column"`
	Value    any            `json:"value"`
	Operator FilterOperator `json:"operator"`
}

type Sorter struct {
	Column string `json:"column"`
	Order  string `json:"order"` // "asc" | "desc"
}

// Query mirrors packages/api/src/filter.ts:QueryArgs.
type Query struct {
	Realm      string
	Map        string
	GUID       int64 // 0 = unset
	Name       string
	Difficulty *int // pointer so we can distinguish "0" from "unset"
	TalentSpec *int
	Page       int
	PageSize   int
	Filters    []Filter
	Sorter     *Sorter
}

// Encode renders Query into a URL query string compatible with the TS client.
func (q Query) Encode() string {
	v := url.Values{}

	realm := q.Realm
	if realm == "" {
		realm = "Helios"
	}
	// upstream expects the first letter uppercased; we already canonicalise upstream
	if len(realm) > 0 {
		realm = strings.ToUpper(realm[:1]) + realm[1:]
	}
	v.Set("realm", realm)

	if q.Difficulty != nil {
		v.Set("mode", strconv.Itoa(*q.Difficulty))
	}
	if q.Map != "" {
		v.Set("map", q.Map)
	}
	if q.GUID != 0 {
		v.Set("guid", strconv.FormatInt(q.GUID, 10))
	}
	if q.Name != "" {
		v.Set("name", q.Name)
	}
	if q.TalentSpec != nil {
		v.Set("talent_spec", strconv.Itoa(*q.TalentSpec))
	}
	// page=0 is meaningful (the first page), so always set when PageSize is set or Page > 0
	v.Set("page", strconv.Itoa(q.Page))
	if q.PageSize > 0 {
		v.Set("pageSize", strconv.Itoa(q.PageSize))
	} else {
		v.Set("pageSize", "100")
	}
	if len(q.Filters) > 0 {
		b, _ := json.Marshal(q.Filters)
		v.Set("filters", string(b))
	}
	if q.Sorter != nil {
		b, _ := json.Marshal(q.Sorter)
		v.Set("sorter", string(b))
	}
	return v.Encode()
}
