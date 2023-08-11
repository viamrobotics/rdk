// Package main is the CLI command itself.
package main

import (
	"os"

	"go.viam.com/rdk/cli"
)

func main() {
	app := cli.NewApp(os.Stdout, os.Stderr)
	if err := app.Run(os.Args); err != nil {
		cli.Errorf(app.ErrWriter, err.Error())
	}
}
