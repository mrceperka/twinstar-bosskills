package sqlutil

import "strings"

func Placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

// GuildFilter returns the optional boss_kill guild predicate and its argument.
func GuildFilter(guild string) (string, []any) {
	if guild == "" {
		return "", nil
	}
	return " AND guild = ?", []any{guild}
}
