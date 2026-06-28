package characters

import (
	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/links"
)

func formAction(realmName string) templ.SafeURL {
	return templ.SafeURL(links.Characters(realmName))
}

func characterHref(realmName, name string) templ.SafeURL {
	return templ.SafeURL(links.Character(realmName, name))
}
