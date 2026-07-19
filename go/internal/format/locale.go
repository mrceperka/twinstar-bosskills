package format

import (
	"context"
	"strings"
)

// Locale holds the punctuation a number should use for the active user
// locale. Only group / decimal characters are tracked - neither affects
// units, currency, or word ordering.
type Locale struct {
	// Group is the thousand separator ("," in en-US, " " in fr/cs, "." in de).
	Group string
	// Decimal is the radix character ("." in en-US, "," in most of Europe).
	Decimal string
}

// Common locales. Add more as needed; FromAcceptLanguage falls back to
// EN when nothing matches.
var (
	EN = Locale{Group: ",", Decimal: "."}
	CS = Locale{Group: " ", Decimal: ","} // non-breaking space groups in cs-CZ
	DE = Locale{Group: ".", Decimal: ","}
	FR = Locale{Group: " ", Decimal: ","}
	NL = Locale{Group: ".", Decimal: ","}
	IT = Locale{Group: ".", Decimal: ","}
	ES = Locale{Group: ".", Decimal: ","}
	PL = Locale{Group: " ", Decimal: ","}
	SK = Locale{Group: " ", Decimal: ","}
)

// FromAcceptLanguage picks the best-matching Locale from the request header.
// Parses just the first sub-tag (language code) to keep the lookup small.
// Quality values (`;q=`) are honoured: tags are scanned in header order, so
// browsers' high-q preferences win.
func FromAcceptLanguage(header string) Locale {
	for _, part := range strings.Split(header, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if i := strings.IndexByte(tag, '-'); i > 0 {
			tag = tag[:i]
		}
		switch tag {
		case "cs":
			return CS
		case "de":
			return DE
		case "fr":
			return FR
		case "nl":
			return NL
		case "it":
			return IT
		case "es":
			return ES
		case "pl":
			return PL
		case "sk":
			return SK
		case "en":
			return EN
		}
	}
	return EN
}

// ctxKey is unexported so other packages can't accidentally collide.
type ctxKey struct{}

// WithLocale returns a derived context that carries loc.
func WithLocale(ctx context.Context, loc Locale) context.Context {
	return context.WithValue(ctx, ctxKey{}, loc)
}

// LocaleFromContext returns the locale attached by WithLocale, or EN if none.
func LocaleFromContext(ctx context.Context) Locale {
	if v, ok := ctx.Value(ctxKey{}).(Locale); ok {
		return v
	}
	return EN
}
