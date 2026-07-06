// Package realm mirrors packages/core/src/realm.ts.
//
// Realm strings are case-sensitive in the Twinstar API but the app routes
// accept lowercase too. Normalize before any lookup.
package realm

import "strings"

const (
	Kronos         = "Kronos"
	Helios         = "Helios"
	Athena         = "Athena"
	Apollo         = "Apollo"
	CataProudmoore = "Proudmoore"
	MoPPrivatePvE  = "MoPPvE"
)

const (
	IDKronos        = 4
	IDHelios        = 18
	IDApollo        = 9
	IDAthena        = 19
	IDCataPrivate   = 21
	IDMoPPrivatePvE = 24
)

// Expansion codes mirror the TS constants: 0 vanilla, 3 cata, 4 mop.
const (
	ExpansionVanilla = 0
	ExpansionCata    = 3
	ExpansionMoP     = 4
)

var canonical = map[string]string{
	strings.ToLower(Kronos):         Kronos,
	strings.ToLower(Helios):         Helios,
	strings.ToLower(Athena):         Athena,
	strings.ToLower(Apollo):         Apollo,
	strings.ToLower(CataProudmoore): CataProudmoore,
	strings.ToLower(MoPPrivatePvE):  MoPPrivatePvE,
}

var privateRealms = map[string]bool{
	strings.ToLower(MoPPrivatePvE): true,
}

var toExpansion = map[string]int{
	Kronos:         ExpansionVanilla,
	Helios:         ExpansionMoP,
	Athena:         ExpansionCata,
	Apollo:         ExpansionCata,
	CataProudmoore: ExpansionCata,
	MoPPrivatePvE:  ExpansionMoP,
}

var toID = map[string]int{
	Helios:         IDHelios,
	Athena:         IDAthena,
	Apollo:         IDApollo,
	CataProudmoore: IDCataPrivate,
	MoPPrivatePvE:  IDMoPPrivatePvE,
	Kronos:         IDKronos,
}

var mergedTo = map[string]string{
	Apollo: Athena,
}

// Normalize returns the canonical case form, or Helios as a fallback.
func Normalize(r string) string {
	if v, ok := canonical[strings.ToLower(r)]; ok {
		return v
	}
	return Helios
}

// MergedTo returns the canonical realm a given realm has been merged into,
// or empty string if it hasn't been merged.
func MergedTo(r string) string {
	return mergedTo[Normalize(r)]
}

func IsPublic(r string) bool {
	return !privateRealms[strings.ToLower(r)]
}

func IsKnown(r string) bool {
	_, ok := canonical[strings.ToLower(r)]
	return ok
}

func Expansion(r string) int {
	return toExpansion[Normalize(r)]
}

func ID(r string) int {
	return toID[Normalize(r)]
}

func IsCata(expansion int) bool    { return expansion == ExpansionCata }
func IsMoP(expansion int) bool     { return expansion == ExpansionMoP }
func IsVanilla(expansion int) bool { return expansion == ExpansionVanilla }

// All canonical realms in a stable order (handy for CLI defaults).
func All() []string {
	return []string{Helios, Athena, Apollo, CataProudmoore, MoPPrivatePvE, Kronos}
}
