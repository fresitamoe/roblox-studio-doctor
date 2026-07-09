package scan

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// SessionFile is one session's log, identified from its filename alone
type SessionFile struct {
	Path  string    `json:"path"`
	Build string    `json:"build"`
	Hash  string    `json:"hash"`
	Start time.Time `json:"start"`
}

var nameRe = regexp.MustCompile(
	`^(\d+\.\d+\.\d+\.\d+)_(\d{8}T\d{6}Z)_Studio_([0-9A-Fa-f]+)_last\.log$`)

const nameTimeLayout = "20060102T150405Z"

// Player logs sit in the same folder and look similar, so the _Studio_ part matters
func ParseName(name string) (SessionFile, bool) {
	m := nameRe.FindStringSubmatch(name)
	if m == nil {
		return SessionFile{}, false
	}
	ts, err := time.Parse(nameTimeLayout, m[2])
	if err != nil {
		return SessionFile{}, false
	}
	return SessionFile{Build: m[1], Start: ts.UTC(), Hash: m[3]}, true
}

// Find returns the Studio logs in dir, newest first
func Find(dir string) ([]SessionFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []SessionFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		sf, ok := ParseName(e.Name())
		if !ok {
			continue
		}
		sf.Path = filepath.Join(dir, e.Name())
		out = append(out, sf)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.After(out[j].Start) })
	return out, nil
}

// Since keeps the sessions that started at or after cutoff
func Since(files []SessionFile, cutoff time.Time) []SessionFile {
	var out []SessionFile
	for _, f := range files {
		if !f.Start.Before(cutoff) {
			out = append(out, f)
		}
	}
	return out
}
