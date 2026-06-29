package characters

import (
	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func formAction(realmName string) templ.SafeURL {
	return templ.SafeURL(links.Characters(realmName))
}

var characterHref = viewhelpers.CharacterHref
