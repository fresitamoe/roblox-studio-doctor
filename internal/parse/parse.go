package parse

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

// Event is one parsed log line
type Event struct {
	Wall     time.Time `json:"wall"`
	Mono     float64   `json:"mono"`
	Thread   string    `json:"thread"`
	Severity string    `json:"severity"`
	Channel  string    `json:"channel"`
	Message  string    `json:"message"`
	LineNo   int       `json:"line_no"`
	Raw      string    `json:"-"`
}

// Coverage is how much of a log was understood
type Coverage struct {
	Total   int `json:"total_lines"`
	Parsed  int `json:"parsed_lines"`
	Skipped int `json:"skipped_lines"`
}

// Ratio is the fraction of lines parsed. Empty input counts as 1
func (c Coverage) Ratio() float64 {
	if c.Total == 0 {
		return 1.0
	}
	return float64(c.Parsed) / float64(c.Total)
}

const wallLayout = "2006-01-02T15:04:05.999Z"

const maxLine = 1 << 20

// Read parses log lines from r. Bad content is skipped, not an error
func Read(r io.Reader) ([]Event, Coverage, error) {
	cov := Coverage{}
	var out []Event

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)

	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		cov.Total++
		ev, ok := parseLine(line)
		if !ok {
			cov.Skipped++
			continue
		}
		ev.LineNo = n
		cov.Parsed++
		out = append(out, ev)
	}
	if err := sc.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			cov.Total++
			cov.Skipped++
			return out, cov, nil
		}
		return out, cov, err
	}
	return out, cov, nil
}

// Studio writes two line shapes, one with a severity word and one without
// Majority of real lines most likely the second kind
func parseLine(line string) (Event, bool) {

	parts := strings.SplitN(line, ",", 5)
	if len(parts) < 5 {
		return Event{}, false
	}
	wall, err := time.Parse(wallLayout, parts[0])
	if err != nil {
		return Event{}, false
	}
	mono, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return Event{}, false
	}

	rest := parts[4]
	open := strings.IndexByte(rest, '[')
	closeIdx := strings.IndexByte(rest, ']')
	if open < 0 || closeIdx < open {
		return Event{}, false
	}

	channel := rest[open+1 : closeIdx]
	channel = strings.TrimPrefix(channel, "FLog::")

	return Event{
		Wall:     wall.UTC(),
		Mono:     mono,
		Thread:   parts[2],
		Severity: strings.TrimSpace(rest[:open]),
		Channel:  channel,
		Message:  strings.TrimSpace(rest[closeIdx+1:]),
		Raw:      line,
	}, true
}
