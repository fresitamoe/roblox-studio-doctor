package sessionize

import (
	"regexp"
	"sort"
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

// ScriptError is one distinct error and how often it came up
type ScriptError struct {
	Path      string    `json:"path,omitempty"`
	Line      int       `json:"line,omitempty"`
	Message   string    `json:"message"`
	Count     int       `json:"count"`
	FirstWall time.Time `json:"first_wall"`
	LastWall  time.Time `json:"last_wall"`
}

// AssetAccessFailure is an asset this session could not reach
type AssetAccessFailure struct {
	AssetID   string `json:"asset_id"`
	AssetType string `json:"asset_type"`
	Count     int    `json:"count"`
}

// Playtest is one playtest and how long it took to load
type Playtest struct {
	Wall        time.Time `json:"wall"`
	LoadSeconds float64   `json:"load_seconds"`
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

	ScriptErrors        []ScriptError        `json:"script_errors,omitempty"`
	ScriptWarningCount  int                  `json:"script_warning_count"`
	AssetAccessFailures []AssetAccessFailure `json:"asset_access_failures,omitempty"`
	Playtests           []Playtest           `json:"playtests,omitempty"`

	Ongoing bool `json:"ongoing"`
}

func (s Session) Duration() time.Duration { return s.End.Sub(s.Start) }

var disconnectRe = regexp.MustCompile(
	`Disconnected due to (\w+) \((\d+)\)\.\s*LostConnection\s*=\s*(true|false)`)

var scriptErrorRe = regexp.MustCompile(`^(.+?):(\d+): (.+)$`)

var assetFailRe = regexp.MustCompile(
	`Received assetFetchFailed\w* signal for asset ID (\d+), expected type (\w+)`)

var playtestRe = regexp.MustCompile(`^PlaySoloStartTotalTime\s*:\s*([0-9.]+)\s*sec`)

type scriptErrorKey struct {
	path    string
	line    int
	message string
}

type assetFailureKey struct {
	id      string
	assType string
}

// Build turns log events into a Session. No filesystem, no clock
func Build(f scan.SessionFile, evs []parse.Event, cov parse.Coverage) Session {
	s := Session{File: f, Coverage: cov, ChannelCounts: map[string]int{}}

	errIndex := map[scriptErrorKey]*ScriptError{}
	var errOrder []scriptErrorKey
	assetIndex := map[assetFailureKey]*AssetAccessFailure{}
	var assetOrder []assetFailureKey

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
		case "CreatorError":
			k := scriptErrorKeyOf(e.Message)
			cur, seen := errIndex[k]
			if !seen {
				cur = &ScriptError{
					Path: k.path, Line: k.line, Message: k.message,
					FirstWall: e.Wall,
				}
				errIndex[k] = cur
				errOrder = append(errOrder, k)
			}
			cur.Count++
			cur.LastWall = e.Wall
		case "CreatorWarning":
			s.ScriptWarningCount++
		case "AssetAccessDataModelObserver":
			if m := assetFailRe.FindStringSubmatch(e.Message); m != nil {
				k := assetFailureKey{id: m[1], assType: m[2]}
				cur, seen := assetIndex[k]
				if !seen {
					cur = &AssetAccessFailure{AssetID: k.id, AssetType: k.assType}
					assetIndex[k] = cur
					assetOrder = append(assetOrder, k)
				}
				cur.Count++
			}
		case "StudioTimingLog":
			if m := playtestRe.FindStringSubmatch(e.Message); m != nil {
				if secs, err := strconv.ParseFloat(m[1], 64); err == nil {
					s.Playtests = append(s.Playtests,
						Playtest{Wall: e.Wall, LoadSeconds: secs})
				}
			}
		}
	}

	for _, k := range errOrder {
		s.ScriptErrors = append(s.ScriptErrors, *errIndex[k])
	}
	sort.SliceStable(s.ScriptErrors, func(i, j int) bool {
		a, b := s.ScriptErrors[i], s.ScriptErrors[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Path < b.Path
	})

	for _, k := range assetOrder {
		s.AssetAccessFailures = append(s.AssetAccessFailures, *assetIndex[k])
	}
	sort.SliceStable(s.AssetAccessFailures, func(i, j int) bool {
		a, b := s.AssetAccessFailures[i], s.AssetAccessFailures[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.AssetID < b.AssetID
	})

	return s
}

func scriptErrorKeyOf(msg string) scriptErrorKey {
	text := strings.TrimPrefix(strings.TrimSpace(msg), "Error: ")
	m := scriptErrorRe.FindStringSubmatch(text)
	if m == nil {
		return scriptErrorKey{message: text}
	}
	line, err := strconv.Atoi(m[2])
	if err != nil {
		return scriptErrorKey{message: text}
	}
	return scriptErrorKey{path: m[1], line: line, message: m[3]}
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
