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

const readBuf = 64 * 1024

// Read parses log lines from r. Bad content is skipped, not an error
func Read(r io.Reader) ([]Event, Coverage, error) {
	cov := Coverage{}
	var out []Event

	br := bufio.NewReaderSize(r, readBuf)
	for n := 1; ; n++ {
		line, overlong, err := readLine(br)

		switch {
		case overlong:
			cov.Total++
			cov.Skipped++
		case strings.TrimSpace(line) != "":
			cov.Total++
			if ev, ok := parseLine(line); ok {
				ev.LineNo = n
				cov.Parsed++
				out = append(out, ev)
			} else {
				cov.Skipped++
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, cov, nil
			}
			return out, cov, err
		}
	}
}

func readLine(br *bufio.Reader) (string, bool, error) {
	var b strings.Builder
	overlong := false
	for {
		chunk, err := br.ReadSlice('\n')
		if n := len(chunk); n > 0 {
			if room := maxLine - b.Len(); room > 0 {
				if n > room {
					chunk, overlong = chunk[:room], true
				}
				b.Write(chunk)
			} else {
				overlong = true
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return strings.TrimRight(b.String(), "\r\n"), overlong, err
	}
}

// Studio writes two line shapes, one with a severity word and one without
// Majority of real lines most likely the second kind
func parseLine(line string) (Event, bool) {
	open := strings.IndexByte(line, '[')
	if open < 0 {
		return Event{}, false
	}
	rel := strings.IndexByte(line[open:], ']')
	if rel < 0 {
		return Event{}, false
	}
	closeIdx := open + rel

	fields := strings.Split(line[:open], ",")
	if len(fields) < 4 {
		return Event{}, false
	}
	wall, err := time.Parse(wallLayout, fields[0])
	if err != nil {
		return Event{}, false
	}
	mono, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return Event{}, false
	}
	severity := ""
	if len(fields) >= 5 {
		severity = strings.TrimSpace(fields[4])
	}

	return Event{
		Wall:     wall.UTC(),
		Mono:     mono,
		Thread:   fields[2],
		Severity: severity,
		Channel:  strings.TrimPrefix(line[open+1:closeIdx], "FLog::"),
		Message:  strings.TrimSpace(line[closeIdx+1:]),
		Raw:      line,
	}, true
}
