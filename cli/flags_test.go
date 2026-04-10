package cli

import (
	"testing"

	"github.com/urfave/cli/v3"
	"go.viam.com/test"
)

//nolint:dupl
func TestAliasStringFlag(t *testing.T) {
	f := AliasStringFlag{
		cli.StringFlag{
			Name: "foo",
		},
	}
	stringRepresentationWithoutAliases := f.StringFlag.String()
	test.That(t, f.Names(), test.ShouldResemble, []string{"foo"})
	test.That(t, f.String(), test.ShouldEqual, stringRepresentationWithoutAliases)

	f = AliasStringFlag{
		cli.StringFlag{
			Name:    "foo",
			Aliases: []string{"hello"},
		},
	}
	test.That(t, f.Names(), test.ShouldResemble, []string{"hello", "foo"})
	test.That(t, f.String(), test.ShouldEqual, stringRepresentationWithoutAliases)

	f = AliasStringFlag{
		cli.StringFlag{
			Name:    "foo",
			Aliases: []string{"hello", "world"},
		},
	}
	test.That(t, f.Names(), test.ShouldResemble, []string{"hello", "world", "foo"})
	test.That(t, f.String(), test.ShouldEqual, stringRepresentationWithoutAliases)
}

//nolint:dupl
func TestAliasStringSliceFlag(t *testing.T) {
	f := AliasStringSliceFlag{
		cli.StringSliceFlag{
			Name: "foo",
		},
	}
	stringRepresentationWithoutAliases := f.StringSliceFlag.String()
	test.That(t, f.Names(), test.ShouldResemble, []string{"foo"})
	test.That(t, f.String(), test.ShouldEqual, stringRepresentationWithoutAliases)

	f = AliasStringSliceFlag{
		cli.StringSliceFlag{
			Name:    "foo",
			Aliases: []string{"hello"},
		},
	}
	test.That(t, f.Names(), test.ShouldResemble, []string{"hello", "foo"})
	test.That(t, f.String(), test.ShouldEqual, stringRepresentationWithoutAliases)

	f = AliasStringSliceFlag{
		cli.StringSliceFlag{
			Name:    "foo",
			Aliases: []string{"hello", "world"},
		},
	}
	test.That(t, f.Names(), test.ShouldResemble, []string{"hello", "world", "foo"})
	test.That(t, f.String(), test.ShouldEqual, stringRepresentationWithoutAliases)
}
