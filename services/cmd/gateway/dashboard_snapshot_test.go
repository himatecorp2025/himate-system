package main

import (
	"testing"
	"time"
)

func TestDashboardSnapshotForReadCurrentYearIsMemoryOnly(t *testing.T) {
	a := &app{}
	payload, updatedAt, stale := a.dashboardSnapshotForRead(time.Now().UTC().Year())
	if payload != nil {
		t.Fatalf("expected no current-year payload while the in-memory snapshot is warming, got %#v", payload)
	}
	if !updatedAt.IsZero() {
		t.Fatalf("expected zero updated_at while warming, got %s", updatedAt)
	}
	if !stale {
		t.Fatal("expected warming current-year snapshot to be marked stale")
	}
}

func TestDashboardSnapshotForReadCurrentYearReturnsHotMemorySnapshot(t *testing.T) {
	now := time.Now().UTC()
	a := &app{
		dashboardPayload: map[string]any{"year": now.Year(), "meta": map[string]any{"status": "healthy"}},
		dashboardUpdatedAt: now,
		dashboardExpires: now.Add(time.Minute),
	}
	payload, updatedAt, stale := a.dashboardSnapshotForRead(now.Year())
	if payload == nil || payload["year"] != now.Year() {
		t.Fatalf("expected current-year in-memory snapshot, got %#v", payload)
	}
	if !updatedAt.Equal(now) {
		t.Fatalf("updated_at = %s, want %s", updatedAt, now)
	}
	if stale {
		t.Fatal("fresh in-memory snapshot unexpectedly marked stale")
	}
}
