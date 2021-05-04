package utils

import (
	"fmt"
	"testing"

	"go.viam.com/test"
)

func TestRandomAlphaString(t *testing.T) {
	for _, tc := range []int{-1, 0, 1, 2, 3, 4, 5} {
		t.Run(fmt.Sprintf("size %d", tc), func(t *testing.T) {
			str := RandomAlphaString(tc)
			if tc <= 0 {
				test.That(t, str, test.ShouldBeEmpty)
				return
			}
			test.That(t, str, test.ShouldHaveLength, tc)
		})
	}
}

func TestStringSet(t *testing.T) {
	ss := NewStringSet("foo")
	_, ok := ss["foo"]
	test.That(t, ok, test.ShouldBeTrue)
}
