package redact

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"

	"github.com/Vliysl/roblox-studio-doctor/internal/parse"
)

// Allowlist, it is strictly not a denylist. The place names are just whatever it was typed, so no
// pattern catches them all. Anything not listed here gets dropped
var AllowedChannels = map[string]bool{
	"TeamCreateManager":      true,
	"StudioApplicationState": true,
	"AppMemUsageStatus":      true,
}

var idRe = regexp.MustCompile(`\d{8,}`)

var pathRe = regexp.MustCompile(`(?i)([A-Z]:\\users\\|/home/|/Users/)([^\\/\s]+)`)

// Events keeps the allowlisted channels and redacts what is left
func Events(evs []parse.Event) []parse.Event {
	var out []parse.Event
	for _, e := range evs {
		if !AllowedChannels[e.Channel] {
			continue
		}
		e.Message = Text(e.Message)
		e.Raw = ""
		out = append(out, e)
	}
	return out
}

// Text swaps identifiers in a string for stable replacements
func Text(s string) string {
	s = pathRe.ReplaceAllString(s, "${1}user_redacted")
	return idRe.ReplaceAllStringFunc(s, func(m string) string {
		return "id_" + token(m)
	})
}

func token(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:6]
}
