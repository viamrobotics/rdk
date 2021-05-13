package rimage

import (
	"testing"

	"go.viam.com/test"
)

type rangeArrayHelper struct {
	dim      int
	expected []int
}

func TestRangeArray(t *testing.T) {
	cases := []rangeArrayHelper{
		{5, []int{-2, -1, 0, 1, 2}},
		{4, []int{-2, -1, 0, 1}},
		{3, []int{-1, 0, 1}},
		{2, []int{-1, 0}},
		{1, []int{0}},
		{0, []int{}},
		{-2, []int{}},
	}
	for _, c := range cases {
		got := makeRangeArray(c.dim)
		test.That(t, c.expected, test.ShouldResemble, got)
	}
}

func TestStructuringElement(t *testing.T) {
	expected := &DepthMap{3, 3, []Depth{0, 1, 0, 1, 1, 1, 0, 1, 0}}
	got := makeStructuringElement(3)
	test.That(t, expected, test.ShouldResemble, got)

}
