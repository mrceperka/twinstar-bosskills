package main

import (
	"time"

	"twinstar-bosskills/internal/api"
)

// row is the in-memory shape of one boss_kill INSERT, with Nested columns
// expanded into parallel slices the way ClickHouse expects.
type row struct {
	RemoteID     string
	Realm        string
	RaidName     string
	BossRemoteID uint32
	BossName     string
	Mode         uint8
	Guild        string
	KillTime     time.Time
	Length       uint32
	Wipes        uint32
	Deaths       uint32
	RessUsed     uint32

	PlayersGUID            []uint64
	PlayersTalentSpec      []uint16
	PlayersAvgItemLvl      []float32
	PlayersDmgDone         []uint64
	PlayersHealingDone     []uint64
	PlayersOverhealingDone []uint64
	PlayersAbsorbDone      []uint64
	PlayersDmgTaken        []uint64
	PlayersDmgAbsorbed     []uint64
	PlayersHealingTaken    []uint64
	PlayersDispels         []uint32
	PlayersInterrupts      []uint32
	PlayersName            []string
	PlayersRace            []uint8
	PlayersClass           []uint8
	PlayersGender          []uint8
	PlayersLevel           []uint8

	DeathsRemoteID []uint32
	DeathsGUID     []uint64
	DeathsTime     []int32

	LootRemoteID []uint32
	LootItemID   []uint32
	LootCount    []uint8

	TimelineTime            []int32
	TimelineEncounterDamage []uint64
	TimelineEncounterHeal   []uint64
	TimelineRaidDamage      []uint64
	TimelineRaidHeal        []uint64
}

// buildRow assembles a row from a list-API entry and its detail. detail may be
// nil — in that case the Nested slices are left empty.
func buildRow(realmName, raidName string, bk api.BossKill, detail *api.BossKillDetail) (row, error) {
	t, err := parseAPITime(bk.Time)
	if err != nil {
		return row{}, err
	}
	r := row{
		RemoteID:     bk.ID,
		Realm:        realmName,
		RaidName:     raidName,
		BossRemoteID: uint32(bk.Entry),
		BossName:     bk.CreatureName,
		Mode:         uint8(bk.Mode),
		Guild:        bk.Guild,
		KillTime:     t,
		Length:       uint32(bk.Length),
		Wipes:        uint32(bk.Wipes),
		Deaths:       uint32(bk.Deaths),
		RessUsed:     uint32(bk.RessUsed),
	}
	if detail == nil {
		return r, nil
	}
	for _, p := range detail.Players {
		r.PlayersGUID = append(r.PlayersGUID, uint64(p.GUID))
		r.PlayersTalentSpec = append(r.PlayersTalentSpec, uint16(p.TalentSpec))
		r.PlayersAvgItemLvl = append(r.PlayersAvgItemLvl, float32(p.AvgItemLvl))
		r.PlayersDmgDone = append(r.PlayersDmgDone, toU64(p.DmgDone))
		r.PlayersHealingDone = append(r.PlayersHealingDone, toU64(p.HealingDone))
		r.PlayersOverhealingDone = append(r.PlayersOverhealingDone, toU64(p.OverhealingDone))
		r.PlayersAbsorbDone = append(r.PlayersAbsorbDone, toU64(p.AbsorbDone))
		r.PlayersDmgTaken = append(r.PlayersDmgTaken, toU64(p.DmgTaken))
		r.PlayersDmgAbsorbed = append(r.PlayersDmgAbsorbed, toU64(p.DmgAbsorbed))
		r.PlayersHealingTaken = append(r.PlayersHealingTaken, toU64(p.HealingTaken))
		r.PlayersDispels = append(r.PlayersDispels, uint32(p.Dispels))
		r.PlayersInterrupts = append(r.PlayersInterrupts, uint32(p.Interrupts))
		r.PlayersName = append(r.PlayersName, p.Name)
		r.PlayersRace = append(r.PlayersRace, uint8(p.Race))
		r.PlayersClass = append(r.PlayersClass, uint8(p.Class))
		r.PlayersGender = append(r.PlayersGender, uint8(p.Gender))
		r.PlayersLevel = append(r.PlayersLevel, uint8(p.Level))
	}
	for _, d := range detail.Deaths {
		r.DeathsRemoteID = append(r.DeathsRemoteID, uint32(d.ID))
		r.DeathsGUID = append(r.DeathsGUID, uint64(d.GUID))
		r.DeathsTime = append(r.DeathsTime, int32(d.Time))
	}
	for _, l := range detail.Loot {
		r.LootRemoteID = append(r.LootRemoteID, uint32(l.ID))
		// upstream sends itemId as a string; tolerate non-numeric by skipping
		var iid uint32
		_, _ = fmtSscanU32(l.ItemID, &iid)
		r.LootItemID = append(r.LootItemID, iid)
		r.LootCount = append(r.LootCount, uint8(l.Count))
	}
	for _, tl := range detail.Timeline {
		r.TimelineTime = append(r.TimelineTime, int32(tl.Time))
		r.TimelineEncounterDamage = append(r.TimelineEncounterDamage, toU64(tl.EncounterDamage))
		r.TimelineEncounterHeal = append(r.TimelineEncounterHeal, toU64(tl.EncounterHeal))
		r.TimelineRaidDamage = append(r.TimelineRaidDamage, toU64(tl.RaidDamage))
		r.TimelineRaidHeal = append(r.TimelineRaidHeal, toU64(tl.RaidHeal))
	}
	return r, nil
}

func toU64(v api.FlexInt) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

// parseAPITime parses the timestamps the upstream API returns. Empirically the
// API uses RFC3339-like strings ("2024-01-02T03:04:05Z" or with offset); fall
// back to a couple of other formats just in case.
func parseAPITime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	var err error
	for _, f := range formats {
		var t time.Time
		t, err = time.Parse(f, s)
		if err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, err
}

// fmtSscanU32 is a tiny wrapper to avoid importing strconv just for one parse.
func fmtSscanU32(s string, out *uint32) (n int, err error) {
	var v uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			if i == 0 {
				return 0, nil
			}
			break
		}
		v = v*10 + uint64(c-'0')
		n++
	}
	*out = uint32(v)
	return n, nil
}
