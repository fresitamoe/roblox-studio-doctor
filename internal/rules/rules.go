package rules

import (
	"fmt"
	"strings"
	"time"

	"github.com/fresitamoe/roblox-studio-doctor/internal/sessionize"
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

	playtestSlowdownFactor       = 2.5
	playtestSlowdownFloorSeconds = 10.0

	scriptErrorEvidenceLimit = 5
	scriptErrorMessageChars  = 120

	assetFailureEvidenceLimit = 5
)

type rule func(sessionize.Session) *Finding

var all = []rule{
	teamCreateLostConnection,
	crashNoCleanExit,
	scriptErrors,
	assetAccessDenied,
	playtestSlowdown,
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
			Summary:  "Still running, so no shutdown sequence yet. Nothing wrong here",
			Evidence: []string{"log was still being written, no LastWindowClosed event yet"},
		}
	}
	return &Finding{
		Rule:     "crash-no-clean-exit",
		Severity: Warn,
		Summary:  "No shutdown sequence in the log, so it crashed or got killed",
		Evidence: []string{"no LastWindowClosed event in the log"},
	}
}

func scriptErrors(s sessionize.Session) *Finding {
	if len(s.ScriptErrors) == 0 {
		return nil
	}
	total, placeTotal, placeDistinct := 0, 0, 0
	for _, e := range s.ScriptErrors {
		total += e.Count
		if e.Origin == sessionize.OriginPlace {
			placeTotal += e.Count
			placeDistinct++
		}
	}
	ev := make([]string, 0, scriptErrorEvidenceLimit)
	for _, e := range s.ScriptErrors {
		if len(ev) == scriptErrorEvidenceLimit {
			break
		}
		ev = append(ev, fmt.Sprintf("%dx %s%s%s",
			e.Count, originTag(e.Origin), location(e),
			truncate(e.Message, scriptErrorMessageChars)))
	}

	severity, ownership := Info, "none of them in your own scripts"
	if placeDistinct > 0 {
		severity = Warn
		ownership = fmt.Sprintf("%d error(s) from %d of those, in your own scripts",
			placeTotal, placeDistinct)
	}
	return &Finding{
		Rule:     "script-errors",
		Severity: severity,
		Summary: fmt.Sprintf("%d error(s) from %d distinct problem(s), %s",
			total, len(s.ScriptErrors), ownership),
		Evidence: ev,
	}
}

func originTag(o sessionize.Origin) string {
	if o == sessionize.OriginPlace {
		return ""
	}
	return fmt.Sprintf("[%s] ", o)
}

func location(e sessionize.ScriptError) string {
	switch {
	case e.Path == "":
		return ""
	case e.Line == 0:
		return e.Path + ": "
	}
	return fmt.Sprintf("%s:%d: ", e.Path, e.Line)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimRight(string(r[:n]), " ") + "..."
}

func assetAccessDenied(s sessionize.Session) *Finding {
	if len(s.AssetAccessFailures) == 0 {
		return nil
	}
	ev := make([]string, 0, assetFailureEvidenceLimit)
	for _, a := range s.AssetAccessFailures {
		if len(ev) == assetFailureEvidenceLimit {
			break
		}
		ev = append(ev, fmt.Sprintf("asset %s (%s), %d attempt(s)",
			a.AssetID, a.AssetType, a.Count))
	}
	return &Finding{
		Rule:     "asset-access-denied",
		Severity: Warn,
		Summary: fmt.Sprintf("%d asset(s) this session had no access to",
			len(s.AssetAccessFailures)),
		Evidence: ev,
	}
}

func playtestSlowdown(s sessionize.Session) *Finding {
	if len(s.Playtests) < 2 {
		return nil
	}
	first, slowest := s.Playtests[0], s.Playtests[0]
	for _, p := range s.Playtests {
		if p.LoadSeconds > slowest.LoadSeconds {
			slowest = p
		}
	}
	if slowest.LoadSeconds < playtestSlowdownFloorSeconds {
		return nil
	}
	if first.LoadSeconds <= 0 ||
		slowest.LoadSeconds < first.LoadSeconds*playtestSlowdownFactor {
		return nil
	}
	return &Finding{
		Rule:     "playtest-slowdown",
		Severity: Warn,
		Summary: fmt.Sprintf("Playtest loads got slower, %.1fs up from %.1fs across %d playtest(s)",
			slowest.LoadSeconds, first.LoadSeconds, len(s.Playtests)),
		Evidence: []string{
			fmt.Sprintf("first playtest loaded in %.4f sec at %s",
				first.LoadSeconds, first.Wall.Format("15:04:05")),
			fmt.Sprintf("slowest playtest loaded in %.4f sec at %s",
				slowest.LoadSeconds, slowest.Wall.Format("15:04:05")),
		},
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
