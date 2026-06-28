// Package domain holds pure logic ported from packages/core/src.
package domain

import "time"

// RaidLockWindow is a half-open interval [Start, End) covering one weekly raid lock.
type RaidLockWindow struct {
	Start time.Time
	End   time.Time
}

// RaidLock mirrors packages/core/src/date.ts:raidLock.
//
// A raid lock starts on Wednesday 06:00 UTC and lasts 7 days.
// `shift` rewinds the start by N weeks (0 = current lock).
//
// Returned times are UTC.
func RaidLock(at time.Time, shift int) RaidLockWindow {
	start := raidLockStart(at)
	if shift > 0 {
		start = start.AddDate(0, 0, -7*shift)
	}
	end := start.AddDate(0, 0, 7)
	return RaidLockWindow{Start: start, End: end}
}

func raidLockStart(at time.Time) time.Time {
	t := at.UTC()
	// Walk back to the most recent Wednesday (inclusive).
	// time.Weekday: Sunday=0..Saturday=6, Wednesday=3.
	weekday := int(t.Weekday())
	deltaDays := (weekday - int(time.Wednesday) + 7) % 7
	wed := t.AddDate(0, 0, -deltaDays)
	wedAt6 := time.Date(wed.Year(), wed.Month(), wed.Day(), 6, 0, 0, 0, time.UTC)
	// If `at` falls earlier than Wed 06:00 on this Wednesday (e.g. Wed 04:00),
	// the lock that's currently active is the previous one.
	if !wedAt6.After(t) {
		return wedAt6
	}
	return wedAt6.AddDate(0, 0, -7)
}
