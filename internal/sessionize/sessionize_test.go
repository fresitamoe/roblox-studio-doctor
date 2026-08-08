package sessionize

import (
	"strings"
	"testing"

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
