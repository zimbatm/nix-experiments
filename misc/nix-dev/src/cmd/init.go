package cmd

import (
	"fmt"

	"github.com/urfave/cli"
)

// Commands is the list of commands
var Commands []cli.Command

var initCmd = cli.Command{
	Name: "init",
	Action: func(ctx *cli.Context) {
		fmt.Println("init")
	},
}

func init() {
	Commands = append(Commands, initCmd)
}
