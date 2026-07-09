package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseName(t *testing.T) {
	sf, ok := ParseName("0.737.0.7371584_20260705T170510Z_Studio_9E502_last.log")
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if sf.Build != "0.737.0.7371584" {
		t.Errorf("build = %q", sf.Build)
	}
	if sf.Hash != "9E502" {
		t.Errorf("hash = %q", sf.Hash)
	}
	want := time.Date(2026, 7, 5, 17, 5, 10, 0, time.UTC)
	if !sf.Start.Equal(want) {
		t.Errorf("start = %v, want %v", sf.Start, want)
	}
}

func TestParseNameRejectsNonStudio(t *testing.T) {

	for _, name := range []string{
		"0.737.0.7371584_20260705T170510Z_Player_9E502_last.log",
		"notalog.txt",
		"0.737_bad_Studio_X_last.log",
	} {
		if _, ok := ParseName(name); ok {
			t.Errorf("ParseName(%q) succeeded, want reject", name)
		}
	}
}

func TestFindSortsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"0.737.0.7371584_20260702T100000Z_Studio_AAAAA_last.log",
		"0.737.0.7371584_20260705T170510Z_Studio_BBBBB_last.log",
		"0.736.0.7360000_20260630T090000Z_Studio_CCCCC_last.log",
		"ignore-me.txt",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Find(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d files, want 3", len(got))
	}
	if got[0].Hash != "BBBBB" {
		t.Errorf("newest = %q, want BBBBB", got[0].Hash)
	}
	if got[2].Hash != "CCCCC" {
		t.Errorf("oldest = %q, want CCCCC", got[2].Hash)
	}
}

func TestSince(t *testing.T) {
	files := []SessionFile{
		{Hash: "new", Start: time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)},
		{Hash: "old", Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	got := Since(files, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	if len(got) != 1 || got[0].Hash != "new" {
		t.Fatalf("got %v, want only 'new'", got)
	}
}
