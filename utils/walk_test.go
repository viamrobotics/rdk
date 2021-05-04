package utils

import (
	"fmt"
	"testing"

	"go.viam.com/test"
)

func TestWalk1(t *testing.T) {

	m := map[string]int{}

	Walk(1, 1, 1, func(x, y int) error {
		s := fmt.Sprintf("%d,%d", x, y)
		old := m[s]
		m[s] = old + 1
		return nil
	})

	test.That(t, m, test.ShouldHaveLength, 9)

	for k, v := range m {
		t.Run(k, func(t *testing.T) {
			test.That(t, v, test.ShouldEqual, 1)
		})
	}
}
