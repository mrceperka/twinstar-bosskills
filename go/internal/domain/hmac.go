package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/mrceperka/twinstar-bosskills/go/internal/realm"
)

// GuildToken mirrors packages/sveltekit/src/lib/server/guild-token.service.ts.
//
// The existing TS implementation hashes the concatenation
// SECRET_TOKEN_GUILD + realm + guild with SHA-256 and returns hex.
// Order and lack of separators are intentional — must match byte-for-byte
// to keep existing cookies valid after cutover.
func GuildToken(secret, realmName, guild string) string {
	sum := sha256.Sum256([]byte(secret + realmName + guild))
	return hex.EncodeToString(sum[:])
}

// VerifyGuildToken returns true for public realms unconditionally and
// performs a constant-time-ish string compare otherwise.
func VerifyGuildToken(secret, realmName, guild, token string) bool {
	if realm.IsPublic(realmName) {
		return true
	}
	return strings.EqualFold(token, GuildToken(secret, realmName, guild))
}
