package rules

import (
	"strings"
	"testing"
	"time"

	"github.com/fresitamoe/roblox-studio-doctor/internal/parse"
	"github.com/fresitamoe/roblox-studio-doctor/internal/scan"
	"github.com/fresitamoe/roblox-studio-doctor/internal/sessionize"
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
	f := memoryGrowth(s)
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
	if f := memoryGrowth(s); f != nil {
		t.Fatalf("fired across slots: %+v", f)
	}
}

func TestMemoryGrowthDottedValues(t *testing.T) {
	s := buildFrom(t,
		memLine("2026-07-12T17:28:22.764Z", "2492914692.1044687361"),
		memLine("2026-07-12T17:28:22.828Z", "3748799858.1044687361"),
		memLine("2026-07-12T17:32:01.391Z", "3784003596.1044687361"),
	)
	if f := memoryGrowth(s); f != nil {
		t.Fatalf("should not fire: %+v", f)
	}
}

func TestMemoryGrowthSlot0(t *testing.T) {
	s := buildFrom(t,
		memLine("2026-07-12T17:00:00.000Z", "2000000000.1044687361"),
		memLine("2026-07-12T17:30:00.000Z", "4000000000.1044687361"),
		memLine("2026-07-12T18:00:00.000Z", "9000000000.1044687361"),
	)
	f := memoryGrowth(s)
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
	f := memoryGrowth(s)
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

func TestMemoryGrowthIsDormant(t *testing.T) {
	base := time.Date(2026, 8, 3, 17, 0, 0, 0, time.UTC)
	s := sessionize.Session{
		CleanExit: true,
		Start:     base,
		End:       base.Add(time.Hour),
		Memory: []sessionize.MemSample{
			{Wall: base, Bytes: 2_000_000_000, Slot: 0},
			{Wall: base.Add(time.Hour), Bytes: 9_000_000_000, Slot: 0},
		},
	}
	if memoryGrowth(s) == nil {
		t.Fatal("setup is wrong, rule should fire")
	}
	if f := find(Apply(s), "memory-growth"); f != nil {
		t.Errorf("memory-growth should be off: %+v", f)
	}
	if got := Apply(s); len(got) != 0 {
		t.Errorf("healthy session produced %+v", got)
	}
}

func TestActiveRuleCount(t *testing.T) {
	if len(all) != 5 {
		t.Fatalf("active rule set has %d rules, want 5", len(all))
	}
	s := sessionize.Session{
		CleanExit:           false,
		Disconnects:         []sessionize.Disconnect{{Reason: "X", Code: 1, LostConnection: true}},
		ScriptErrors:        []sessionize.ScriptError{{Path: "A", Line: 1, Message: "boom", Count: 2}},
		AssetAccessFailures: []sessionize.AssetAccessFailure{{AssetID: "1", AssetType: "Animation", Count: 1}},
		Playtests:           playtests(5.0, 20.0),
	}
	got := map[string]bool{}
	for _, f := range Apply(s) {
		got[f.Rule] = true
	}
	for _, want := range []string{
		"teamcreate-lost-connection",
		"crash-no-clean-exit",
		"script-errors",
		"asset-access-denied",
		"playtest-slowdown",
	} {
		if !got[want] {
			t.Errorf("rule %q did not fire", want)
		}
	}
	if len(got) != 5 {
		t.Errorf("unexpected active rules: %v", got)
	}
}

func TestScriptErrorsTotals(t *testing.T) {
	s := sessionize.Session{
		CleanExit: true,
		ScriptErrors: []sessionize.ScriptError{
			{Path: "ReplicatedStorage.ArmyView", Line: 1318, Message: "attempt to index nil with 'reduced'", Count: 62_000},
			{Path: "ServerScriptService.Combat", Line: 42, Message: "attempt to call a nil value", Count: 684},
		},
	}
	f := find(Apply(s), "script-errors")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if f.Severity != Warn {
		t.Errorf("severity = %q, want warn", f.Severity)
	}
	if !strings.Contains(f.Summary, "62684") || !strings.Contains(f.Summary, "2 distinct") {
		t.Errorf("summary = %q", f.Summary)
	}
	if len(f.Evidence) == 0 {
		t.Fatal("finding must cite evidence")
	}
	if !strings.Contains(f.Evidence[0], "62000x") ||
		!strings.Contains(f.Evidence[0], "ReplicatedStorage.ArmyView:1318") {
		t.Errorf("evidence[0] = %q", f.Evidence[0])
	}
}

func TestScriptErrorsEvidenceCap(t *testing.T) {
	var errs []sessionize.ScriptError
	for i := 0; i < 12; i++ {
		errs = append(errs, sessionize.ScriptError{
			Path:    "ServerScriptService.Script",
			Line:    i,
			Message: strings.Repeat("very long error text ", 40),
			Count:   20 - i,
		})
	}
	f := find(Apply(sessionize.Session{CleanExit: true, ScriptErrors: errs}), "script-errors")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if len(f.Evidence) != scriptErrorEvidenceLimit {
		t.Errorf("cited %d errors, want the top %d", len(f.Evidence), scriptErrorEvidenceLimit)
	}
	for i, e := range f.Evidence {
		if len(e) > 2*scriptErrorMessageChars {
			t.Errorf("evidence[%d] is %d chars", i, len(e))
		}
	}
}

func TestAssetAccessDeniedFires(t *testing.T) {
	s := sessionize.Session{
		CleanExit: true,
		AssetAccessFailures: []sessionize.AssetAccessFailure{
			{AssetID: "1239927101", AssetType: "Animation", Count: 39},
			{AssetID: "92114724928102", AssetType: "Model", Count: 35},
		},
	}
	f := find(Apply(s), "asset-access-denied")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if f.Severity != Warn {
		t.Errorf("severity = %q, want warn", f.Severity)
	}
	if !strings.Contains(f.Summary, "2 asset") {
		t.Errorf("summary = %q", f.Summary)
	}
	if len(f.Evidence) != 2 {
		t.Fatalf("evidence = %v", f.Evidence)
	}
	if !strings.Contains(f.Evidence[0], "1239927101") ||
		!strings.Contains(f.Evidence[0], "Animation") ||
		!strings.Contains(f.Evidence[0], "39") {
		t.Errorf("evidence[0] = %q", f.Evidence[0])
	}
}

func playtests(secs ...float64) []sessionize.Playtest {
	base := time.Date(2026, 8, 3, 17, 0, 0, 0, time.UTC)
	var out []sessionize.Playtest
	for i, s := range secs {
		out = append(out, sessionize.Playtest{
			Wall:        base.Add(time.Duration(i) * 10 * time.Minute),
			LoadSeconds: s,
		})
	}
	return out
}

func TestPlaytestNormalRange(t *testing.T) {
	s := sessionize.Session{CleanExit: true, Playtests: playtests(8.6, 10.4, 12.2)}
	if f := find(Apply(s), "playtest-slowdown"); f != nil {
		t.Fatalf("should not fire: %+v", f)
	}
}

func TestPlaytestSlowdownFires(t *testing.T) {
	s := sessionize.Session{CleanExit: true, Playtests: playtests(5.0, 9.0, 15.0)}
	f := find(Apply(s), "playtest-slowdown")
	if f == nil {
		t.Fatal("rule did not fire on a 3x rise to 15s")
	}
	if f.Severity != Warn {
		t.Errorf("severity = %q, want warn", f.Severity)
	}
	if len(f.Evidence) == 0 {
		t.Error("finding must cite evidence")
	}
}

func TestPlaytestFloor(t *testing.T) {
	s := sessionize.Session{CleanExit: true, Playtests: playtests(1.0, 3.0)}
	if f := find(Apply(s), "playtest-slowdown"); f != nil {
		t.Fatalf("should not fire: %+v", f)
	}
}

func TestPlaytestNeedsTwo(t *testing.T) {
	s := sessionize.Session{CleanExit: true, Playtests: playtests(30.0)}
	if f := find(Apply(s), "playtest-slowdown"); f != nil {
		t.Fatalf("one playtest is not a trend: %+v", f)
	}
}

func TestEveryNewRuleCitesEvidence(t *testing.T) {
	s := sessionize.Session{
		CleanExit:           true,
		ScriptErrors:        []sessionize.ScriptError{{Path: "A", Line: 1, Message: "boom", Count: 3}},
		AssetAccessFailures: []sessionize.AssetAccessFailure{{AssetID: "1", AssetType: "Animation", Count: 2}},
		Playtests:           playtests(5.0, 20.0),
	}
	fired := map[string]bool{}
	for _, f := range Apply(s) {
		fired[f.Rule] = true
		if len(f.Evidence) == 0 {
			t.Errorf("finding %q has no evidence", f.Rule)
		}
	}
	for _, want := range []string{"script-errors", "asset-access-denied", "playtest-slowdown"} {
		if !fired[want] {
			t.Errorf("rule %q did not fire", want)
		}
	}
}

func TestHealthyWithPlaytests(t *testing.T) {
	s := sessionize.Session{
		CleanExit:          true,
		ScriptWarningCount: 12,
		Playtests:          playtests(1.9, 2.4, 3.2),
	}
	if got := Apply(s); len(got) != 0 {
		t.Fatalf("healthy session produced %+v", got)
	}
}

func TestScriptErrorsWarnsOnOwnCode(t *testing.T) {
	s := sessionize.Session{
		CleanExit: true,
		ScriptErrors: []sessionize.ScriptError{
			{Path: "ReplicatedStorage.ArmyView", Line: 1318, Message: "attempt to index nil", Count: 2, Origin: sessionize.OriginPlace},
			{Message: "Not running script because past shutdown deadline", Count: 4976, Origin: sessionize.OriginUnknown},
			{Path: "cloud_6230964447.Align", Line: 44, Message: "bad argument", Count: 5, Origin: sessionize.OriginPlugin},
		},
	}
	f := find(Apply(s), "script-errors")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if f.Severity != Warn {
		t.Errorf("severity = %q, want warn", f.Severity)
	}
	if !strings.Contains(f.Summary, "your own scripts") {
		t.Errorf("summary = %q", f.Summary)
	}
	if len(f.Evidence) < 3 {
		t.Fatalf("want the top three cited, got %v", f.Evidence)
	}
	if !strings.Contains(f.Evidence[0], "ReplicatedStorage.ArmyView") {
		t.Errorf("evidence = %q", f.Evidence[0])
	}
	if strings.Contains(f.Evidence[0], "[") {
		t.Errorf("place errors carry no origin tag: %q", f.Evidence[0])
	}
	if !strings.Contains(f.Evidence[1], "[unknown]") {
		t.Errorf("evidence[1] must name its origin: %q", f.Evidence[1])
	}
	if !strings.Contains(f.Evidence[2], "[plugin]") {
		t.Errorf("evidence[2] must name its origin: %q", f.Evidence[2])
	}
}

func TestScriptErrorsInfoOnly(t *testing.T) {
	s := sessionize.Session{
		CleanExit: true,
		ScriptErrors: []sessionize.ScriptError{
			{Message: "Not running script because past shutdown deadline", Count: 4976, Origin: sessionize.OriginUnknown},
			{Path: "CorePackages.Chrome", Line: 7, Message: "nope", Count: 1, Origin: sessionize.OriginEngine},
		},
	}
	f := find(Apply(s), "script-errors")
	if f == nil {
		t.Fatal("rule should still fire")
	}
	if f.Severity != Info {
		t.Errorf("severity = %q, want info", f.Severity)
	}
	if !strings.Contains(f.Summary, "none of them in your own scripts") {
		t.Errorf("summary = %q", f.Summary)
	}
	if len(f.Evidence) == 0 {
		t.Error("finding must cite evidence")
	}
}

func TestScriptErrorsTaggedNoLine(t *testing.T) {

	s := sessionize.Session{
		CleanExit: true,
		ScriptErrors: []sessionize.ScriptError{{
			Path:    "CoreGui.RobloxGui.Modules.Chrome.Integrations.ToggleMic",
			Message: "Not running script because past shutdown deadline",
			Count:   45597,
			Origin:  sessionize.OriginEngine,
		}},
	}
	f := find(Apply(s), "script-errors")
	if f == nil {
		t.Fatal("rule did not fire")
	}
	if f.Severity != Info {
		t.Errorf("severity = %q, want info", f.Severity)
	}
	if len(f.Evidence) == 0 {
		t.Fatal("finding must cite evidence")
	}
	if strings.Contains(f.Evidence[0], ":0:") {
		t.Errorf("bad line number: %q", f.Evidence[0])
	}
	if !strings.Contains(f.Evidence[0], "[engine] CoreGui.RobloxGui.Modules.Chrome.Integrations.ToggleMic: ") {
		t.Errorf("evidence[0] = %q", f.Evidence[0])
	}
}
