package utils

import (
	"os"
	"testing"

	"go.viam.com/robotcore/artifact"

	"github.com/stretchr/testify/assert"
)

func TestCLF(t *testing.T) {
	f, err := os.Open(artifact.MustPath("utils/aces_sample.clf"))
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 20, numMessages)
	assert.True(t, haveAnOdom)
}
