package slam

import (
	"testing"

	"github.com/edaniels/test"
)

func TestNewFromFile(t *testing.T) {
	_, err := NewFromFile("test.las")
	test.That(t, err, test.ShouldBeNil)
}
