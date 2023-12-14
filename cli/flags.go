package cli

import "github.com/urfave/cli/v2"

type ViamStringFlag struct {
	cli.StringFlag
}

// Names have to be overwritten to prevent required flag errors from using aliases in its message.
// This returns f.Name as the last member of Names().
func (f ViamStringFlag) Names() []string {
	names := append(f.Aliases, f.Name)
	return cli.FlagNames(names[0], names[1:])
}
