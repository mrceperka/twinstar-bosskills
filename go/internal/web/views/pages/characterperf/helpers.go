package characterperf

import (
	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func resetHref(realmName, name string) templ.SafeURL {
	return templ.SafeURL(links.CharacterPerformance(realmName, name))
}

var (
	profileHref       = viewhelpers.CharacterHref
	bossHref          = viewhelpers.BossWithDifficultyHref
	itoa              = viewhelpers.EmptyItoa
	selectFilterClass = viewhelpers.SelectFilterClass
	armoryHref        = links.TwinstarArmory
)
