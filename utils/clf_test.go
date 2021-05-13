package utils

import (
	"os"
	"testing"

	"go.viam.com/core/artifact"

	"go.viam.com/test"
)

func TestCLF(t *testing.T) {
	f, err := os.Open(artifact.MustPath("utils/aces_sample.clf"))
	test.That(t, err, test.ShouldBeNil)
	defer f.Close()

	clf := NewCLFReader(f)

	numMessages := 0
	var haveAnOdom bool
	err = clf.Process(func(message CLFMessage) error {
		numMessages++
		if message.Type() == CLFMessageTypeOdometry {
			haveAnOdom = true
		}
		return nil
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numMessages, test.ShouldEqual, 20)
	test.That(t, haveAnOdom, test.ShouldBeTrue)
}
