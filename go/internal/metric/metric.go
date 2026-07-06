// Package metric centralizes the DPS/HPS rate formula used both in Go
// aggregation code and in the ClickHouse queries.
//
// The rate is always `value * 1000 / lengthMs` because `length` in the schema
// is stored in milliseconds. HPS treats absorbs and heals interchangeably —
// they are added before dividing.
package metric

// Player-field accessors for CH queries.
//
// ARRAYJOIN variants read the array element bound by `ARRAY JOIN players`.
// Indexed variants take a bound `idx` alias (`WITH indexOf(players.guid, ?) AS idx`)
// so a single player's stat can be selected without unnesting.
const (
	DmgDoneArrayJoin    = "players.dmg_done"
	HealAbsorbArrayJoin = "(players.healing_done + players.absorb_done)"
	DmgDoneIndexed      = "players.dmg_done[idx]"
	HealAbsorbIndexed   = "(players.healing_done[idx] + players.absorb_done[idx])"
)

// SQLUInt64 builds a rate expression that truncates to a uint64. Use for
// per-row rates that feed into `max` / `argMax` and similar aggregates where
// integer arithmetic is fine.
//
//	metric.SQLUInt64(metric.DmgDoneArrayJoin)
//	  -> "toUInt64(players.dmg_done * 1000 / greatest(length, 1))"
func SQLUInt64(fieldExpr string) string {
	return "toUInt64(" + fieldExpr + " * 1000 / greatest(length, 1))"
}

// SQLFloat64 builds a rate expression that stays in float64. Use when the
// value feeds into `quantile*` / `quantilesExact` — integer truncation would
// bias the sample.
//
//	metric.SQLFloat64(metric.DmgDoneArrayJoin)
//	  -> "toFloat64(players.dmg_done) * 1000 / greatest(length, 1)"
func SQLFloat64(fieldExpr string) string {
	return "toFloat64(" + fieldExpr + ") * 1000 / greatest(length, 1)"
}

// DPS returns damage-per-second given total damage and fight length in
// milliseconds. Returns 0 if lengthMs is not positive.
func DPS(dmgDone, lengthMs int64) int64 {
	if lengthMs <= 0 {
		return 0
	}
	return dmgDone * 1000 / lengthMs
}

// HPS returns healing-per-second (healing + absorb) given raw values and
// fight length in milliseconds. Returns 0 if lengthMs is not positive.
func HPS(healingDone, absorbDone, lengthMs int64) int64 {
	if lengthMs <= 0 {
		return 0
	}
	return (healingDone + absorbDone) * 1000 / lengthMs
}
