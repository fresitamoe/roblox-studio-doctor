package parse

import (
	"strings"
	"testing"
	"time"
)

const teamCreateLine = `2026-07-13T23:31:35.625Z,665.625732,0128,6,Warning [FLog::TeamCreateManager] Disconnected due to DisconnectClientInitiated (285). LostConnection = false`

func TestReadParsesFLogLine(t *testing.T) {
	evs, cov, err := Read(strings.NewReader(teamCreateLine + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	e := evs[0]
	if !e.Wall.Equal(time.Date(2026, 7, 13, 23, 31, 35, 625000000, time.UTC)) {
		t.Errorf("wall = %v", e.Wall)
	}
	if e.Mono != 665.625732 {
		t.Errorf("mono = %v", e.Mono)
	}
	if e.Thread != "0128" {
		t.Errorf("thread = %q", e.Thread)
	}
	if e.Severity != "Warning" {
		t.Errorf("severity = %q", e.Severity)
	}
	if e.Channel != "TeamCreateManager" {
		t.Errorf("channel = %q", e.Channel)
	}
	if !strings.HasPrefix(e.Message, "Disconnected due to") {
		t.Errorf("message = %q", e.Message)
	}
	if cov.Parsed != 1 || cov.Total != 1 {
		t.Errorf("coverage = %+v", cov)
	}
}

func TestReadLogGroup(t *testing.T) {
	line := `2026-07-13T23:31:35.631Z,665.631714,0128,6,Error [logGroup] Client Peer updating state from Connected -> Closing`
	evs, _, err := Read(strings.NewReader(line + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Channel != "logGroup" {
		t.Fatalf("got %+v, want channel logGroup", evs)
	}
}

func TestReadTruncatedTail(t *testing.T) {
	in := teamCreateLine + "\n" + `2026-07-13T23:31:36.000Z,666.0,0128,6,Info [FLog::Part`
	evs, cov, err := Read(strings.NewReader(in))
	if err != nil {
		t.Fatalf("truncated tail must not error, got %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1 complete event", len(evs))
	}
	if cov.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", cov.Skipped)
	}
}

func TestReadUnknownChannel(t *testing.T) {
	in := `2026-07-13T23:31:35.625Z,665.625732,0128,6,Info [FLog::SomeFutureChannel] hello` + "\n"
	evs, cov, err := Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	if cov.Ratio() != 1.0 {
		t.Errorf("ratio = %v, want 1.0", cov.Ratio())
	}
}

func TestCoverageRatioEmpty(t *testing.T) {
	if got := (Coverage{}).Ratio(); got != 1.0 {
		t.Errorf("empty coverage ratio = %v, want 1.0", got)
	}
}

func TestReadFractionalSeconds(t *testing.T) {
	for _, ts := range []string{
		"2026-07-13T23:31:35.625Z",
		"2026-07-13T23:31:35.6Z",
		"2026-07-13T23:31:35.625123Z",
	} {
		line := ts + `,665.625732,0128,6,Info [FLog::TeamCreateManager] hello`
		evs, cov, err := Read(strings.NewReader(line + "\n"))
		if err != nil {
			t.Fatalf("%s: %v", ts, err)
		}
		if len(evs) != 1 || cov.Skipped != 0 {
			t.Errorf("%s: got %d events, %d skipped", ts, len(evs), cov.Skipped)
		}
	}
}

func TestReadOverlongNoError(t *testing.T) {
	huge := strings.Repeat("x", 2<<20)
	in := teamCreateLine + "\n" + huge + "\n"
	evs, cov, err := Read(strings.NewReader(in))
	if err != nil {
		t.Fatalf("bad content must never error, got %v", err)
	}
	if len(evs) != 1 {
		t.Errorf("got %d events, want the 1 good line", len(evs))
	}
	if cov.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", cov.Skipped)
	}
}
