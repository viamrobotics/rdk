package cli

import "github.com/urfave/cli/v2"

// AliasStringFlag returns f.Name as the last member of Names(), and hides aliases from the
// String representation of the flag. This is useful for decluttering error messages and help
// text when aliases shouldn't be exposed to the user. Otherwise it is the same as cli.StringFlag.
type AliasStringFlag struct {
	cli.StringFlag
}

// Names have to be overwritten to prevent required flag errors from using aliases in its message.
// This returns f.Name as the last member of Names(), which is what the required flag error uses in its message.
func (f AliasStringFlag) Names() []string {
	var names []string
	names = append(names, f.Aliases...)
	names = append(names, f.Name)
	return cli.FlagNames(names[0], names[1:])
}

func (f *AliasStringFlag) String() string {
	aliases := f.Aliases
	f.Aliases = []string{}
	s := cli.FlagStringer(f)
	f.Aliases = aliases
	return s
}
