package sessionize

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Vliysl/roblox-studio-doctor/internal/parse"
	"github.com/Vliysl/roblox-studio-doctor/internal/scan"
)

func build(t *testing.T, lines string) Session {
	t.Helper()
	evs, cov, err := parse.Read(strings.NewReader(lines))
	if err != nil {
		t.Fatal(err)
	}
	return Build(scan.SessionFile{Build: "0.737.0.7371584"}, evs, cov)
}

func TestBuildExtractsDisconnect(t *testing.T) {
	s := build(t, `2026-07-13T23:31:35.625Z,665.625732,0128,6,Warning [FLog::TeamCreateManager] Disconnected due to DisconnectClientInitiated (285). LostConnection = false`+"\n")
	if len(s.Disconnects) != 1 {
		t.Fatalf("got %d disconnects, want 1", len(s.Disconnects))
	}
	d := s.Disconnects[0]
	if d.Reason != "DisconnectClientInitiated" {
		t.Errorf("reason = %q", d.Reason)
	}
	if d.Code != 285 {
		t.Errorf("code = %d", d.Code)
	}
	if d.LostConnection {
		t.Errorf("LostConnection should be false")
	}
}

func TestBuildLostConnection(t *testing.T) {
	s := build(t, `2026-08-02T02:08:14.000Z,100.0,0128,6,Warning [FLog::TeamCreateManager] Disconnected due to TimeAsleepDisconnectThreshold (17). LostConnection = true`+"\n")
	if len(s.Disconnects) != 1 || !s.Disconnects[0].LostConnection {
		t.Fatalf("got %+v, want LostConnection true", s.Disconnects)
	}
}

func TestBuildMemoryForms(t *testing.T) {
	in := strings.Join([]string{
		`2026-07-11T11:41:01.898Z,40316.898438,0024,6,Info [FLog::AppMemUsageStatus] 3616824723`,
		`2026-07-11T01:19:01.448Z,2996.448975,0180,6,Warning [FLog::AppMemUsageStatus] 3748799858.1044687361`,
	}, "\n") + "\n"
	s := build(t, in)
	if len(s.Memory) != 3 {
		t.Fatalf("got %d samples, want 3", len(s.Memory))
	}
	if s.Memory[0].Bytes != 3616824723 {
		t.Errorf("sample 0 = %d", s.Memory[0].Bytes)
	}
	if s.Memory[1].Bytes != 3748799858 || s.Memory[2].Bytes != 1044687361 {
		t.Errorf("dotted pair mis-parsed: %d, %d", s.Memory[1].Bytes, s.Memory[2].Bytes)
	}
}

func TestBuildRecordsMemorySlot(t *testing.T) {
	in := strings.Join([]string{
		`2026-07-11T11:41:01.898Z,40316.898438,0024,6,Info [FLog::AppMemUsageStatus] 3616824723`,
		`2026-07-11T01:19:01.448Z,2996.448975,0180,6,Warning [FLog::AppMemUsageStatus] 3748799858.1044687361`,
	}, "\n") + "\n"
	s := build(t, in)
	want := []int{0, 0, 1}
	if len(s.Memory) != len(want) {
		t.Fatalf("got %d samples, want %d", len(s.Memory), len(want))
	}
	for i, w := range want {
		if s.Memory[i].Slot != w {
			t.Errorf("sample %d slot = %d, want %d", i, s.Memory[i].Slot, w)
		}
	}
}

func TestBuildCleanExit(t *testing.T) {
	in := strings.Join([]string{
		`2026-07-11T11:41:01.898Z,1.0,0024,6,Info [FLog::StudioApplicationState] AboutToQuit`,
		`2026-07-11T11:41:02.898Z,2.0,0024,6,Info [FLog::StudioApplicationState] LastWindowClosed`,
	}, "\n") + "\n"
	if s := build(t, in); !s.CleanExit {
		t.Error("want CleanExit true")
	}
}

func TestBuildCrashHasNoCleanExit(t *testing.T) {
	in := `2026-07-11T11:41:01.898Z,1.0,0024,6,Info [FLog::AppMemUsageStatus] 100` + "\n"
	if s := build(t, in); s.CleanExit {
		t.Error("want CleanExit false")
	}
}

func errLine(ts, payload string) string {
	return ts + `,1.0,0118,6,Error [FLog::CreatorError] ` + payload
}

func TestBuildDedupesScriptErrors(t *testing.T) {
	in := strings.Join([]string{
		errLine("2026-07-11T11:41:01.898Z", `Error: ReplicatedStorage.ArtOfWar.Modules.ArmyView:1318: attempt to index nil with 'reduced'`),
		errLine("2026-07-11T11:41:02.898Z", `Error: ReplicatedStorage.ArtOfWar.Modules.Other:12: boom`),
		errLine("2026-07-11T11:41:03.898Z", `Error: ReplicatedStorage.ArtOfWar.Modules.ArmyView:1318: attempt to index nil with 'reduced'`),
		errLine("2026-07-11T11:41:04.898Z", `Error: ReplicatedStorage.ArtOfWar.Modules.ArmyView:1318: attempt to index nil with 'reduced'`),
	}, "\n") + "\n"
	s := build(t, in)
	if len(s.ScriptErrors) != 2 {
		t.Fatalf("got %d distinct errors, want 2: %+v", len(s.ScriptErrors), s.ScriptErrors)
	}
	top := s.ScriptErrors[0]
	if top.Count != 3 {
		t.Errorf("top count = %d, want 3", top.Count)
	}
	if top.Path != "ReplicatedStorage.ArtOfWar.Modules.ArmyView" || top.Line != 1318 {
		t.Errorf("path/line = %q:%d", top.Path, top.Line)
	}
	if top.Message != "attempt to index nil with 'reduced'" {
		t.Errorf("message = %q", top.Message)
	}
	if !top.FirstWall.Equal(mustTime(t, "2026-07-11T11:41:01.898Z")) {
		t.Errorf("first wall = %v", top.FirstWall)
	}
	if !top.LastWall.Equal(mustTime(t, "2026-07-11T11:41:04.898Z")) {
		t.Errorf("last wall = %v", top.LastWall)
	}
	if s.ScriptErrors[1].Count != 1 {
		t.Errorf("second entry count = %d, want 1", s.ScriptErrors[1].Count)
	}
}

func TestBuildErrorNoPath(t *testing.T) {
	in := errLine("2026-07-11T11:41:01.898Z", `Error: Unable to load plugin icon: rbxassetid://11058574694`) + "\n"
	s := build(t, in)
	if len(s.ScriptErrors) != 1 {
		t.Fatalf("got %d errors, want 1", len(s.ScriptErrors))
	}
	e := s.ScriptErrors[0]
	if e.Path != "" || e.Line != 0 {
		t.Errorf("want no path/line, got %q:%d", e.Path, e.Line)
	}
	if e.Message != "Unable to load plugin icon: rbxassetid://11058574694" {
		t.Errorf("message = %q", e.Message)
	}
}

func TestBuildCountsScriptWarnings(t *testing.T) {

	in := strings.Join([]string{
		`2026-07-11T11:41:01.898Z,1.0,0118,6,Warning [FLog::CreatorWarning] Warning: Infinite yield possible on 'Players.X:WaitForChild("Y")'`,
		`2026-07-11T11:41:02.898Z,2.0,0118,6,Warning [FLog::CreatorWarning] Warning: something else entirely`,
	}, "\n") + "\n"
	s := build(t, in)
	if s.ScriptWarningCount != 2 {
		t.Errorf("warning count = %d, want 2", s.ScriptWarningCount)
	}
}

func TestBuildAssetNoise(t *testing.T) {
	asset := func(ts, payload string) string {
		return ts + `,1.0,0024,6,Info [FLog::AssetAccessDataModelObserver] ` + payload
	}
	in := strings.Join([]string{
		asset("2026-07-11T11:41:01.898Z", "Connected to ContentProvider assetFetchFailedNoExperienceAccess signal"),
		asset("2026-07-11T11:41:02.898Z", "Entering message receiver"),
		asset("2026-07-11T11:41:03.898Z", "Received assetFetchFailedNoExperienceAccess signal for asset ID 1239927101, expected type Animation"),
		asset("2026-07-11T11:41:04.898Z", "Exiting message receiver"),
		asset("2026-07-11T11:41:05.898Z", "Received assetFetchFailedNoExperienceAccess signal for asset ID 1239927101, expected type Animation"),
		asset("2026-07-11T11:41:06.898Z", "Received assetFetchFailedNoExperienceAccess signal for asset ID 92114724928102, expected type Model"),
	}, "\n") + "\n"
	s := build(t, in)
	if len(s.AssetAccessFailures) != 2 {
		t.Fatalf("got %d failures, want 2: %+v", len(s.AssetAccessFailures), s.AssetAccessFailures)
	}
	first := s.AssetAccessFailures[0]
	if first.AssetID != "1239927101" || first.AssetType != "Animation" || first.Count != 2 {
		t.Errorf("first = %+v, want count 2", first)
	}
	if s.AssetAccessFailures[1].AssetID != "92114724928102" {
		t.Errorf("second = %+v", s.AssetAccessFailures[1])
	}
}

func TestBuildPlaytestTimes(t *testing.T) {

	in := strings.Join([]string{
		`2026-07-13T00:17:14.813Z,583.813599,0024,6 [FLog::StudioTimingLog] ======== Studio Play Testing Times =======`,
		`2026-07-13T00:17:15.813Z,584.813599,0024,6 [FLog::StudioTimingLog] PlaySoloStartTotalTime     : 8.6300 sec`,
		`2026-07-13T00:27:15.813Z,1184.813599,0024,6 [FLog::StudioTimingLog] PlaySoloStartTotalTime     : 3.1934 sec`,
	}, "\n") + "\n"
	s := build(t, in)
	if len(s.Playtests) != 2 {
		t.Fatalf("got %d playtests, want 2: %+v", len(s.Playtests), s.Playtests)
	}
	if s.Playtests[0].LoadSeconds != 8.63 {
		t.Errorf("first load = %v, want 8.63", s.Playtests[0].LoadSeconds)
	}
	if s.Playtests[1].LoadSeconds != 3.1934 {
		t.Errorf("second load = %v, want 3.1934", s.Playtests[1].LoadSeconds)
	}
	if !s.Playtests[0].Wall.Equal(mustTime(t, "2026-07-13T00:17:15.813Z")) {
		t.Errorf("first wall = %v", s.Playtests[0].Wall)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	w, err := time.Parse("2006-01-02T15:04:05.999Z", s)
	if err != nil {
		t.Fatal(err)
	}
	return w.UTC()
}

func TestBuildRanksOwnErrors(t *testing.T) {
	var lines []string
	add := func(n int, payload string) {
		for i := 0; i < n; i++ {
			lines = append(lines, errLine("2026-07-11T11:41:01.898Z", payload))
		}
	}
	add(9, `Error: Not running script because past shutdown deadline`)
	add(2, `Error: ReplicatedStorage.ArtOfWar.Modules.ArmyView:1318: attempt to index nil with 'reduced'`)
	add(5, `Error: cloud_6230964447.AlignmentPlusPlus.Main:44: bad argument`)
	add(7, `Error: CorePackages.Workspace.Packages._Workspace.Chrome.Chrome:7: nope`)
	s := build(t, strings.Join(lines, "\n")+"\n")

	if len(s.ScriptErrors) != 4 {
		t.Fatalf("got %d distinct errors, want 4: %+v", len(s.ScriptErrors), s.ScriptErrors)
	}
	wantOrigins := []Origin{OriginPlace, OriginUnknown, OriginPlugin, OriginEngine}
	for i, want := range wantOrigins {
		if got := s.ScriptErrors[i].Origin; got != want {
			t.Errorf("Origin = %v, want %v", got, want)
		}
	}
	if top := s.ScriptErrors[0]; top.Count != 2 {
		t.Errorf("top count = %d, want 2", top.Count)
	}
}

func TestOriginByPathRoot(t *testing.T) {
	cases := []struct {
		path string
		want Origin
	}{
		{"ReplicatedStorage.ArtOfWar.Modules.ArmyView", OriginPlace},
		{"ServerScriptService.Combat", OriginPlace},
		{"StarterPlayer.StarterPlayerScripts.Client", OriginPlace},
		{"Workspace", OriginPlace},
		{"cloud_6230964447.AlignmentPlusPlus.Main", OriginPlugin},
		{"cloud_123", OriginPlugin},
		{"CorePackages.Workspace.Packages._Workspace.Chrome", OriginEngine},
		{"RobloxGui.Modules.Something", OriginEngine},
		{"", OriginUnknown},
		{"cloud_notanid.Thing", OriginUnknown},
		{"SomethingNobodyHasSeen.Script", OriginUnknown},
	}
	for _, c := range cases {
		if got := originOf(c.path); got != c.want {
			t.Errorf("originOf(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestOriginMarshalsAsAName(t *testing.T) {
	b, err := json.Marshal(ScriptError{Path: "CoreGui.X", Origin: OriginEngine})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"origin":"engine"`) {
		t.Errorf("origin must serialise as its name: %s", b)
	}
}
