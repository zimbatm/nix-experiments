package main

import (
	"log"
	"os"

	"github.com/urfave/cli"
	"nix-dev/src/cmd"
)

func main() {
	app := cli.NewApp()
	app.Usage = "a nix-based developer environment"
	app.Commands = cmd.Commands

	err := app.Run(os.Args)

	if err != nil {
		log.Fatal(err)
	}
}
