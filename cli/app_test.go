package cli

import (
	"fmt"
	"io"
	"testing"

	"github.com/urfave/cli/v3"
	"go.viam.com/test"
)

// uintFlagsNamed returns every cli.UintFlag called <name> anywhere in the command tree, so a test
// can assert against all of them without naming the commands that carry it.
func uintFlagsNamed(cmds []*cli.Command, name string) []*cli.UintFlag {
	var found []*cli.UintFlag
	for _, cmd := range cmds {
		for _, f := range cmd.Flags {
			if uintFlag, ok := f.(*cli.UintFlag); ok && uintFlag.Name == name {
				found = append(found, uintFlag)
			}
		}
		found = append(found, uintFlagsNamed(cmd.Commands, name)...)
	}
	return found
}

// TestParallelFlagRejectsZero asserts every --parallel flag rejects an explicit 0 while leaving
// its default intact. Searching the tree (rather than naming the commands)
// means future --parallel flags added without the validator will fail here.
func TestParallelFlagRejectsZero(t *testing.T) {
	flags := uintFlagsNamed(NewApp(io.Discard, io.Discard).Commands, dataFlagParallelDownloads)
	// Guards the search itself: if it stops finding flags, the loop below asserts nothing.
	test.That(t, len(flags), test.ShouldBeGreaterThan, 0)

	for _, flag := range flags {
		test.That(t, flag.Validator, test.ShouldNotBeNil)
		test.That(t, flag.Validator(0), test.ShouldBeError,
			fmt.Errorf("--%s must be greater than 0", dataFlagParallelDownloads))
		// Whatever default the flag carries has to survive its own validator.
		test.That(t, flag.Validator(1), test.ShouldBeNil)
		test.That(t, flag.Validator(flag.Value), test.ShouldBeNil)
	}
}
