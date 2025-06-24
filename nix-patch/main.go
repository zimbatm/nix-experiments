package main

import (
	"fmt"
	"log"
	"os"

	"github.com/zimbatm/nix-experiments/nix-store-edit/cmd"
	"github.com/zimbatm/nix-experiments/nix-store-edit/internal/errors"
)

func main() {
	log.SetPrefix("")
	log.SetFlags(0)

	if err := cmd.Execute(); err != nil {
		// Format the error nicely for users
		fmt.Fprintf(os.Stderr, "Error: %s\n", errors.Format(err))

		// Exit with appropriate code
		if errors.IsCode(err, errors.ErrCodeConfig) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
