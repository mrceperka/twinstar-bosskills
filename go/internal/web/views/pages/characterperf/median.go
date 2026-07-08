package characterperf

import (
	"context"
	"database/sql"
	"strings"

	"twinstar-bosskills/internal/metric"
)

// MedianPair holds p50 DPS and p50 HPS for a (boss, mode) combination.
type MedianPair struct {
	DPS int64
	HPS int64
}

// loadMedianByBoss returns p50 DPS and p50 HPS per (boss_remote_id, mode),
// optionally filtered by modes. Computes exact percentiles directly from boss_kill.
func loadMedianByBoss(ctx context.Context, db *sql.DB, realmName string, bossIDs []uint32, filter FilterValues) (
	map[uint32]map[int]MedianPair, error,
) {
	out := map[uint32]map[int]MedianPair{}
	if len(bossIDs) == 0 || len(filter.Specs) != 1 {
		return out, nil
	}

	args := make([]any, 0, len(bossIDs)+len(filter.Modes)+4)
	args = append(args, realmName)
	bossPH := make([]string, len(bossIDs))
	for i, id := range bossIDs {
		bossPH[i] = "?"
		args = append(args, id)
	}
	q := "SELECT boss_remote_id, mode, " +
		"quantileExact(0.5)(" + metric.SQLFloat64(metric.DmgDoneArrayJoin) + ") AS p50_dps, " +
		"quantileExact(0.5)(" + metric.SQLFloat64(metric.HealAbsorbArrayJoin) + ") AS p50_hps " +
		"FROM boss_kill ARRAY JOIN players " +
		"WHERE realm = ? AND boss_remote_id IN (" + strings.Join(bossPH, ",") + ") AND length > 0 AND players.talent_spec = ?"
	args = append(args, uint16(filter.Specs[0]))

	if len(filter.Modes) > 0 {
		modePH := make([]string, len(filter.Modes))
		for i, m := range filter.Modes {
			modePH[i] = "?"
			args = append(args, uint8(m))
		}
		q += " AND mode IN (" + strings.Join(modePH, ",") + ")"
	}
	if filter.IlvlMin > 0 {
		q += " AND toFloat32(players.avg_item_lvl) >= ?"
		args = append(args, float32(filter.IlvlMin))
	}
	if filter.IlvlMax > 0 {
		q += " AND toFloat32(players.avg_item_lvl) <= ?"
		args = append(args, float32(filter.IlvlMax))
	}
	q += " GROUP BY boss_remote_id, mode"

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uint32
		var mode uint8
		var pDPS, pHPS float64
		if err := rows.Scan(&id, &mode, &pDPS, &pHPS); err != nil {
			return nil, err
		}
		if out[id] == nil {
			out[id] = map[int]MedianPair{}
		}
		out[id][int(mode)] = MedianPair{
			DPS: int64(pDPS),
			HPS: int64(pHPS),
		}
	}
	return out, rows.Err()
}

func keysU32(m map[uint32]bool) []uint32 {
	out := make([]uint32, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
