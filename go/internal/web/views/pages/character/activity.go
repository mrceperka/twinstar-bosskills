package character

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mrceperka/twinstar-bosskills/go/internal/api"
	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
	"github.com/mrceperka/twinstar-bosskills/go/internal/wow"
)

const (
	activityCacheTTL          = time.Hour
	activityFeedPageSize      = 25
	activityDisplayPageSize   = 5
	activityTypeRefreshMarker = 0
	activityTypeAchievement   = 1
	activityTypeLoot          = 2
	activityTypeBossKill      = 3
)

const insertActivitySQL = `
	INSERT INTO character_activity_feed (
		realm, character_name, event_type, event_time, api_id, data, data2,
		difficulty, item_guid, item_quality, icon, title, boss_kill_remote_id,
		achievement_points, achievement_realm_first, refresh_error, source_page
	)
`

type activityAPIClient interface {
	GetCharacterActivityFeed(ctx context.Context, realmName, characterName string, page, pageSize int) (api.PaginatedCharacterActivityFeed, error)
}

type activityRefreshMeta struct {
	Found         bool
	LastCheckedAt time.Time
	LastSuccessAt time.Time
	LastError     string
}

func activityIsStale(meta activityRefreshMeta, now time.Time) bool {
	if !meta.Found || meta.LastCheckedAt.IsZero() {
		return true
	}
	return !meta.LastCheckedAt.After(now.Add(-activityCacheTTL))
}

type activityStorageRow struct {
	Realm                 string
	CharacterName         string
	EventType             uint8
	EventTime             time.Time
	APIID                 uint16
	Data                  uint32
	Data2                 uint64
	Difficulty            uint8
	ItemGUID              uint64
	ItemQuality           uint8
	Icon                  string
	Title                 string
	BossKillRemoteID      string
	AchievementPoints     uint16
	AchievementRealmFirst bool
	RefreshError          string
	SourcePage            uint8
}

type ActivityViewModel struct {
	Realm         string
	CharName      string
	Rows          []ActivityRow
	Notice        string
	Warning       string
	Error         string
	LastCheckedAt string
	LastSuccessAt string
	Page          int
	HasMore       bool
	NextHref      string
}

type ActivityRow struct {
	EventType        int
	EventKind        string
	EventTime        string
	Title            string
	DifficultyLabel  string
	ItemQuality      int
	ItemQualityColor string
	ItemID           uint32
	BossKillRemoteID string
}

func buildActivityStorageRows(realmName, characterName string, sourcePage int, events []api.CharacterActivityEvent) []activityStorageRow {
	rows := make([]activityStorageRow, 0, len(events))
	for _, e := range events {
		row := activityStorageRow{
			Realm:         realmName,
			CharacterName: characterName,
			EventType:     uint8(e.Type),
			EventTime:     time.Unix(e.Date, 0).UTC(),
			APIID:         uint16(e.ID),
			Data:          e.Data,
			Data2:         e.Data2,
			Difficulty:    uint8(e.Difficulty),
			ItemGUID:      e.ItemGUID,
			ItemQuality:   uint8(e.ItemQuality),
			Icon:          e.Icon,
			SourcePage:    uint8(sourcePage),
		}
		switch e.Type {
		case activityTypeAchievement:
			if e.Achievement != nil {
				row.Title = e.Achievement.Name
				row.AchievementPoints = uint16(e.Achievement.Points)
				row.AchievementRealmFirst = e.Achievement.RealmFirst
			}
		case activityTypeLoot:
			if e.Loot != nil {
				row.Title = e.Loot.Name
			}
		case activityTypeBossKill:
			if e.Boss != nil {
				row.Title = e.Boss.Name
			}
			if e.Data2 > 0 {
				row.BossKillRemoteID = fmt.Sprintf("%d_%d", realm.ID(realmName), e.Data2)
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func buildActivityRefreshMarker(realmName, characterName string, checkedAt time.Time, lastError string) activityStorageRow {
	return activityStorageRow{
		Realm:         realmName,
		CharacterName: characterName,
		EventType:     activityTypeRefreshMarker,
		EventTime:     checkedAt.UTC(),
		Title:         "refresh",
		RefreshError:  lastError,
	}
}

func loadActivityRefreshMeta(ctx context.Context, db *sql.DB, realmName, characterName string) (activityRefreshMeta, error) {
	const q = `
		SELECT
			count(),
			max(event_time) AS last_checked_at,
			maxIf(event_time, refresh_error = '') AS last_success_at,
			argMax(refresh_error, event_time) AS last_error
		FROM character_activity_feed FINAL
		WHERE realm = ? AND character_name = ?
		  AND event_type = ?
	`
	var (
		meta  activityRefreshMeta
		count uint64
	)
	if err := db.QueryRowContext(ctx, q, realmName, characterName, uint8(activityTypeRefreshMarker)).
		Scan(&count, &meta.LastCheckedAt, &meta.LastSuccessAt, &meta.LastError); err != nil {
		if err == sql.ErrNoRows {
			return activityRefreshMeta{}, nil
		}
		return activityRefreshMeta{}, err
	}
	meta.Found = count > 0
	return meta, nil
}

func insertActivityRows(ctx context.Context, db *sql.DB, rows []activityStorageRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, insertActivitySQL)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		_, err := stmt.ExecContext(ctx,
			r.Realm, r.CharacterName, r.EventType, r.EventTime, r.APIID, r.Data, r.Data2,
			r.Difficulty, r.ItemGUID, r.ItemQuality, r.Icon, r.Title, r.BossKillRemoteID,
			r.AchievementPoints, r.AchievementRealmFirst, r.RefreshError, r.SourcePage,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func refreshActivityIfStale(ctx context.Context, db *sql.DB, cli activityAPIClient, realmName, characterName string, now time.Time) (activityRefreshMeta, error) {
	meta, err := loadActivityRefreshMeta(ctx, db, realmName, characterName)
	if err != nil {
		return meta, err
	}
	if !activityIsStale(meta, now) {
		return meta, nil
	}

	var rows []activityStorageRow
	for page := 0; page <= 1; page++ {
		res, err := cli.GetCharacterActivityFeed(ctx, realmName, characterName, page, activityFeedPageSize)
		if err != nil {
			_ = insertActivityRows(ctx, db, []activityStorageRow{buildActivityRefreshMarker(realmName, characterName, now, err.Error())})
			meta.LastCheckedAt = now
			meta.LastError = err.Error()
			meta.Found = true
			return meta, err
		}
		rows = append(rows, buildActivityStorageRows(realmName, characterName, page, res.Data)...)
	}
	rows = append(rows, buildActivityRefreshMarker(realmName, characterName, now, ""))
	if err := insertActivityRows(ctx, db, rows); err != nil {
		return meta, err
	}
	return activityRefreshMeta{
		Found:         true,
		LastCheckedAt: now,
		LastSuccessAt: now,
	}, nil
}

func loadActivityRows(ctx context.Context, db *sql.DB, realmName, characterName string, expansion int, limit int) ([]ActivityRow, error) {
	const q = `
		SELECT
			event_type,
			event_time,
			title,
			difficulty,
			item_quality,
			data,
			if(
				boss_kill_remote_id != ''
				AND boss_kill_remote_id IN (SELECT remote_id FROM boss_kill WHERE realm = ?),
				boss_kill_remote_id,
				''
			) AS boss_kill_remote_id
		FROM character_activity_feed FINAL
		WHERE realm = ? AND character_name = ?
		  AND event_type != ?
		ORDER BY event_time DESC, api_id ASC
		LIMIT ?
	`
	rows, err := db.QueryContext(ctx, q, realmName, realmName, characterName, uint8(activityTypeRefreshMarker), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ActivityRow
	for rows.Next() {
		var (
			eventType        uint8
			eventTime        time.Time
			title            string
			difficulty       uint8
			itemQuality      uint8
			itemID           uint32
			bossKillRemoteID string
		)
		if err := rows.Scan(&eventType, &eventTime, &title, &difficulty, &itemQuality, &itemID, &bossKillRemoteID); err != nil {
			return nil, err
		}
		if eventType != activityTypeLoot {
			itemID = 0
		}
		out = append(out, ActivityRow{
			EventType:        int(eventType),
			EventKind:        activityKindLabel(int(eventType)),
			EventTime:        eventTime.Format("2006-01-02 15:04"),
			Title:            title,
			DifficultyLabel:  activityDifficultyLabel(expansion, int(eventType), int(difficulty)),
			ItemQuality:      int(itemQuality),
			ItemQualityColor: api.QualityColor(int(itemQuality)),
			ItemID:           itemID,
			BossKillRemoteID: bossKillRemoteID,
		})
	}
	return out, rows.Err()
}

func activityKindLabel(eventType int) string {
	switch eventType {
	case activityTypeAchievement:
		return "Achievement"
	case activityTypeLoot:
		return "Loot"
	case activityTypeBossKill:
		return "Boss kill"
	default:
		return "Activity"
	}
}

func activityDifficultyLabel(expansion, eventType, difficulty int) string {
	if eventType != activityTypeBossKill || difficulty == 0 {
		return ""
	}
	return wow.Difficulty(expansion, difficulty)
}
