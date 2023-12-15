package cli

import (
	"testing"

	"github.com/urfave/cli/v2"
	"go.viam.com/test"
)

func TestAliasStringFlag(t *testing.T) {
	f := AliasStringFlag{
		cli.StringFlag{
			Name: "foo",
		},
	}
	test.That(t, f.Names(), test.ShouldResemble, []string{"foo"})
	test.That(t, f.String(), test.ShouldEqual, f.StringFlag.String())

	f = AliasStringFlag{
		cli.StringFlag{
			Name:    "foo",
			Aliases: []string{"hello"},
		},
	}
	test.That(t, f.Names(), test.ShouldResemble, []string{"hello", "foo"})
	test.That(t, f.String(), test.ShouldEqual, f.StringFlag.String())

	f = AliasStringFlag{
		cli.StringFlag{
			Name:    "foo",
			Aliases: []string{"hello", "world"},
		},
	}
	test.That(t, f.Names(), test.ShouldResemble, []string{"hello", "world", "foo"})
	test.That(t, f.String(), test.ShouldEqual, f.StringFlag.String())
}
