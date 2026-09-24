package main

import (
	"testing"
	"time"
)

func TestStableReplayTimesReusesPersistedServerDefaults(t *testing.T) {
	firstOccurred := time.Date(2026, 9, 24, 18, 58, 14, 0, time.UTC)
	firstAvailable := firstOccurred
	retryOccurred := firstOccurred.Add(2 * time.Minute)
	retryAvailable := retryOccurred

	occurred, available := stableReplayTimes(
		retryOccurred,
		retryAvailable,
		false,
		false,
		firstOccurred,
		firstAvailable,
	)
	if !occurred.Equal(firstOccurred) || !available.Equal(firstAvailable) {
		t.Fatalf("server-defaulted replay times must reuse persisted values: occurred=%s available=%s", occurred, available)
	}
}

func TestStableReplayTimesPreservesExplicitProducerTimes(t *testing.T) {
	existingOccurred := time.Date(2026, 9, 24, 18, 58, 14, 0, time.UTC)
	existingAvailable := existingOccurred
	explicitOccurred := existingOccurred.Add(time.Hour)
	explicitAvailable := explicitOccurred.Add(15 * time.Minute)

	occurred, available := stableReplayTimes(
		explicitOccurred,
		explicitAvailable,
		true,
		true,
		existingOccurred,
		existingAvailable,
	)
	if !occurred.Equal(explicitOccurred) || !available.Equal(explicitAvailable) {
		t.Fatalf("explicit producer times must remain authoritative: occurred=%s available=%s", occurred, available)
	}
}

func TestStableReplayTimesCanMixExplicitAndServerDefaultTimes(t *testing.T) {
	existingOccurred := time.Date(2026, 9, 24, 18, 58, 14, 0, time.UTC)
	existingAvailable := existingOccurred.Add(30 * time.Minute)
	explicitAvailable := existingAvailable.Add(time.Hour)

	occurred, available := stableReplayTimes(
		time.Now().UTC(),
		explicitAvailable,
		false,
		true,
		existingOccurred,
		existingAvailable,
	)
	if !occurred.Equal(existingOccurred) {
		t.Fatalf("omitted occurred_at must reuse persisted server default: %s", occurred)
	}
	if !available.Equal(explicitAvailable) {
		t.Fatalf("explicit available_at must be preserved: %s", available)
	}
}
