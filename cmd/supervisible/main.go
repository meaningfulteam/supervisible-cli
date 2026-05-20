package main

import (
	"fmt"
	"os"

	"github.com/supervisible/supervisible-cli/internal/cmd"
	"github.com/supervisible/supervisible-cli/internal/output"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, output.FormatCLIError(err))
		os.Exit(1)
	}
}
