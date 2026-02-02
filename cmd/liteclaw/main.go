// Package main provides the entry point for LiteClaw CLI.
package main

import (
	"os"

	"github.com/liteclaw/liteclaw/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
