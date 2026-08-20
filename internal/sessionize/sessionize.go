package sessionize

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Vliysl/roblox-studio-doctor/internal/parse"
	"github.com/Vliysl/roblox-studio-doctor/internal/scan"
)

// MemSample is one memory reading, with the slot it came from
type MemSample struct {
	Wall  time.Time `json:"wall"`
	Mono  float64   `json:"mono"`
	Bytes int64     `json:"bytes"`
	Slot  int       `json:"slot"`
}

// Disconnect is one Team Create drop and whether the link was lost
type Disconnect struct {
	Wall           time.Time `json:"wall"`
	Reason         string    `json:"reason"`
	Code           int       `json:"code"`
	LostConnection bool      `json:"lost_connection"`
	LineNo         int       `json:"line_no"`
}

// Session is one Studio run, built out of its log events
type Session struct {
	File          scan.SessionFile `json:"file"`
	Coverage      parse.Coverage   `json:"coverage"`
	Start         time.Time        `json:"start"`
	End           time.Time        `json:"end"`
	Memory        []MemSample      `json:"memory,omitempty"`
	Disconnects   []Disconnect     `json:"disconnects,omitempty"`
	CleanExit     bool             `json:"clean_exit"`
	ChannelCounts map[string]int   `json:"channel_counts,omitempty"`
}

func (s Session) Duration() time.Duration { return s.End.Sub(s.Start) }

var disconnectRe = regexp.MustCompile(
	`Disconnected due to (\w+) \((\d+)\)\.\s*LostConnection\s*=\s*(true|false)`)

// Build turns log events into a Session. No filesystem, no clock
func Build(f scan.SessionFile, evs []parse.Event, cov parse.Coverage) Session {
	s := Session{File: f, Coverage: cov, ChannelCounts: map[string]int{}}

	for _, e := range evs {
		s.ChannelCounts[e.Channel]++
		if s.Start.IsZero() || e.Wall.Before(s.Start) {
			s.Start = e.Wall
		}
		if e.Wall.After(s.End) {
			s.End = e.Wall
		}

		switch e.Channel {
		case "TeamCreateManager":
			if m := disconnectRe.FindStringSubmatch(e.Message); m != nil {
				code, _ := strconv.Atoi(m[2])
				s.Disconnects = append(s.Disconnects, Disconnect{
					Wall:           e.Wall,
					Reason:         m[1],
					Code:           code,
					LostConnection: m[3] == "true",
					LineNo:         e.LineNo,
				})
			}
		case "AppMemUsageStatus":
			for slot, b := range memBytes(e.Message) {
				s.Memory = append(s.Memory,
					MemSample{Wall: e.Wall, Mono: e.Mono, Bytes: b, Slot: slot})
			}
		case "StudioApplicationState":
			if strings.Contains(e.Message, "LastWindowClosed") {
				s.CleanExit = true
			}
		}
	}
	return s
}

// This channel sends either one number, or two joined by a dot. The dotted form
// is two separate readings, not a float
func memBytes(msg string) []int64 {
	var out []int64
	for _, field := range strings.Split(strings.TrimSpace(msg), ".") {
		if field == "" {
			continue
		}
		if n, err := strconv.ParseInt(field, 10, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}
