package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fixture = `2026-08-05T10:00:00.000Z,1.000000,0128,6,Warning [FLog::TeamCreateManager] Disconnected due to TimeAsleepDisconnectThreshold (17). LostConnection = true
2026-08-05T10:00:01.000Z,2.000000,0128,6,Info [FLog::AppMemUsageStatus] 3000000000
`

func writeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := "0.737.0.7371584_20260805T100000Z_Studio_ABCDE_last.log"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRunReportsLostConnection(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"-log-dir", writeFixture(t)}, &out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "teamcreate-lost-connection") {
		t.Errorf("finding missing:\n%s", out.String())
	}
}

func TestRunExits3WhenNoLogsFound(t *testing.T) {
	var errBuf bytes.Buffer
	code := run([]string{"-log-dir", t.TempDir()}, &bytes.Buffer{}, &errBuf)
	if code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
}

func TestRunExits2OnMissingDir(t *testing.T) {
	var errBuf bytes.Buffer
	code := run([]string{"-log-dir", "/definitely/not/here"}, &bytes.Buffer{}, &errBuf)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunJSONMode(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"-log-dir", writeFixture(t), "-json"}, &out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out.String(), `"schema"`) {
		t.Errorf("not json:\n%s", out.String())
	}
}

func TestRunOngoing(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"-log-dir", writeFixture(t)}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "[info] crash-no-clean-exit") {
		t.Errorf("want info:\n%s", got)
	}
	if strings.Contains(got, "crashed") {
		t.Errorf("should not say crash:\n%s", got)
	}
}

func TestRunOldLogIsCrash(t *testing.T) {
	dir := writeFixture(t)
	old := time.Now().Add(-time.Hour)
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if err := os.Chtimes(filepath.Join(dir, e.Name()), old, old); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if code := run([]string{"-log-dir", dir}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out.String(), "[warn] crash-no-clean-exit") {
		t.Errorf("want crash:\n%s", out.String())
	}
}
