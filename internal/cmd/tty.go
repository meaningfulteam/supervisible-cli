package cmd

import (
	"os"

	"golang.org/x/term"
)

// isStdinInteractive reports whether os.Stdin is connected to a TTY.
func isStdinInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// isStdoutInteractive reports whether os.Stdout is connected to a TTY.
func isStdoutInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
