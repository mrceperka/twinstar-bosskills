package wow

import "testing"

func TestRaidPosition(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"Mogu'shan Vaults", 1},
		{"Mogu’shan Vaults", 1},
		{"Dragon Soul", 11},
		{"Unknown Raid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RaidPosition(tt.name); got != tt.want {
				t.Fatalf("RaidPosition(%q) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestBossPosition(t *testing.T) {
	tests := []struct {
		name     string
		remoteID uint32
		want     int
	}{
		{"first boss in raid", 59915, 1},
		{"later boss in raid", 71865, 14},
		{"unknown", 999999, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BossPosition(tt.remoteID); got != tt.want {
				t.Fatalf("BossPosition(%d) = %d, want %d", tt.remoteID, got, tt.want)
			}
		})
	}
}
