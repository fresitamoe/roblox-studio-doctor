package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Vliysl/roblox-studio-doctor/internal/rules"
	"github.com/Vliysl/roblox-studio-doctor/internal/sessionize"
)

func TestTextSaysNothingFound(t *testing.T) {
	var buf bytes.Buffer
	if err := Text(&buf, NewResult(sessionize.Session{CleanExit: true}, nil)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "nothing found") {
		t.Errorf("missing explicit all-clear:\n%s", buf.String())
	}
}

func TestTextRanksCriticalFirst(t *testing.T) {
	fs := []rules.Finding{
		{Rule: "b", Severity: rules.Warn, Summary: "warn thing", Evidence: []string{"e"}},
		{Rule: "a", Severity: rules.Critical, Summary: "critical thing", Evidence: []string{"e"}},
	}
	var buf bytes.Buffer
	if err := Text(&buf, NewResult(sessionize.Session{}, fs)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Index(out, "critical thing") > strings.Index(out, "warn thing") {
		t.Errorf("critical must sort first:\n%s", out)
	}
}

func TestTextAlwaysShowsCoverage(t *testing.T) {
	var buf bytes.Buffer
	s := sessionize.Session{}
	s.Coverage.Total = 100
	s.Coverage.Parsed = 94
	if err := Text(&buf, NewResult(s, nil)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "94") {
		t.Errorf("coverage missing from output:\n%s", buf.String())
	}
}

func TestJSONCarriesSchemaField(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, NewResult(sessionize.Session{}, nil)); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["schema"] == nil || m["schema"] == "" {
		t.Errorf("schema field missing: %v", m)
	}
}

func TestTextSessionHeader(t *testing.T) {
	s := sessionize.Session{
		CleanExit: true,
		Playtests: []sessionize.Playtest{
			{LoadSeconds: 4.5}, {LoadSeconds: 1.9}, {LoadSeconds: 12.2},
		},
		ScriptErrors: []sessionize.ScriptError{
			{Path: "ReplicatedStorage.A", Line: 1, Message: "boom", Count: 62_000},
			{Path: "ServerScriptService.B", Line: 2, Message: "bang", Count: 684},
		},
		ScriptWarningCount: 17,
	}
	var buf bytes.Buffer
	if err := Text(&buf, NewResult(s, nil)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Playtests 3", "fastest 1.9s", "slowest 12.2s",
		"62684 script error(s), 2 distinct",
		"17 script warning(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("header missing %q:\n%s", want, out)
		}
	}
}

func TestTextOmitsEmptyHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := Text(&buf, NewResult(sessionize.Session{CleanExit: true}, nil)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, unwanted := range []string{"Playtests", "Errors", "Warnings"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("printed %q for a session with none:\n%s", unwanted, out)
		}
	}
}

func rankedFixture() []Result {
	at := func(day int) sessionize.Session {
		var s sessionize.Session
		s.File.Build = "0.737.0.7371584"
		s.File.Start = time.Date(2026, 8, day, 10, 0, 0, 0, time.UTC)
		s.Start = s.File.Start
		s.End = s.File.Start.Add(time.Hour)
		s.Coverage.Total, s.Coverage.Parsed = 100, 99
		return s
	}
	warn := []rules.Finding{{Rule: "w", Severity: rules.Warn, Summary: "w", Evidence: []string{"e"}}}
	crit := []rules.Finding{
		{Rule: "c", Severity: rules.Critical, Summary: "c", Evidence: []string{"e"}},
		{Rule: "w", Severity: rules.Warn, Summary: "w", Evidence: []string{"e"}},
	}
	return []Result{
		NewResult(at(4), nil),
		NewResult(at(2), crit),
		NewResult(at(3), warn),
	}
}

func TestRankedWorstFirst(t *testing.T) {
	var buf bytes.Buffer
	if err := RankedText(&buf, rankedFixture()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	crit, warn, clean := strings.Index(out, "1 critical"), strings.Index(out, "1 warn"), strings.Index(out, "clean")
	if crit < 0 || warn < 0 || clean < 0 {
		t.Fatalf("missing rows:\n%s", out)
	}
	if !(crit < warn && warn < clean) {
		t.Errorf("wrong order:\n%s", out)
	}
	if !strings.Contains(out, "1 critical, 2 warn") &&
		!strings.Contains(out, "1 critical, 1 warn") {
		t.Errorf("bad findings column:\n%s", out)
	}
	if n := strings.Count(out, "0.737.0.7371584"); n != 3 {
		t.Errorf("got %d rows, want one per session:\n%s", n, out)
	}
	if !strings.Contains(out, "3 session(s), worst first") {
		t.Errorf("missing the count header:\n%s", out)
	}
}

func TestRankedDoesNotMutate(t *testing.T) {
	in := rankedFixture()
	first := in[0].Session.File.Start
	var buf bytes.Buffer
	if err := RankedText(&buf, in); err != nil {
		t.Fatal(err)
	}
	if !in[0].Session.File.Start.Equal(first) {
		t.Error("RankedText mutated the input")
	}
}

func TestRankedEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := RankedText(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("empty output")
	}
}
