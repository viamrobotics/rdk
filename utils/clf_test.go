package utils_test

import (
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/utils"
)

func TestCLF(t *testing.T) {
	f, err := os.Open(artifact.MustPath("utils/aces_sample.clf"))
	test.That(t, err, test.ShouldBeNil)
	defer f.Close()

	clf := utils.NewCLFReader(f)

	numMessages := 0
	var haveAnOdom bool
	err = clf.Process(func(message utils.CLFMessage) error {
		numMessages++
		if message.Type() == utils.CLFMessageTypeOdometry {
			haveAnOdom = true
		}
		return nil
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numMessages, test.ShouldEqual, 20)
	test.That(t, haveAnOdom, test.ShouldBeTrue)
}
