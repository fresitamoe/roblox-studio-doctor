package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Vliysl/roblox-studio-doctor/internal/parse"
	"github.com/Vliysl/roblox-studio-doctor/internal/rules"
	"github.com/Vliysl/roblox-studio-doctor/internal/scan"
	"github.com/Vliysl/roblox-studio-doctor/internal/sessionize"
)

const fixtureDir = "../../testdata/fixtures"

const coverageFloor = 0.95

type analysed struct {
	session  sessionize.Session
	findings []rules.Finding
}

func analyseAll(t *testing.T) []analysed {
	t.Helper()
	files, err := scan.Find(fixtureDir)
	if err != nil {
		t.Fatalf("scan.Find(%s): %v", fixtureDir, err)
	}
	out := make([]analysed, 0, len(files))
	for _, f := range files {
		fh, err := os.Open(f.Path)
		if err != nil {
			t.Fatalf("open %s: %v", f.Path, err)
		}
		evs, cov, err := parse.Read(fh)
		fh.Close()
		if err != nil {
			t.Fatalf("parse %s: %v", f.Path, err)
		}
		s := sessionize.Build(f, evs, cov)
		out = append(out, analysed{session: s, findings: rules.Apply(s)})
	}
	return out
}

func TestScanFindsFixtures(t *testing.T) {
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			want = append(want, e.Name())
		}
	}
	if len(want) == 0 {
		t.Fatalf("no fixtures in %s", fixtureDir)
	}

	files, err := scan.Find(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, f := range files {
		found[filepath.Base(f.Path)] = true
	}
	for _, name := range want {
		if !found[name] {
			t.Errorf("Find missed %q", name)
		}
	}
	if len(files) != len(want) {
		t.Errorf("discovered %d files, want %d", len(files), len(want))
	}
}

func TestCoverageFloor(t *testing.T) {
	var parsed, total int
	for _, a := range analyseAll(t) {
		c := a.session.Coverage
		parsed += c.Parsed
		total += c.Total
		t.Logf("%s: %d/%d lines (%.1f%%)",
			filepath.Base(a.session.File.Path), c.Parsed, c.Total, c.Ratio()*100)
	}
	if total == 0 {
		t.Fatal("fixtures produced no lines at all")
	}
	got := float64(parsed) / float64(total)
	if got < coverageFloor {
		t.Errorf("coverage %.4f, floor %.2f", got, coverageFloor)
	}
}

func TestEveryFixtureYieldsEvents(t *testing.T) {
	for _, a := range analyseAll(t) {
		name := filepath.Base(a.session.File.Path)
		if a.session.Coverage.Parsed == 0 {
			t.Errorf("%s: parsed no events", name)
		}
		if a.session.Start.IsZero() || a.session.End.Before(a.session.Start) {
			t.Errorf("%s: nonsensical span %v..%v", name, a.session.Start, a.session.End)
		}
	}
}

func TestFindingsHaveEvidence(t *testing.T) {
	for _, a := range analyseAll(t) {
		name := filepath.Base(a.session.File.Path)
		for _, f := range a.findings {
			if f.Rule == "" {
				t.Errorf("%s: finding with no rule id: %+v", name, f)
			}
			if f.Summary == "" {
				t.Errorf("%s: rule %q has no summary", name, f.Rule)
			}
			if len(f.Evidence) == 0 {
				t.Errorf("%s: rule %q cites no evidence", name, f.Rule)
			}
			for i, e := range f.Evidence {
				if e == "" {
					t.Errorf("%s: rule %q evidence[%d] is empty", name, f.Rule, i)
				}
			}
		}
	}
}

func TestOnlyActiveRules(t *testing.T) {
	active := map[string]bool{
		"teamcreate-lost-connection": true,
		"crash-no-clean-exit":        true,
	}
	for _, a := range analyseAll(t) {
		name := filepath.Base(a.session.File.Path)
		for _, f := range a.findings {
			if !active[f.Rule] {
				t.Errorf("%s: unexpected rule %q in findings", name, f.Rule)
			}
		}
	}
}
