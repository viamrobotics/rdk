package cli

import (
	"github.com/urfave/cli/v2"

	"go.viam.com/rdk/ftdc/parser"
)

type ftdcArgs struct {
	Path string
}

// FTDCParseAction is the cli action to parse an ftdc file.
func FTDCParseAction(c *cli.Context, args ftdcArgs) error {
	parser.LaunchREPL(args.Path)
	return nil
}
