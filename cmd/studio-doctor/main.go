package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Vliysl/roblox-studio-doctor/internal/parse"
	"github.com/Vliysl/roblox-studio-doctor/internal/paths"
	"github.com/Vliysl/roblox-studio-doctor/internal/report"
	"github.com/Vliysl/roblox-studio-doctor/internal/rules"
	"github.com/Vliysl/roblox-studio-doctor/internal/scan"
	"github.com/Vliysl/roblox-studio-doctor/internal/sessionize"
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
	if err := fs.Parse(args); err != nil {
		return 2
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

	f := files[0]
	fh, err := os.Open(f.Path)
	if err != nil {
		fmt.Fprintf(stderr, "studio-doctor: cannot open %s: %v\n", f.Path, err)
		return 2
	}
	defer fh.Close()

	evs, cov, err := parse.Read(fh)
	if err != nil {
		fmt.Fprintf(stderr, "studio-doctor: reading %s: %v\n", f.Path, err)
		return 2
	}

	sess := sessionize.Build(f, evs, cov)
	if st, err := fh.Stat(); err == nil && time.Since(st.ModTime()) < ongoingWindow {
		sess.Ongoing = true
	}
	res := report.NewResult(sess, rules.Apply(sess))
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
