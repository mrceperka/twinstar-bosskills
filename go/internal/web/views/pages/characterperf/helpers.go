package characterperf

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
)

func profileHref(realmName, name string) templ.SafeURL {
	return templ.SafeURL(links.Character(realmName, name))
}

func resetHref(realmName, name string) templ.SafeURL {
	return templ.SafeURL(links.CharacterPerformance(realmName, name))
}

func bossHref(realmName string, bossID uint32, mode int) templ.SafeURL {
	return templ.SafeURL(links.Boss(realmName, bossID) + "?mode=" + strconv.Itoa(mode))
}
