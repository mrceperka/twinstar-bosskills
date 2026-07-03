package character

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/api"
	"github.com/mrceperka/twinstar-bosskills/go/internal/format"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

type statsAPIClient interface {
	GetCharacterStats(ctx context.Context, realmName, characterName string) (api.CharacterStatsPayload, error)
}

const insertStatsSQL = `
	INSERT INTO character_stats (
		realm, character_name, raw_json,
		health, mana, stamina, strength, agility, intellect, spirit,
		stamina_pos_buf, stamina_neg_buf,
		strength_pos_buf, strength_neg_buf,
		agility_pos_buf, agility_neg_buf,
		intellect_pos_buf, intellect_neg_buf,
		spirit_pos_buf, spirit_neg_buf,
		energy_regen,
		attack_power, spell_power,
		hit_base, hit_percent, hit_rating,
		crit_base, crit_percent, crit_rating,
		haste_base, haste_percent, haste_rating,
		mastery_base, mastery_percent, mastery_rating,
		armor_base, armor_percent, armor_rating,
		dodge_base, dodge_percent, dodge_rating,
		parry_base, parry_percent, parry_rating,
		last_checked_at, last_success_at, last_error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

type statsStorageRow struct {
	Found          bool
	Realm          string
	CharacterName  string
	CharacterClass int
	RawJSON        string

	Health          uint32
	Mana            uint32
	Stamina         uint32
	Strength        uint32
	Agility         uint32
	Intellect       uint32
	Spirit          uint32
	StaminaPosBuf   uint32
	StaminaNegBuf   uint32
	StrengthPosBuf  uint32
	StrengthNegBuf  uint32
	AgilityPosBuf   uint32
	AgilityNegBuf   uint32
	IntellectPosBuf uint32
	IntellectNegBuf uint32
	SpiritPosBuf    uint32
	SpiritNegBuf    uint32
	EnergyRegen     float64
	AttackPower     uint32
	SpellPower      uint32
	HitBase         uint32
	HitPercent      float64
	HitRating       float64
	CritBase        uint32
	CritPercent     float64
	CritRating      float64
	HasteBase       uint32
	HastePercent    float64
	HasteRating     float64
	MasteryBase     uint32
	MasteryPercent  float64
	MasteryRating   float64
	ArmorBase       uint32
	ArmorPercent    float64
	ArmorRating     float64
	DodgeBase       uint32
	DodgePercent    float64
	DodgeRating     float64
	ParryBase       uint32
	ParryPercent    float64
	ParryRating     float64

	LastCheckedAt time.Time
	LastSuccessAt time.Time
	LastError     string
}

type StatsViewModel struct {
	Notice        string
	Warning       string
	Error         string
	LastSuccessAt string
	Groups        []StatsGroup
}

type StatsGroup struct {
	Title   string
	Metrics []StatsMetric
}

type StatsMetric struct {
	Label      string
	Value      string
	ValueClass string
	PosDelta   string
	NegDelta   string
}

func statsIsStale(row statsStorageRow, now time.Time) bool {
	if !row.Found || row.LastCheckedAt.IsZero() {
		return true
	}
	return !row.LastCheckedAt.After(now.Add(-activityCacheTTL))
}

func buildStatsStorageRow(realmName, characterName string, payload api.CharacterStatsPayload, now time.Time, lastError string) statsStorageRow {
	stats := payload.Stats
	hit := firstRating(stats.Melee.HitChance, stats.Spell.HitChance)
	crit := firstRating(stats.Melee.CritChance, stats.Spell.CritChance)
	haste := firstRating(stats.Melee.Haste, stats.Spell.Haste)
	mastery := firstRating(stats.Melee.Mastery, stats.Spell.Mastery)
	return statsStorageRow{
		Found:           true,
		Realm:           realmName,
		CharacterName:   characterName,
		RawJSON:         string(payload.Raw),
		Health:          stats.Attributes.Stamina.Health,
		Mana:            nonNegativeUint32(float64(stats.Attributes.Intellect.Mana)),
		Stamina:         stats.Attributes.Stamina.Effective,
		Strength:        stats.Attributes.Strength.Effective,
		Agility:         stats.Attributes.Agility.Effective,
		Intellect:       stats.Attributes.Intellect.Effective,
		Spirit:          stats.Attributes.Spirit.Effective,
		StaminaPosBuf:   stats.Attributes.Stamina.PosBuf,
		StaminaNegBuf:   stats.Attributes.Stamina.NegBuf,
		StrengthPosBuf:  stats.Attributes.Strength.PosBuf,
		StrengthNegBuf:  stats.Attributes.Strength.NegBuf,
		AgilityPosBuf:   stats.Attributes.Agility.PosBuf,
		AgilityNegBuf:   stats.Attributes.Agility.NegBuf,
		IntellectPosBuf: stats.Attributes.Intellect.PosBuf,
		IntellectNegBuf: stats.Attributes.Intellect.NegBuf,
		SpiritPosBuf:    stats.Attributes.Spirit.PosBuf,
		SpiritNegBuf:    stats.Attributes.Spirit.NegBuf,
		EnergyRegen:     stats.Melee.EnergyRegen,
		AttackPower:     nonNegativeUint32(stats.Melee.AttackPower.Effective),
		SpellPower:      stats.Spell.SpellPower,
		HitBase:         hit.Base,
		HitPercent:      hit.Percent,
		HitRating:       hit.Rating,
		CritBase:        crit.Base,
		CritPercent:     crit.Percent,
		CritRating:      crit.Rating,
		HasteBase:       haste.Base,
		HastePercent:    haste.Percent,
		HasteRating:     haste.Rating,
		MasteryBase:     mastery.Base,
		MasteryPercent:  mastery.Percent,
		MasteryRating:   mastery.Rating,
		ArmorBase:       stats.Defense.Armor.Base,
		ArmorPercent:    stats.Defense.Armor.Percent,
		ArmorRating:     stats.Defense.Armor.Rating,
		DodgeBase:       stats.Defense.Dodge.Base,
		DodgePercent:    stats.Defense.Dodge.Percent,
		DodgeRating:     stats.Defense.Dodge.Rating,
		ParryBase:       stats.Defense.Parry.Base,
		ParryPercent:    stats.Defense.Parry.Percent,
		ParryRating:     stats.Defense.Parry.Rating,
		LastCheckedAt:   now.UTC(),
		LastSuccessAt:   now.UTC(),
		LastError:       lastError,
	}
}

func firstRating(values ...api.CharacterRatingStat) api.CharacterRatingStat {
	for _, v := range values {
		if v.Base != 0 || v.Percent != 0 || v.Rating != 0 {
			return v
		}
	}
	return api.CharacterRatingStat{}
}

func loadStatsRow(ctx context.Context, db *sql.DB, realmName, characterName string) (statsStorageRow, error) {
	const q = `
		SELECT
			raw_json,
			health, mana, stamina, strength, agility, intellect, spirit,
			stamina_pos_buf, stamina_neg_buf,
			strength_pos_buf, strength_neg_buf,
			agility_pos_buf, agility_neg_buf,
			intellect_pos_buf, intellect_neg_buf,
			spirit_pos_buf, spirit_neg_buf,
			energy_regen,
			attack_power, spell_power,
			hit_base, hit_percent, hit_rating,
			crit_base, crit_percent, crit_rating,
			haste_base, haste_percent, haste_rating,
			mastery_base, mastery_percent, mastery_rating,
			armor_base, armor_percent, armor_rating,
			dodge_base, dodge_percent, dodge_rating,
			parry_base, parry_percent, parry_rating,
			last_checked_at, last_success_at, last_error
		FROM character_stats FINAL
		WHERE realm = ? AND character_name = ?
		LIMIT 1
	`
	row := statsStorageRow{Realm: realmName, CharacterName: characterName}
	err := db.QueryRowContext(ctx, q, realmName, characterName).Scan(
		&row.RawJSON,
		&row.Health, &row.Mana, &row.Stamina, &row.Strength, &row.Agility, &row.Intellect, &row.Spirit,
		&row.StaminaPosBuf, &row.StaminaNegBuf,
		&row.StrengthPosBuf, &row.StrengthNegBuf,
		&row.AgilityPosBuf, &row.AgilityNegBuf,
		&row.IntellectPosBuf, &row.IntellectNegBuf,
		&row.SpiritPosBuf, &row.SpiritNegBuf,
		&row.EnergyRegen,
		&row.AttackPower, &row.SpellPower,
		&row.HitBase, &row.HitPercent, &row.HitRating,
		&row.CritBase, &row.CritPercent, &row.CritRating,
		&row.HasteBase, &row.HastePercent, &row.HasteRating,
		&row.MasteryBase, &row.MasteryPercent, &row.MasteryRating,
		&row.ArmorBase, &row.ArmorPercent, &row.ArmorRating,
		&row.DodgeBase, &row.DodgePercent, &row.DodgeRating,
		&row.ParryBase, &row.ParryPercent, &row.ParryRating,
		&row.LastCheckedAt, &row.LastSuccessAt, &row.LastError,
	)
	if err == sql.ErrNoRows {
		return statsStorageRow{}, nil
	}
	if err != nil {
		return statsStorageRow{}, err
	}
	row.Found = true
	return row, nil
}

func insertStatsRow(ctx context.Context, db *sql.DB, row statsStorageRow) error {
	if row.LastSuccessAt.IsZero() {
		row.LastSuccessAt = time.Unix(0, 0).UTC()
	}
	_, err := db.ExecContext(ctx, insertStatsSQL,
		row.Realm, row.CharacterName, row.RawJSON,
		row.Health, row.Mana, row.Stamina, row.Strength, row.Agility, row.Intellect, row.Spirit,
		row.StaminaPosBuf, row.StaminaNegBuf,
		row.StrengthPosBuf, row.StrengthNegBuf,
		row.AgilityPosBuf, row.AgilityNegBuf,
		row.IntellectPosBuf, row.IntellectNegBuf,
		row.SpiritPosBuf, row.SpiritNegBuf,
		row.EnergyRegen,
		row.AttackPower, row.SpellPower,
		row.HitBase, row.HitPercent, row.HitRating,
		row.CritBase, row.CritPercent, row.CritRating,
		row.HasteBase, row.HastePercent, row.HasteRating,
		row.MasteryBase, row.MasteryPercent, row.MasteryRating,
		row.ArmorBase, row.ArmorPercent, row.ArmorRating,
		row.DodgeBase, row.DodgePercent, row.DodgeRating,
		row.ParryBase, row.ParryPercent, row.ParryRating,
		row.LastCheckedAt.UTC(), row.LastSuccessAt.UTC(), row.LastError,
	)
	return err
}

func refreshStatsIfStale(ctx context.Context, db *sql.DB, cli statsAPIClient, realmName, characterName string, now time.Time) (statsStorageRow, error) {
	row, err := loadStatsRow(ctx, db, realmName, characterName)
	if err != nil {
		return row, err
	}
	if !statsIsStale(row, now) {
		return row, nil
	}

	payload, err := cli.GetCharacterStats(ctx, realmName, characterName)
	if err != nil {
		failed := row
		failed.Found = row.Found
		failed.Realm = realmName
		failed.CharacterName = characterName
		failed.LastCheckedAt = now.UTC()
		failed.LastError = err.Error()
		if ierr := insertStatsRow(ctx, db, failed); ierr != nil {
			return row, ierr
		}
		return failed, err
	}

	fresh := buildStatsStorageRow(realmName, characterName, payload, now, "")
	if err := insertStatsRow(ctx, db, fresh); err != nil {
		return row, err
	}
	return fresh, nil
}

func buildStatsViewModel(ctx context.Context, row statsStorageRow, refreshErr error) StatsViewModel {
	vm := StatsViewModel{
		Notice: "Stats are cached and may be up to 1 hour stale.",
	}
	if !row.Found {
		if refreshErr != nil {
			vm.Error = "Stats are unavailable right now."
		}
		return vm
	}
	if !row.LastSuccessAt.IsZero() && row.LastSuccessAt.After(time.Unix(0, 0)) {
		vm.LastSuccessAt = row.LastSuccessAt.Format("2006-01-02 15:04")
	}
	if refreshErr != nil {
		vm.Warning = "Latest refresh failed; showing cached stats."
	}

	loc := format.LocaleFromContext(ctx)
	coreMetrics := []StatsMetric{
		{Label: "Health", Value: format.Int(loc, int(row.Health)), ValueClass: "text-red-300"},
	}
	if row.Mana > 0 {
		coreMetrics = append(coreMetrics, StatsMetric{Label: "Mana", Value: format.Int(loc, int(row.Mana)), ValueClass: "text-blue-300"})
	} else if resource := primaryResourceMetric(loc, row.CharacterClass); resource.Label != "" {
		coreMetrics = append(coreMetrics, resource)
	}
	if row.EnergyRegen > 0 {
		coreMetrics = append(coreMetrics, StatsMetric{Label: "Energy regen", Value: formatNumber(row.EnergyRegen), ValueClass: "text-yellow-300"})
	}
	vm.Groups = []StatsGroup{
		{
			Title:   "Core",
			Metrics: coreMetrics,
		},
		{
			Title: "Attributes",
			Metrics: []StatsMetric{
				attributeMetric(loc, "Stamina", row.Stamina, row.StaminaPosBuf, row.StaminaNegBuf),
				attributeMetric(loc, "Strength", row.Strength, row.StrengthPosBuf, row.StrengthNegBuf),
				attributeMetric(loc, "Agility", row.Agility, row.AgilityPosBuf, row.AgilityNegBuf),
				attributeMetric(loc, "Intellect", row.Intellect, row.IntellectPosBuf, row.IntellectNegBuf),
				attributeMetric(loc, "Spirit", row.Spirit, row.SpiritPosBuf, row.SpiritNegBuf),
			},
		},
		{
			Title: "Combat",
			Metrics: []StatsMetric{
				{Label: "Attack Power", Value: format.Int(loc, int(row.AttackPower))},
				{Label: "Spell Power", Value: format.Int(loc, int(row.SpellPower))},
				{Label: "Hit", Value: formatRatingStat(loc, row.HitBase, row.HitPercent, row.HitRating)},
				{Label: "Crit", Value: formatRatingStat(loc, row.CritBase, row.CritPercent, row.CritRating)},
				{Label: "Haste", Value: formatRatingStat(loc, row.HasteBase, row.HastePercent, row.HasteRating)},
				{Label: "Mastery", Value: formatRatingStat(loc, row.MasteryBase, row.MasteryPercent, row.MasteryRating)},
			},
		},
		{
			Title: "Defense",
			Metrics: []StatsMetric{
				{Label: "Armor", Value: formatRatingStat(loc, row.ArmorBase, row.ArmorPercent, row.ArmorRating)},
				{Label: "Dodge", Value: formatRatingStat(loc, row.DodgeBase, row.DodgePercent, row.DodgeRating)},
				{Label: "Parry", Value: formatRatingStat(loc, row.ParryBase, row.ParryPercent, row.ParryRating)},
			},
		},
	}
	return vm
}

func primaryResourceMetric(loc format.Locale, characterClass int) StatsMetric {
	switch characterClass {
	case wow.ClassWarrior:
		return StatsMetric{Label: "Rage", Value: format.Int(loc, 100), ValueClass: "text-orange-300"}
	case wow.ClassHunter:
		return StatsMetric{Label: "Focus", Value: format.Int(loc, 100), ValueClass: "text-orange-300"}
	case wow.ClassRogue, wow.ClassMonk:
		return StatsMetric{Label: "Energy", Value: format.Int(loc, 100), ValueClass: "text-yellow-300"}
	default:
		return StatsMetric{}
	}
}

func attributeMetric(loc format.Locale, label string, value, posBuf, negBuf uint32) StatsMetric {
	return StatsMetric{
		Label:    label,
		Value:    format.Int(loc, int(value)),
		PosDelta: formatSignedDelta(loc, "+", posBuf),
		NegDelta: formatSignedDelta(loc, "-", negBuf),
	}
}

func formatSignedDelta(loc format.Locale, sign string, value uint32) string {
	if value == 0 {
		return ""
	}
	return sign + format.Int(loc, int(value))
}

func nonNegativeUint32(v float64) uint32 {
	if v <= 0 {
		return 0
	}
	const maxUint32 = float64(1<<32 - 1)
	if v >= maxUint32 {
		return ^uint32(0)
	}
	return uint32(math.Round(v))
}

func formatRatingStat(loc format.Locale, base uint32, percent, rating float64) string {
	return fmt.Sprintf("Base %s | %s | Rating %s", format.Int(loc, int(base)), formatPercent(percent), formatNumber(rating))
}

func formatNumber(v float64) string {
	rounded := math.Round(v*10) / 10
	s := fmt.Sprintf("%.1f", rounded)
	return strings.TrimSuffix(strings.TrimSuffix(s, "0"), ".")
}

func formatPercent(v float64) string {
	return formatNumber(v) + "%"
}
