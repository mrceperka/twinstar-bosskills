// Package format holds presentation helpers shared by the page templates.
//
// Number formatting is locale-aware: the user's locale comes from the
// Accept-Language header (parsed by FromAcceptLanguage), is attached to
// ctx by middleware.AttachLocale, and read out by IntCtx / Int64Ctx.
package format

import (
	"context"
	"strconv"
)

// Int formats v with locale-specific thousand grouping.
func Int(loc Locale, v int) string { return Int64(loc, int64(v)) }

// Int64 is the int64 variant.
func Int64(loc Locale, v int64) string {
	s := strconv.FormatInt(v, 10)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	n := len(s)
	if n <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	group := loc.Group
	if group == "" {
		group = ","
	}
	var b []byte
	first := n % 3
	if first == 0 {
		first = 3
	}
	b = append(b, s[:first]...)
	for i := first; i < n; i += 3 {
		b = append(b, group...)
		b = append(b, s[i:i+3]...)
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// IntCtx is the context-aware shortcut used inside templ files (where ctx
// is always in scope).
func IntCtx(ctx context.Context, v int) string {
	return Int(LocaleFromContext(ctx), v)
}

// Int64Ctx is the int64 variant.
func Int64Ctx(ctx context.Context, v int64) string {
	return Int64(LocaleFromContext(ctx), v)
}

// Duration formats a number of seconds as "m:ss" (or "h:mm:ss" for ≥1 hour).
// Returns "-" for non-positive durations. Locale-neutral.
func Duration(seconds int) string {
	if seconds <= 0 {
		return "-"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return strconv.Itoa(h) + ":" + pad2(m) + ":" + pad2(s)
	}
	return strconv.Itoa(m) + ":" + pad2(s)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
