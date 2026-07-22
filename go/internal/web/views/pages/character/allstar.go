package character

import (
	"context"
	"database/sql"
	"math"
	"net/url"
	"sort"
	"strconv"
	"time"

	"twinstar-bosskills/internal/links"
	"twinstar-bosskills/internal/web/sqlutil"
	"twinstar-bosskills/internal/wow"
)

// All-Star scoring, modelled on Warcraft Logs' character all-star points.
//
// Per boss:   ratio = min(1, myValue/rank1Value)
//             pct   = 100 * (1 - (rank-1)/N)         -- percentile floor
//             points = max(100*ratio, pct) + 20*ratio
// A rank-1 parse scores 120. Zero value or empty pool scores 0.
//
// A character's score is the sum of the best per-boss points across the bosses
// of a single raid, split by difficulty (Normal/Heroic never mix) and computed
// separately for DPS and HPS. Raid and difficulty are both selectable. Raid
// defaults to the last active raid (falling back to the most active when it has
// no ranking data); difficulty defaults to the one with the most ranked bosses.

const allStarBase = 100.0

// AllStarScore is the aggregated result for one metric split.
type AllStarScore struct {
	DPSPoints float64
	HPSPoints float64
	DPSBosses int
	HPSBosses int
}

// raidActivity is one raid the character has kills in.
type raidActivity struct {
	Name      string
	KillCount int
	LastKill  time.Time
}

// AllStarRaidOption is one selectable raid in the badge.
type AllStarRaidOption struct {
	Name     string
	Selected bool
	Href     string
}

// AllStarDiffOption is one selectable difficulty in the badge.
type AllStarDiffOption struct {
	Mode     int
	Label    string
	Selected bool
	Href     string
}

// AllStarViewModel is the header badge showing the character's all-star score
// for the selected raid + difficulty, plus the raid and difficulty selectors.
type AllStarViewModel struct {
	Realm           string
	CharName        string
	RaidName        string
	Mode            int
	DifficultyLabel string
	Raids           []AllStarRaidOption
	Difficulties    []AllStarDiffOption
	HasDPS          bool
	DPSPoints       int
	DPSBosses       int
	DPSAvg          float64
	DPSGrade        string
	DPSGradeClass   string
	HasHPS          bool
	HPSPoints       int
	HPSBosses       int
	HPSAvg          float64
	HPSGrade        string
	HPSGradeClass   string
}

// allStarGrade buckets an average points-per-boss (0..120) into a medal tier.
// Cutoffs are provisional until calibrated against the real score distribution.
func allStarGrade(avg float64) (label, class string) {
	switch {
	case avg >= 95:
		return "Gold", "text-yellow-400"
	case avg >= 75:
		return "Silver", "text-slate-300"
	case avg > 0:
		return "Bronze", "text-orange-300"
	default:
		return "", ""
	}
}

// loadAllStar computes the character's all-star score for the requested raid.
// When selectedRaid is empty it defaults to the last active raid, falling back
// to the most active raid if the last active one scores nothing. Returns a
// zero-value (empty) viewmodel when the character has no ranked kills.
func loadAllStar(ctx context.Context, db *sql.DB, realmName, charName string, guid uint64, expansion int, selectedRaid string, selectedMode int) (AllStarViewModel, error) {
	raids, err := loadCharacterRaids(ctx, db, realmName, guid)
	if err != nil {
		return AllStarViewModel{}, err
	}
	if len(raids) == 0 {
		return AllStarViewModel{}, nil
	}

	known := map[string]bool{}
	for _, r := range raids {
		known[r.Name] = true
	}

	usingDefault := selectedRaid == "" || !known[selectedRaid]
	raidName := selectedRaid
	if usingDefault {
		raidName = raidByLastKill(raids)
	}

	scores, err := scoreRaid(ctx, db, realmName, guid, raidName)
	if err != nil {
		return AllStarViewModel{}, err
	}
	// When falling back to the default raid and it has no ranking data, try the
	// most active raid instead.
	if usingDefault && len(scores) == 0 {
		if mostActive := raidByKillCount(raids); mostActive != raidName {
			raidName = mostActive
			scores, err = scoreRaid(ctx, db, realmName, guid, raidName)
			if err != nil {
				return AllStarViewModel{}, err
			}
		}
	}
	return buildAllStarViewModel(realmName, charName, expansion, raidName, raids, scores, selectedMode), nil
}

// scoreRaid computes the all-star score per difficulty for one raid.
func scoreRaid(ctx context.Context, db *sql.DB, realmName string, guid uint64, raidName string) (map[int]AllStarScore, error) {
	bossIDs, err := loadRaidBossIDs(ctx, db, realmName, raidName)
	if err != nil {
		return nil, err
	}
	if len(bossIDs) == 0 {
		return nil, nil
	}
	rows, err := loadAllStarRankRows(ctx, db, realmName, guid, bossIDs)
	if err != nil {
		return nil, err
	}
	return computeAllStar(rows), nil
}

func loadCharacterRaids(ctx context.Context, db *sql.DB, realmName string, guid uint64) ([]raidActivity, error) {
	const q = `
		SELECT raid_name, count() AS kills, max(kill_time) AS last_kill
		FROM boss_kill
		WHERE realm = ? AND has(players.guid, ?) AND raid_name != ''
		GROUP BY raid_name
	`
	rows, err := db.QueryContext(ctx, q, realmName, guid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []raidActivity
	for rows.Next() {
		var (
			name  string
			kills uint64
			last  time.Time
		)
		if err := rows.Scan(&name, &kills, &last); err != nil {
			return nil, err
		}
		out = append(out, raidActivity{Name: name, KillCount: int(kills), LastKill: last})
	}
	return out, rows.Err()
}

func raidByLastKill(raids []raidActivity) string {
	best := ""
	var bestT time.Time
	for _, r := range raids {
		if best == "" || r.LastKill.After(bestT) {
			best, bestT = r.Name, r.LastKill
		}
	}
	return best
}

func raidByKillCount(raids []raidActivity) string {
	best := ""
	bestN := -1
	for _, r := range raids {
		if r.KillCount > bestN {
			best, bestN = r.Name, r.KillCount
		}
	}
	return best
}

// allStarHref builds the fragment URL for a raid + difficulty. A negative mode
// omits the difficulty param, letting the server pick the default difficulty.
func allStarHref(realmName, charName, raidName string, mode int) string {
	href := links.Character(realmName, charName) + "/allstar?raid=" + url.QueryEscape(raidName)
	if mode >= 0 {
		href += "&diff=" + strconv.Itoa(mode)
	}
	return href
}

func loadRaidBossIDs(ctx context.Context, db *sql.DB, realmName, raidName string) ([]uint32, error) {
	const q = `
		SELECT remote_id
		FROM boss FINAL
		WHERE realm = ? AND raid_name = ?
	`
	rows, err := db.QueryContext(ctx, q, realmName, raidName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uint32
	for rows.Next() {
		var id uint32
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// loadAllStarRankRows returns the character's best-per-(boss, mode, spec) for the
// given bosses, with rank, top parse value and peer-pool size per metric. For
// vanilla realms talent_spec is 0, so the (boss, mode, spec) bracket naturally
// collapses to (boss, mode) — the same query serves every expansion.
func loadAllStarRankRows(ctx context.Context, db *sql.DB, realmName string, guid uint64, bossIDs []uint32) ([]rankRow, error) {
	ph := sqlutil.Placeholders(len(bossIDs))
	q := `
	WITH
	  my_bests AS (
	    SELECT boss_remote_id, mode, talent_spec,
	           maxMerge(dps_state) AS dps, maxMerge(hps_state) AS hps
	    FROM character_boss_rankings
	    WHERE realm = ? AND guid = ? AND boss_remote_id IN (` + ph + `)
	    GROUP BY realm, boss_remote_id, mode, talent_spec, guid
	    HAVING dps > 0 OR hps > 0
	  ),
	  peers AS (
	    SELECT boss_remote_id, mode, talent_spec,
	           maxMerge(dps_state) AS dps, maxMerge(hps_state) AS hps
	    FROM character_boss_rankings
	    WHERE realm = ?
	      AND (boss_remote_id, mode, talent_spec) IN (
	        SELECT boss_remote_id, mode, talent_spec FROM my_bests
	      )
	    GROUP BY realm, boss_remote_id, mode, talent_spec, guid
	  )
	SELECT
	  m.boss_remote_id,
	  m.mode,
	  m.talent_spec,
	  m.dps,
	  m.hps,
	  countIf(p.dps > m.dps) + 1 AS dps_rank,
	  countIf(p.hps > m.hps) + 1 AS hps_rank,
	  max(p.dps) AS dps_rank1,
	  max(p.hps) AS hps_rank1,
	  countIf(p.dps > 0) AS dps_n,
	  countIf(p.hps > 0) AS hps_n
	FROM my_bests m
	JOIN peers p USING (boss_remote_id, mode, talent_spec)
	GROUP BY m.boss_remote_id, m.mode, m.talent_spec, m.dps, m.hps
	`
	args := make([]any, 0, len(bossIDs)+3)
	args = append(args, realmName, guid)
	for _, id := range bossIDs {
		args = append(args, id)
	}
	args = append(args, realmName)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []rankRow
	for rows.Next() {
		var (
			bossID             uint32
			mode               uint8
			spec               uint16
			dps, hps           uint64
			dpsRank, hpsRank   uint64
			dpsRank1, hpsRank1 uint64
			dpsN, hpsN         uint64
		)
		if err := rows.Scan(&bossID, &mode, &spec, &dps, &hps, &dpsRank, &hpsRank, &dpsRank1, &hpsRank1, &dpsN, &hpsN); err != nil {
			return nil, err
		}
		out = append(out, rankRow{
			BossID:   bossID,
			Mode:     int(mode),
			Spec:     int(spec),
			DPS:      int64(dps),
			HPS:      int64(hps),
			DPSRank:  int(dpsRank),
			HPSRank:  int(hpsRank),
			DPSRank1: int64(dpsRank1),
			HPSRank1: int64(hpsRank1),
			DPSN:     int(dpsN),
			HPSN:     int(hpsN),
		})
	}
	return out, rows.Err()
}

func buildAllStarViewModel(realmName, charName string, expansion int, raidName string, raids []raidActivity, scores map[int]AllStarScore, selectedMode int) AllStarViewModel {
	vm := AllStarViewModel{Realm: realmName, CharName: charName, RaidName: raidName}

	// Raid selector, most-recently-active first.
	names := make([]raidActivity, len(raids))
	copy(names, raids)
	sort.Slice(names, func(i, j int) bool { return names[i].LastKill.After(names[j].LastKill) })
	for _, r := range names {
		vm.Raids = append(vm.Raids, AllStarRaidOption{
			Name:     r.Name,
			Selected: r.Name == raidName,
			Href:     allStarHref(realmName, charName, r.Name, -1),
		})
	}

	if len(scores) == 0 {
		return vm
	}

	// Resolve the selected difficulty, defaulting when unset/unavailable.
	if _, ok := scores[selectedMode]; !ok {
		selectedMode = pickDefaultMode(scores)
	}
	vm.Mode = selectedMode
	vm.DifficultyLabel = wow.Difficulty(expansion, selectedMode)

	// Difficulty selector, highest mode first (hardest difficulties lead).
	modes := make([]int, 0, len(scores))
	for m := range scores {
		modes = append(modes, m)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(modes)))
	for _, m := range modes {
		vm.Difficulties = append(vm.Difficulties, AllStarDiffOption{
			Mode:     m,
			Label:    wow.Difficulty(expansion, m),
			Selected: m == selectedMode,
			Href:     allStarHref(realmName, charName, raidName, m),
		})
	}

	score := scores[selectedMode]
	if score.DPSBosses > 0 {
		vm.HasDPS = true
		vm.DPSPoints = int(math.Round(score.DPSPoints))
		vm.DPSBosses = score.DPSBosses
		vm.DPSAvg = score.DPSPoints / float64(score.DPSBosses)
		vm.DPSGrade, vm.DPSGradeClass = allStarGrade(vm.DPSAvg)
	}
	if score.HPSBosses > 0 {
		vm.HasHPS = true
		vm.HPSPoints = int(math.Round(score.HPSPoints))
		vm.HPSBosses = score.HPSBosses
		vm.HPSAvg = score.HPSPoints / float64(score.HPSBosses)
		vm.HPSGrade, vm.HPSGradeClass = allStarGrade(vm.HPSAvg)
	}
	return vm
}

func allStarPoints(value, rank1Value int64, rank, n int) float64 {
	if value <= 0 || rank1Value <= 0 || n <= 0 {
		return 0
	}
	ratio := float64(value) / float64(rank1Value)
	if ratio > 1 {
		ratio = 1
	}
	pct := allStarBase * (1 - float64(rank-1)/float64(n))
	return math.Max(allStarBase*ratio, pct) + 0.2*allStarBase*ratio
}

// computeAllStar sums the best per-boss points, grouped by difficulty (mode) so
// Normal and Heroic never mix. Within each mode it keeps the best points per
// boss (across specs), summed; DPS and HPS stay independent. Returns one score
// per difficulty the character has ranked data in.
func computeAllStar(rows []rankRow) map[int]AllStarScore {
	// mode -> boss -> best points
	dpsBest := map[int]map[uint32]float64{}
	hpsBest := map[int]map[uint32]float64{}
	put := func(m map[int]map[uint32]float64, mode int, boss uint32, p float64) {
		if p <= 0 {
			return
		}
		if m[mode] == nil {
			m[mode] = map[uint32]float64{}
		}
		if p > m[mode][boss] {
			m[mode][boss] = p
		}
	}

	for _, r := range rows {
		put(dpsBest, r.Mode, r.BossID, allStarPoints(r.DPS, r.DPSRank1, r.DPSRank, r.DPSN))
		put(hpsBest, r.Mode, r.BossID, allStarPoints(r.HPS, r.HPSRank1, r.HPSRank, r.HPSN))
	}

	out := map[int]AllStarScore{}
	for mode, bosses := range dpsBest {
		s := out[mode]
		for _, p := range bosses {
			s.DPSPoints += p
			s.DPSBosses++
		}
		out[mode] = s
	}
	for mode, bosses := range hpsBest {
		s := out[mode]
		for _, p := range bosses {
			s.HPSPoints += p
			s.HPSBosses++
		}
		out[mode] = s
	}
	return out
}

// pickDefaultMode chooses the difficulty to show by default: the one with the
// most ranked bosses (the character's main progression difficulty), breaking
// ties toward the higher mode. Returns 0 when there are no scores.
func pickDefaultMode(scores map[int]AllStarScore) int {
	bestMode := -1
	bestBosses := -1
	for mode, s := range scores {
		n := s.DPSBosses + s.HPSBosses
		if n > bestBosses || (n == bestBosses && mode > bestMode) {
			bestMode, bestBosses = mode, n
		}
	}
	if bestMode < 0 {
		return 0
	}
	return bestMode
}
