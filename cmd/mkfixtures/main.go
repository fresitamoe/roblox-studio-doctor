package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Vliysl/roblox-studio-doctor/internal/redact"
	"github.com/Vliysl/roblox-studio-doctor/internal/scan"
)

func main() {
	src := flag.String("src", "", "directory of real Studio logs")
	out := flag.String("out", "testdata/fixtures", "output directory")
	limit := flag.Int("n", 5, "how many recent sessions to convert")
	flag.Parse()

	if *src == "" {
		fmt.Fprintln(os.Stderr, "mkfixtures: -src is required")
		os.Exit(2)
	}
	files, err := scan.Find(*src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkfixtures:", err)
		os.Exit(2)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkfixtures:", err)
		os.Exit(2)
	}
	for i, f := range files {
		if i >= *limit {
			break
		}
		in, err := os.Open(f.Path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mkfixtures:", err)
			continue
		}
		text, err := redact.Fixture(in)
		in.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "mkfixtures:", err)
			continue
		}
		name := fmt.Sprintf("%s_%s_Studio_F1%03d_last.log",
			f.Build, f.Start.UTC().Format("20060102T150405Z"), i)
		dst := filepath.Join(*out, name)
		if err := os.WriteFile(dst, []byte(text), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "mkfixtures:", err)
			continue
		}
		fmt.Printf("wrote %s (%d bytes)\n", dst, len(text))
	}
}
