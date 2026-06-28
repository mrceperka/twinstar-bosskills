package ch

import "strings"

// SplitStatements splits a SQL string into top-level statements separated by
// semicolons. It ignores semicolons inside:
//   - single-quoted strings  ('...')
//   - double-quoted identifiers ("...")
//   - backtick identifiers     (`...`)
//   - line comments            (-- to end-of-line)
//   - block comments           (/* ... */)
//
// Empty / whitespace-only statements are dropped.
func SplitStatements(s string) []string {
	var out []string
	var buf strings.Builder

	const (
		stNormal = iota
		stSingle
		stDouble
		stBacktick
		stLineComment
		stBlockComment
	)
	state := stNormal

	flush := func() {
		stmt := strings.TrimSpace(buf.String())
		if stmt != "" {
			out = append(out, stmt)
		}
		buf.Reset()
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		switch state {
		case stNormal:
			switch {
			case c == '\'':
				state = stSingle
				buf.WriteRune(c)
			case c == '"':
				state = stDouble
				buf.WriteRune(c)
			case c == '`':
				state = stBacktick
				buf.WriteRune(c)
			case c == '-' && next == '-':
				state = stLineComment
				i++ // consume second '-'
			case c == '/' && next == '*':
				state = stBlockComment
				i++
			case c == ';':
				flush()
			default:
				buf.WriteRune(c)
			}
		case stSingle:
			buf.WriteRune(c)
			if c == '\\' && next != 0 {
				buf.WriteRune(next)
				i++
				continue
			}
			if c == '\'' {
				state = stNormal
			}
		case stDouble:
			buf.WriteRune(c)
			if c == '"' {
				state = stNormal
			}
		case stBacktick:
			buf.WriteRune(c)
			if c == '`' {
				state = stNormal
			}
		case stLineComment:
			if c == '\n' {
				state = stNormal
				buf.WriteRune(c)
			}
		case stBlockComment:
			if c == '*' && next == '/' {
				state = stNormal
				i++
			}
		}
	}
	flush()
	return out
}
