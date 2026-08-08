package rules

import (
	"testing"
	"time"

	"github.com/Vliysl/roblox-studio-doctor/internal/sessionize"
)

func find(fs []Finding, rule string) *Finding {
	for i := range fs {
		if fs[i].Rule == rule {
			return &fs[i]
		}
	}
	return nil
}

func TestLostConnectionFires(t *testing.T) {
	s := sessionize.Session{
		CleanExit: true,
		Disconnects: []sessionize.Disconnect{
			{Reason: "TimeAsleepDisconnectThreshold", Code: 17, LostConnection: true},
		},
	}
	f := find(Apply(s), "teamcreate-lost-connection")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if f.Severity != Critical {
		t.Errorf("severity = %q, want critical", f.Severity)
	}
	if len(f.Evidence) == 0 {
		t.Error("finding must cite evidence")
	}
}

func TestCleanQuitDoesNotFire(t *testing.T) {
	s := sessionize.Session{
		CleanExit: true,
		Disconnects: []sessionize.Disconnect{
			{Reason: "DisconnectClientInitiated", Code: 285, LostConnection: false},
		},
	}
	if f := find(Apply(s), "teamcreate-lost-connection"); f != nil {
		t.Fatalf("must not fire on a clean quit: %+v", f)
	}
}

func TestCrashFiresWhenNoCleanExit(t *testing.T) {
	s := sessionize.Session{CleanExit: false}
	if f := find(Apply(s), "crash-no-clean-exit"); f == nil {
		t.Fatal("rule did not fire")
	}
}

func TestMemoryGrowthFires(t *testing.T) {
	base := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	s := sessionize.Session{
		CleanExit: true,
		Start:     base,
		End:       base.Add(2 * time.Hour),
		Memory: []sessionize.MemSample{
			{Wall: base, Bytes: 2_000_000_000},
			{Wall: base.Add(time.Hour), Bytes: 6_000_000_000},
			{Wall: base.Add(2 * time.Hour), Bytes: 12_000_000_000},
		},
	}
	f := find(Apply(s), "memory-growth")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if len(f.Evidence) == 0 {
		t.Error("finding must cite evidence")
	}
}

func TestHealthySessionIsClean(t *testing.T) {
	base := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	s := sessionize.Session{
		CleanExit: true,
		Start:     base,
		End:       base.Add(time.Hour),
		Memory: []sessionize.MemSample{
			{Wall: base, Bytes: 3_000_000_000},
			{Wall: base.Add(time.Hour), Bytes: 3_100_000_000},
		},
	}
	if got := Apply(s); len(got) != 0 {
		t.Fatalf("healthy session produced %+v", got)
	}
}

func TestEveryFindingCitesEvidence(t *testing.T) {
	s := sessionize.Session{
		CleanExit:   false,
		Disconnects: []sessionize.Disconnect{{Reason: "X", Code: 1, LostConnection: true}},
	}
	for _, f := range Apply(s) {
		if len(f.Evidence) == 0 {
			t.Errorf("finding %q has no evidence", f.Rule)
		}
	}
}
