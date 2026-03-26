// Package main is the CLI command itself.
package main

import (
	"context"
	"os"

	"go.viam.com/rdk/cli"
)

func main() {
	app := cli.NewApp(os.Stdout, os.Stderr)
	if err := app.Run(context.Background(), os.Args); err != nil {
		cli.Errorf(app.ErrWriter, err.Error())
	}
}
