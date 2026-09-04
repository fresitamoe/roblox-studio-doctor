package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
