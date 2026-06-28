package guildtoken

import "github.com/a-h/templ"

func generateHref(realmName string) templ.SafeURL {
	return templ.SafeURL("/" + realmName + "/guild-token/generate")
}

func adminPlaceholder(hasAdmin bool) string {
	if hasAdmin {
		return "(already verified, reuse)"
	}
	return ""
}

func maskToken(t string) string {
	if len(t) <= 8 {
		return "********"
	}
	return t[:4] + "…" + t[len(t)-4:]
}
