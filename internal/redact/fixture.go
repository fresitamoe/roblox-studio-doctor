package redact

import (
	"fmt"
	"io"
	"strings"

	"github.com/Vliysl/roblox-studio-doctor/internal/parse"
)

// Fixture filters a Studio log through the allowlist for committed test data
func Fixture(r io.Reader) (string, error) {
	evs, _, err := parse.Read(r)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range Events(evs) {
		ts := e.Wall.Format("2006-01-02T15:04:05.000Z")
		if e.Severity == "" {
			fmt.Fprintf(&b, "%s,%f,%s,6 [FLog::%s] %s\n",
				ts, e.Mono, e.Thread, e.Channel, e.Message)
			continue
		}
		fmt.Fprintf(&b, "%s,%f,%s,6,%s [FLog::%s] %s\n",
			ts, e.Mono, e.Thread, e.Severity, e.Channel, e.Message)
	}
	return b.String(), nil
}
