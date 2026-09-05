package redact

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/Vliysl/roblox-studio-doctor/internal/parse"
)

// Policy is how an allowlisted channel's payload gets treated
type Policy int

const (
	PolicyText Policy = iota
	PolicyNumeric
)

// Allowlist, it is strictly not a denylist. The place names are just whatever it was typed, so no
// pattern catches them all. Anything not listed here gets dropped
var AllowedChannels = map[string]Policy{
	"TeamCreateManager":      PolicyText,
	"StudioApplicationState": PolicyText,
	"AppMemUsageStatus":      PolicyNumeric,
}

var idRe = regexp.MustCompile(`\d{8,}`)

var pathRe = regexp.MustCompile(`(?i)([A-Z]:\\users\\|/home/|/Users/)([^\\/\s]+)`)

var numericRe = regexp.MustCompile(`^\d+(\.\d+)*$`)

// Events keeps the allowlisted channels and redacts what is left
func Events(evs []parse.Event) []parse.Event {
	var out []parse.Event
	for _, e := range evs {
		pol, ok := AllowedChannels[e.Channel]
		if !ok {
			continue
		}
		if pol == PolicyNumeric && numericRe.MatchString(strings.TrimSpace(e.Message)) {

		} else {
			e.Message = Text(e.Message)
		}
		e.Severity = Text(e.Severity)
		e.Raw = ""
		out = append(out, e)
	}
	return out
}

// Text swaps identifiers in a string for stand-ins. Same within one run,
// different between runs, since the salt is new each time
func Text(s string) string {
	s = pathRe.ReplaceAllString(s, "${1}user_redacted")
	return idRe.ReplaceAllStringFunc(s, func(m string) string {
		return "id_" + token(m)
	})
}

var salt = func() []byte {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("redact: cannot initialise salt: " + err.Error())
	}
	return b
}()

func token(s string) string {
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))[:6]
}
