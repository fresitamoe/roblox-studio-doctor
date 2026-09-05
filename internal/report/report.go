package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fresitamoe/roblox-studio-doctor/internal/rules"
	"github.com/fresitamoe/roblox-studio-doctor/internal/sessionize"
)

// Schema is the version tag on the JSON output
const Schema = "studio-doctor/v1"

// Result is one analysed session and whatever the rules found
type Result struct {
	Schema   string             `json:"schema"`
	Session  sessionize.Session `json:"session"`
	Findings []rules.Finding    `json:"findings"`
}

// NewResult pairs a session with its findings, worst first
func NewResult(s sessionize.Session, fs []rules.Finding) Result {
	ranked := make([]rules.Finding, len(fs))
	copy(ranked, fs)
	sort.SliceStable(ranked, func(i, j int) bool {
		return rank(ranked[i].Severity) > rank(ranked[j].Severity)
	})
	return Result{Schema: Schema, Session: s, Findings: ranked}
}

func rank(s rules.Severity) int {
	switch s {
	case rules.Critical:
		return 3
	case rules.Warn:
		return 2
	default:
		return 1
	}
}

// Text writes the readable report
func Text(w io.Writer, r Result) error {
	s := r.Session
	if _, err := fmt.Fprintf(w, "Studio %s\n", orDash(s.File.Build)); err != nil {
		return err
	}
	if !s.Start.IsZero() {
		fmt.Fprintf(w, "Session   %s  (%s)\n",
			s.Start.Format("2006-01-02 15:04:05"), s.Duration().Round(1e9))
	}
	if len(s.Playtests) > 0 {
		fast, slow := loadRange(s.Playtests)
		fmt.Fprintf(w, "Playtests %d  (fastest %.1fs, slowest %.1fs)\n",
			len(s.Playtests), fast, slow)
	}
	if total, distinct := errorTotals(s); total > 0 {
		fmt.Fprintf(w, "Errors    %d script error(s), %d distinct\n", total, distinct)
	}
	if s.ScriptWarningCount > 0 {
		fmt.Fprintf(w, "Warnings  %d script warning(s)\n", s.ScriptWarningCount)
	}
	fmt.Fprintf(w, "Coverage  %d/%d lines understood (%.1f%%)\n",
		s.Coverage.Parsed, s.Coverage.Total, s.Coverage.Ratio()*100)
	fmt.Fprintln(w)

	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "nothing found")
		return nil
	}
	for _, f := range r.Findings {
		fmt.Fprintf(w, "[%s] %s\n", f.Severity, f.Rule)
		fmt.Fprintf(w, "  %s\n", f.Summary)
		for _, e := range f.Evidence {
			fmt.Fprintf(w, "    - %s\n", e)
		}
		fmt.Fprintln(w)
	}
	return nil
}

// RankedText writes the table of sessions, worst first
func RankedText(w io.Writer, results []Result) error {
	if len(results) == 0 {
		_, err := fmt.Fprintln(w, "nothing to rank")
		return err
	}
	ranked := make([]Result, len(results))
	copy(ranked, results)
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if wa, wb := worst(a.Findings), worst(b.Findings); wa != wb {
			return wa > wb
		}
		return a.Session.File.Start.After(b.Session.File.Start)
	})

	if _, err := fmt.Fprintf(w, "%d session(s), worst first\n\n",
		len(ranked)); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "START\tBUILD\tDURATION\tCOVERAGE\tFINDINGS")
	for _, r := range ranked {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f%%\t%s\n",
			r.Session.File.Start.Format("2006-01-02 15:04:05"),
			orDash(r.Session.File.Build),
			r.Session.Duration().Round(time.Second),
			r.Session.Coverage.Ratio()*100,
			summarise(r.Findings))
	}
	return tw.Flush()
}

func worst(fs []rules.Finding) int {
	high := 0
	for _, f := range fs {
		if r := rank(f.Severity); r > high {
			high = r
		}
	}
	return high
}

func summarise(fs []rules.Finding) string {
	if len(fs) == 0 {
		return "clean"
	}
	counts := map[rules.Severity]int{}
	for _, f := range fs {
		counts[f.Severity]++
	}
	var parts []string
	for _, sev := range []rules.Severity{rules.Critical, rules.Warn, rules.Info} {
		if n := counts[sev]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, sev))
		}
	}
	return strings.Join(parts, ", ")
}

// JSON writes the machine readable report
func JSON(w io.Writer, r Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func errorTotals(s sessionize.Session) (total, distinct int) {
	for _, e := range s.ScriptErrors {
		total += e.Count
	}
	return total, len(s.ScriptErrors)
}

func loadRange(ps []sessionize.Playtest) (fastest, slowest float64) {
	fastest, slowest = ps[0].LoadSeconds, ps[0].LoadSeconds
	for _, p := range ps {
		if p.LoadSeconds < fastest {
			fastest = p.LoadSeconds
		}
		if p.LoadSeconds > slowest {
			slowest = p.LoadSeconds
		}
	}
	return fastest, slowest
}
