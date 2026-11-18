// Command owlsort tidies the loose files in your Downloads and Desktop:
// preview first, undo anytime.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/08820048/owlsort-cli/internal/cli"
	"github.com/08820048/owlsort-cli/internal/exitcode"
)

// Set by GoReleaser: -ldflags "-X main.version=... -X main.commit=... -X main.date=..."
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root := cli.New(cli.BuildInfo{Version: version, Commit: commit, Date: date})
	if err := root.Execute(); err != nil {
		var coded *cli.ExitError
		if errors.As(err, &coded) {
			if coded.Message != "" {
				fmt.Fprintln(os.Stderr, "owlsort:", coded.Message)
			}
			os.Exit(coded.Code)
		}
		fmt.Fprintln(os.Stderr, "owlsort:", err)
		os.Exit(exitcode.Error)
	}
}
