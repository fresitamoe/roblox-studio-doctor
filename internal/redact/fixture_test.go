package redact

import (
	"strings"
	"testing"
)

func TestFixtureRoundTrip(t *testing.T) {
	in := strings.Join([]string{
		`2026-07-13T23:31:35.625Z,665.625732,0128,6,Warning [FLog::TeamCreateManager] Disconnected due to DisconnectClientInitiated (285). LostConnection = false`,
		`2026-07-13T23:31:36.000Z,666.000000,0128,6,Info [FLog::Output] print("my place name")`,
		`2026-07-13T23:31:37.000Z,667.000000,0128,6,Info [FLog::AppMemUsageStatus] 3748799858.1044687361`,
	}, "\n") + "\n"

	got, err := Fixture(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "my place name") {
		t.Fatalf("Output channel leaked: %q", got)
	}
	if !strings.Contains(got, "TeamCreateManager") {
		t.Errorf("allowlisted channel missing: %q", got)
	}
	if !strings.Contains(got, "LostConnection = false") {
		t.Errorf("payload damaged: %q", got)
	}
	if strings.Count(got, "\n") != 2 {
		t.Errorf("expected 2 lines, got %q", got)
	}
}

func TestFixtureIsIdempotent(t *testing.T) {
	in := `2026-07-13T23:31:35.625Z,665.625732,0128,6,Warning [FLog::TeamCreateManager] place 10553200977 dropped` + "\n"
	once, err := Fixture(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Fixture(strings.NewReader(once))
	if err != nil {
		t.Fatal(err)
	}
	if once != twice {
		t.Errorf("not idempotent:\n once: %q\ntwice: %q", once, twice)
	}
}
