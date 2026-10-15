// Command evtccombine merges the recordings several players made of one
// fight into one .zevtc log.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/42atomys/evtc/combine"
)

func main() {
	out := flag.String("o", "combined.zevtc", "the file to write, raw when it ends in .evtc")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [-o combined.zevtc] <file.zevtc> <file.zevtc>...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(*out, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "evtccombine:", err)
		os.Exit(1)
	}
}

func run(out string, paths []string) error {
	// The recordings are read before anything is written: one of them
	// given as the output would be lost.
	for _, p := range paths {
		if same(p, out) {
			return fmt.Errorf("%s is one of the recordings", out)
		}
	}
	start := time.Now()
	l, err := combine.Files(paths...)
	if err != nil {
		return err
	}
	if err := l.WriteFile(out); err != nil {
		return err
	}

	fmt.Printf("recordings:  %d\n", len(paths))
	fmt.Printf("build:       %s\n", l.Header.Build)
	fmt.Printf("target:      %d\n", l.Header.TargetSpeciesID)
	fmt.Printf("agents:      %d\n", len(l.Agents))
	fmt.Printf("skills:      %d\n", len(l.Skills))
	fmt.Printf("events:      %d\n", len(l.Events))
	fmt.Printf("combined in: %s\n", time.Since(start).Round(time.Millisecond))
	fmt.Printf("written to:  %s\n", out)
	return nil
}

// same reports whether two paths name the same file.
func same(a, b string) bool {
	x, errA := filepath.Abs(a)
	y, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	if ia, err := os.Stat(x); err == nil {
		if ib, err := os.Stat(y); err == nil {
			return os.SameFile(ia, ib)
		}
	}
	return x == y
}
