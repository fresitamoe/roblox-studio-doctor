package rules

import (
	"fmt"
	"time"

	"github.com/Vliysl/roblox-studio-doctor/internal/sessionize"
)

// Severity is how serious a finding is
type Severity string

const (
	Info     Severity = "info"
	Warn     Severity = "warn"
	Critical Severity = "critical"
)

// Finding is a single reported issue
type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Summary  string   `json:"summary"`
	Evidence []string `json:"evidence"`
}

var (
	memGrowthFactor = 2.0

	memGrowthFloorBytes int64 = 2 << 30
)

type rule func(sessionize.Session) *Finding

var all = []rule{
	teamCreateLostConnection,
	crashNoCleanExit,
	memoryGrowth,
}

// Apply runs every active rule and returns the ones that fired
func Apply(s sessionize.Session) []Finding {
	var out []Finding
	for _, r := range all {
		if f := r(s); f != nil {
			out = append(out, *f)
		}
	}
	return out
}

func teamCreateLostConnection(s sessionize.Session) *Finding {
	var ev []string
	for _, d := range s.Disconnects {
		if d.LostConnection {
			ev = append(ev, fmt.Sprintf("line %d: lost connection, reason %s (%d)",
				d.LineNo, d.Reason, d.Code))
		}
	}
	if len(ev) == 0 {
		return nil
	}
	return &Finding{
		Rule:     "teamcreate-lost-connection",
		Severity: Critical,
		Summary:  fmt.Sprintf("Team Create dropped the session %d time(s), anything edited after that might not have reached the server", len(ev)),
		Evidence: ev,
	}
}

func crashNoCleanExit(s sessionize.Session) *Finding {
	if s.CleanExit {
		return nil
	}
	if s.Ongoing {
		return &Finding{
			Rule:     "crash-no-clean-exit",
			Severity: Info,
			Summary:  "Session is still in progress — Studio has not logged its shutdown sequence yet. Nothing is wrong.",
			Evidence: []string{"log was still being written, no LastWindowClosed event yet"},
		}
	}
	return &Finding{
		Rule:     "crash-no-clean-exit",
		Severity: Warn,
		Summary:  "Session ended without Studio's shutdown sequence — it crashed or was killed.",
		Evidence: []string{"no LastWindowClosed event in the log"},
	}
}

// Not in the active list. AppMemUsageStatus turned out to be static tiers rather
// than a memory series, so this can't really fire anyways
func memoryGrowth(s sessionize.Session) *Finding {
	var series []sessionize.MemSample
	for _, m := range s.Memory {
		if m.Slot == 0 {
			series = append(series, m)
		}
	}
	if len(series) < 2 {
		return nil
	}
	first, peak := series[0], series[0]
	for _, m := range series {
		if m.Bytes > peak.Bytes {
			peak = m
		}
	}
	if peak.Bytes < memGrowthFloorBytes {
		return nil
	}
	if float64(peak.Bytes) < float64(first.Bytes)*memGrowthFactor {
		return nil
	}
	span := peak.Wall.Sub(first.Wall)
	if span < 0 {
		span = 0
	}
	return &Finding{
		Rule:     "memory-growth",
		Severity: Warn,
		Summary: fmt.Sprintf("Studio memory went from %.1f GB to %.1f GB over %s",
			gib(first.Bytes), gib(peak.Bytes), span.Round(time.Second)),
		Evidence: []string{
			fmt.Sprintf("first sample %d bytes at %s", first.Bytes, first.Wall.Format("15:04:05")),
			fmt.Sprintf("peak sample %d bytes at %s", peak.Bytes, peak.Wall.Format("15:04:05")),
		},
	}
}

func gib(b int64) float64 { return float64(b) / (1 << 30) }
