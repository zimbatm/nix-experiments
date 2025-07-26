package main

import (
	"fmt"
	"os"

	"github.com/zimbatm/nix-experiments/chronixpkgs/cmd/chronixpkgs"
)

func main() {
	if err := chronixpkgs.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
