package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fresitamoe/roblox-studio-doctor/internal/parse"
	"github.com/fresitamoe/roblox-studio-doctor/internal/paths"
	"github.com/fresitamoe/roblox-studio-doctor/internal/report"
	"github.com/fresitamoe/roblox-studio-doctor/internal/rules"
	"github.com/fresitamoe/roblox-studio-doctor/internal/scan"
	"github.com/fresitamoe/roblox-studio-doctor/internal/sessionize"
)

const ongoingWindow = 2 * time.Minute

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("studio-doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	logDir := fs.String("log-dir", "", "Studio log directory (default: auto-detect)")
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	allSessions := fs.Bool("all", false, "analyse every session and print a ranked table")
	since := fs.String("since", "", "with -all, only sessions started within this window (e.g. 168h)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *allSessions && *asJSON {
		fmt.Fprintln(stderr, "studio-doctor: -json describes a single session and cannot be combined with -all")
		return 2
	}

	var cutoff time.Time
	if *since != "" {
		if !*allSessions {
			fmt.Fprintln(stderr, "studio-doctor: -since only applies with -all")
			return 2
		}
		d, err := time.ParseDuration(*since)
		if err != nil {
			fmt.Fprintf(stderr, "studio-doctor: bad -since value %q: %v\n", *since, err)
			return 2
		}
		cutoff = time.Now().Add(-d)
	}

	dir := *logDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(stderr, "studio-doctor: cannot determine home directory:", err)
			return 2
		}
		cands := paths.Default(home, os.Getenv)
		for _, c := range cands {
			if st, err := os.Stat(c); err == nil && st.IsDir() {
				dir = c
				break
			}
		}
		if dir == "" {
			fmt.Fprintln(stderr, "studio-doctor: no Studio log directory found. Tried:")
			for _, c := range cands {
				fmt.Fprintln(stderr, "  ", c)
			}
			fmt.Fprintln(stderr, "Pass -log-dir to point at it")
			return 2
		}
	}

	files, err := scan.Find(dir)
	if err != nil {
		fmt.Fprintf(stderr, "studio-doctor: cannot read %s: %v\n", dir, err)
		return 2
	}
	if len(files) == 0 {
		fmt.Fprintf(stderr, "studio-doctor: no Studio logs in %s\n", dir)
		return 3
	}

	if *allSessions {
		if !cutoff.IsZero() {
			files = scan.Since(files, cutoff)
		}
		results := make([]report.Result, 0, len(files))
		for _, f := range files {
			res, err := analyse(f)
			if err != nil {
				fmt.Fprintln(stderr, "studio-doctor:", err)
				continue
			}
			results = append(results, res)
		}
		if err := report.RankedText(stdout, results); err != nil {
			fmt.Fprintln(stderr, "studio-doctor:", err)
			return 2
		}
		return 0
	}

	res, err := analyse(files[0])
	if err != nil {
		fmt.Fprintln(stderr, "studio-doctor:", err)
		return 2
	}
	if *asJSON {
		if err := report.JSON(stdout, res); err != nil {
			fmt.Fprintln(stderr, "studio-doctor:", err)
			return 2
		}
		return 0
	}
	if err := report.Text(stdout, res); err != nil {
		fmt.Fprintln(stderr, "studio-doctor:", err)
		return 2
	}
	return 0
}

func analyse(f scan.SessionFile) (report.Result, error) {
	fh, err := os.Open(f.Path)
	if err != nil {
		return report.Result{}, fmt.Errorf("cannot open %s: %w", f.Path, err)
	}
	defer fh.Close()

	evs, cov, err := parse.Read(fh)
	if err != nil {
		return report.Result{}, fmt.Errorf("reading %s: %w", f.Path, err)
	}
	sess := sessionize.Build(f, evs, cov)
	if st, err := fh.Stat(); err == nil && time.Since(st.ModTime()) < ongoingWindow {
		sess.Ongoing = true
	}
	return report.NewResult(sess, rules.Apply(sess)), nil
}
