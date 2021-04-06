package utils

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/edaniels/test"
)

func TestResolveFile(t *testing.T) {
	var sentinel = "great"
	_ = sentinel
	resolved := ResolveFile("utils/file_test.go")
	rd, err := ioutil.ReadFile(resolved)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bytes.Contains(rd, []byte("var sentinel = \"great\"")), test.ShouldBeTrue)
}
