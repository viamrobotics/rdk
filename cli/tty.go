package cli

import (
	"os"

	"golang.org/x/term"
)

// isInteractive reports whether stdin is connected to a terminal.
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// isOutputTTY reports whether stdout is connected to a terminal.
// It is a var so tests can exercise the terminal-attached rendering path.
var isOutputTTY = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
