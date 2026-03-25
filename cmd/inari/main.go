// Inari -- Code intelligence CLI for LLM coding agents.
//
// Builds a local code intelligence index and lets you query it efficiently.
// Use it before editing any non-trivial code to understand structure,
// dependencies, and blast radius.
package main

import (
	"fmt"
	"os"

	"github.com/KilimcininKorOglu/inari/internal/commands"
)

// version is set at build time via -ldflags.
var version = "1.3.5"

func main() {
	commands.Version = version
	cmd := commands.NewRootCommand()
	cmd.Version = version
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
