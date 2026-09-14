package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestGuildToken_MatchesTSAlgorithm(t *testing.T) {
	// Reference value computed the same way as the TS implementation:
	// SHA256(secret + realm + guild) hex.
	secret := "test-secret"
	realm := "Helios"
	guild := "Acme"

	want := func() string {
		s := sha256.Sum256([]byte(secret + realm + guild))
		return hex.EncodeToString(s[:])
	}()

	got := GuildToken(secret, realm, guild)
	if got != want {
		t.Fatalf("token mismatch:\n got  %s\n want %s", got, want)
	}
}

func TestVerifyGuildToken_PublicRealmAlwaysOK(t *testing.T) {
	// Helios is a public realm; verification must pass regardless of token.
	if !VerifyGuildToken("any", "Helios", "Acme", "garbage") {
		t.Fatal("public realm verification should pass")
	}
}

func TestVerifyGuildToken_PrivateRealmRequiresMatch(t *testing.T) {
	secret, realm, guild := "s", "MoPPvE", "Acme"
	good := GuildToken(secret, realm, guild)
	if !VerifyGuildToken(secret, realm, guild, good) {
		t.Fatal("correct token rejected")
	}
	if VerifyGuildToken(secret, realm, guild, "wrong") {
		t.Fatal("wrong token accepted")
	}
}
