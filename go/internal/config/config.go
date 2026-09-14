// Package config centralizes environment-backed application configuration.
package config

import (
	"fmt"
	"os"

	"twinstar-bosskills/internal/api"
)

const (
	EnvClickHouseDSN       = "BK_CH_DSN"
	EnvTwinstarAPIURL      = "BK_TWINSTAR_API_URL"
	EnvHTTPAddr            = "BK_HTTP_ADDR"
	EnvIconDir             = "BK_ICON_DIR"
	EnvItemDir             = "BK_ITEM_DIR"
	EnvSecretTokenGuild    = "SECRET_TOKEN_GUILD"
	EnvSecretTokenAdmin    = "SECRET_TOKEN_ADMIN"
	EnvStatsInsertSmoke    = "BK_STATS_INSERT_SMOKE"
	EnvStatsSmokeCharacter = "BK_STATS_SMOKE_CHARACTER"
)

const (
	DefaultHTTPAddr       = ":3000"
	DefaultIconDir        = "./var/icons"
	DefaultItemDir        = "./var/items"
	DefaultStatsCharacter = "Plaguis"
)

type Config struct {
	ClickHouse ClickHouse
	Twinstar   Twinstar
	HTTP       HTTP
	Cache      Cache
	Secrets    Secrets
	StatsSmoke StatsSmoke
}

type ClickHouse struct {
	DSN string
}

type Twinstar struct {
	APIURL string
}

type HTTP struct {
	Addr string
}

type Cache struct {
	IconDir string
	ItemDir string
}

type Secrets struct {
	GuildToken string
	AdminToken string
}

type StatsSmoke struct {
	Enabled       bool
	CharacterName string
}

func FromEnv() Config {
	return Config{
		ClickHouse: ClickHouse{
			DSN: os.Getenv(EnvClickHouseDSN),
		},
		Twinstar: Twinstar{
			APIURL: envOr(EnvTwinstarAPIURL, api.DefaultBaseURL),
		},
		HTTP: HTTP{
			Addr: envOr(EnvHTTPAddr, DefaultHTTPAddr),
		},
		Cache: Cache{
			IconDir: envOr(EnvIconDir, DefaultIconDir),
			ItemDir: envOr(EnvItemDir, DefaultItemDir),
		},
		Secrets: Secrets{
			GuildToken: os.Getenv(EnvSecretTokenGuild),
			AdminToken: os.Getenv(EnvSecretTokenAdmin),
		},
		StatsSmoke: StatsSmoke{
			Enabled:       os.Getenv(EnvStatsInsertSmoke) == "1",
			CharacterName: envOr(EnvStatsSmokeCharacter, DefaultStatsCharacter),
		},
	}
}

func (c Config) RequireClickHouseDSN() error {
	if c.ClickHouse.DSN == "" {
		return fmt.Errorf("%s is required", EnvClickHouseDSN)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
