package rules

import (
	"strings"
	"testing"
	"time"

	"github.com/Vliysl/roblox-studio-doctor/internal/parse"
	"github.com/Vliysl/roblox-studio-doctor/internal/scan"
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

func buildFrom(t *testing.T, lines ...string) sessionize.Session {
	t.Helper()
	evs, cov, err := parse.Read(strings.NewReader(strings.Join(lines, "\n") + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	return sessionize.Build(scan.SessionFile{Build: "0.737.0.7371584"}, evs, cov)
}

func memLine(ts, payload string) string {
	return ts + `,1.0,0138,6,Warning [FLog::AppMemUsageStatus] ` + payload
}

func TestMemoryGrowthSlots(t *testing.T) {
	s := buildFrom(t,
		memLine("2026-07-12T17:00:00.000Z", "1000000000.5000000000"),
		memLine("2026-07-12T17:30:00.000Z", "1000000000.5000000000"),
		memLine("2026-07-12T18:00:00.000Z", "1000000000.5000000000"),
	)
	if len(s.Memory) != 6 {
		t.Fatalf("got %d samples, want 6", len(s.Memory))
	}
	if f := find(Apply(s), "memory-growth"); f != nil {
		t.Fatalf("fired across slots: %+v", f)
	}
}

func TestMemoryGrowthDottedValues(t *testing.T) {
	s := buildFrom(t,
		memLine("2026-07-12T17:28:22.764Z", "2492914692.1044687361"),
		memLine("2026-07-12T17:28:22.828Z", "3748799858.1044687361"),
		memLine("2026-07-12T17:32:01.391Z", "3784003596.1044687361"),
	)
	if f := find(Apply(s), "memory-growth"); f != nil {
		t.Fatalf("should not fire: %+v", f)
	}
}

func TestMemoryGrowthSlot0(t *testing.T) {
	s := buildFrom(t,
		memLine("2026-07-12T17:00:00.000Z", "2000000000.1044687361"),
		memLine("2026-07-12T17:30:00.000Z", "4000000000.1044687361"),
		memLine("2026-07-12T18:00:00.000Z", "9000000000.1044687361"),
	)
	f := find(Apply(s), "memory-growth")
	if f == nil {
		t.Fatal("real growth in slot 0 must still fire")
	}
	if len(f.Evidence) == 0 {
		t.Error("finding must cite evidence")
	}
	if !strings.Contains(f.Evidence[0], "2000000000") ||
		!strings.Contains(f.Evidence[1], "9000000000") {
		t.Errorf("evidence = %v", f.Evidence)
	}
}

func TestMemoryGrowthSpan(t *testing.T) {
	base := time.Date(2026, 8, 3, 17, 0, 0, 0, time.UTC)
	s := sessionize.Session{
		CleanExit: true,
		Start:     base,
		End:       base.Add(4*time.Hour + 34*time.Minute),
		Memory: []sessionize.MemSample{
			{Wall: base.Add(time.Hour), Bytes: 2_000_000_000, Slot: 0},
			{Wall: base.Add(time.Hour + 4*time.Minute), Bytes: 9_000_000_000, Slot: 0},
		},
	}
	f := find(Apply(s), "memory-growth")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if !strings.Contains(f.Summary, "over 4m0s") {
		t.Errorf("summary = %q, want 4m span", f.Summary)
	}
	if strings.Contains(f.Summary, "4h34m") {
		t.Errorf("summary = %q, wrong span", f.Summary)
	}
}

func TestCrashInfoWhenOngoing(t *testing.T) {
	s := sessionize.Session{CleanExit: false, Ongoing: true}
	f := find(Apply(s), "crash-no-clean-exit")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if f.Severity != Info {
		t.Errorf("severity = %q, want info", f.Severity)
	}
	if strings.Contains(f.Summary, "crashed") {
		t.Errorf("summary = %q, not a crash", f.Summary)
	}
	if len(f.Evidence) == 0 {
		t.Error("finding must cite evidence")
	}
}

func TestCrashWarnWhenDone(t *testing.T) {
	s := sessionize.Session{CleanExit: false, Ongoing: false}
	f := find(Apply(s), "crash-no-clean-exit")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if f.Severity != Warn {
		t.Errorf("severity = %q, want warn", f.Severity)
	}
	if !strings.Contains(f.Summary, "crashed") {
		t.Errorf("summary = %q, want the crash wording", f.Summary)
	}
}
