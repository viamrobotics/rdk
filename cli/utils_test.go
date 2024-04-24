package cli

import (
	"testing"

	"go.viam.com/test"
)

func TestMapOver(t *testing.T) {
	mapped, _ := mapOver([]int{1, 2}, func(x int) (int, error) { return x + 1, nil })
	test.That(t, mapped, test.ShouldResemble, []int{2, 3})
}

func TestSamePath(t *testing.T) {
	equal, _ := samePath("/x", "/x")
	test.That(t, equal, test.ShouldBeTrue)
	equal, _ = samePath("/x", "x")
	test.That(t, equal, test.ShouldBeFalse)
}

func TestGetMapString(t *testing.T) {
	m := map[string]any{
		"x": "x",
		"y": 10,
	}
	test.That(t, getMapString(m, "x"), test.ShouldEqual, "x")
	test.That(t, getMapString(m, "y"), test.ShouldEqual, "")
	test.That(t, getMapString(m, "z"), test.ShouldEqual, "")
}
