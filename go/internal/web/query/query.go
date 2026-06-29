package query

import (
	"net/url"
	"strconv"
)

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
