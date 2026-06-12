package cli

import (
	"os"

	"golang.org/x/term"
)

// isInteractive reports whether stdin is connected to a terminal.
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
