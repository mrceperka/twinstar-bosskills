package dashboard

import (
	"github.com/a-h/templ"
	"github.com/mrceperka/twinstar-bosskills/go/internal/web/views/viewhelpers"
)

func realmHref(realmName, suffix string) templ.SafeURL {
	return templ.SafeURL("/" + realmName + suffix)
}

var formatFloat = viewhelpers.FormatFloat
