package cli

import (
	"os"

	"golang.org/x/term"
)

// isInteractive reports whether stdin is connected to a terminal.
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// isTerminalOutput reports whether stdout is connected to a terminal.
func isTerminalOutput() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// terminalWidth reports stdout's column count.
func terminalWidth() (int, error) {
	cols, _, err := term.GetSize(int(os.Stdout.Fd()))
	return cols, err
}
