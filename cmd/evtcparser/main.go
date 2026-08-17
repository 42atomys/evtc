// Command evtcparser decodes an arcdps .evtc or .zevtc log and prints a
// short summary of its content.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/42atomys/evtc"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s <file.evtc|file.zevtc>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, "evtcparser:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	start := time.Now()
	l, err := evtc.ParseFile(path)
	if err != nil {
		return err
	}

	fmt.Printf("build:     %s\n", l.Header.Build)
	fmt.Printf("revision:  %d\n", l.Header.Revision)
	fmt.Printf("target:    %d\n", l.Header.TargetSpeciesID)
	fmt.Printf("agents:    %d\n", len(l.Agents))
	fmt.Printf("skills:    %d\n", len(l.Skills))
	fmt.Printf("events:    %d\n", len(l.Events))
	fmt.Printf("parsed in: %s\n", time.Since(start).Round(time.Millisecond))
	return nil
}
