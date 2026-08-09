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
