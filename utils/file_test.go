package utils

import (
	"bytes"
	"os"
	"testing"

	"go.viam.com/test"
)

func TestResolveFile(t *testing.T) {
	sentinel := "great"
	_ = sentinel
	resolved := ResolveFile("utils/file_test.go")
	rd, err := os.ReadFile(resolved)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bytes.Contains(rd, []byte(`sentinel := "great"`)), test.ShouldBeTrue)
}
