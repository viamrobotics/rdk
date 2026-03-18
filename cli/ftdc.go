package cli

import (
	"context"

	"github.com/urfave/cli/v3"

	"go.viam.com/rdk/ftdc/parser"
)

type ftdcArgs struct {
	Path string
}

// FTDCParseAction is the cli action to parse an ftdc file.
func FTDCParseAction(ctx context.Context, cmd *cli.Command, args ftdcArgs) error {
	parser.LaunchREPL(args.Path)
	return nil
}
