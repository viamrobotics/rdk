package vision

import (
	"fmt"
	"testing"
)

func TestWalk1(t *testing.T) {

	m := map[string]int{}

	Walk(1, 1, 1, func(x, y int) error {
		s := fmt.Sprintf("%d,%d", x, y)
		old := m[s]
		m[s] = old + 1
		return nil
	})

	if len(m) != 9 {
		t.Errorf("wrong number %d", len(m))
	}

	for k, v := range m {
		if v != 1 {
			t.Errorf("wrong %s %d", k, v)
		}
	}
}
