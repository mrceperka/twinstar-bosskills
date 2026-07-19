package query

import (
	"net/url"
	"strconv"
	"strings"
)

func IntOr(raw string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return n
}

func First(values map[string][]string, name string) string {
	if items := values[name]; len(items) > 0 {
		return items[0]
	}
	return ""
}

// Int returns the first valid integer found under any of names.
func Int(values url.Values, min int, names ...string) (int, bool) {
	for _, name := range names {
		raw := values.Get(name)
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err == nil && n >= min {
			return n, true
		}
	}
	return 0, false
}

func Difficulty(values url.Values) (int, bool) {
	return Int(values, 0, "difficulty", "mode")
}

func RaidLock(values url.Values) (int, bool) {
	return Int(values, 0, "raidlock", "offset")
}
