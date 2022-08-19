package objectdetection

import (
	"image"
	"testing"

	"go.viam.com/test"
)

func TestPostprocessors(t *testing.T) {
	d := []Detection{
		NewDetection(image.Rect(0, 0, 30, 30), 0.5, "A"),
		NewDetection(image.Rect(0, 0, 300, 300), 0.6, "B"),
		NewDetection(image.Rect(150, 150, 310, 310), 1, "C"),
		NewDetection(image.Rect(50, 50, 200, 200), 0.8773934448, "D"),
	}
	sorter := SortByArea()
	got := sorter(d)
	test.That(t, got[0].Label(), test.ShouldEqual, "B")
	test.That(t, got[1].Label(), test.ShouldEqual, "C")
	test.That(t, got[2].Label(), test.ShouldEqual, "D")
	test.That(t, got[3].Label(), test.ShouldEqual, "A")

	areaFilt := NewAreaFilter(1000)
	got = areaFilt(d)
	labelList := make([]string, 4)
	for _, g := range got {
		labelList = append(labelList, g.Label())
	}
	test.That(t, len(got), test.ShouldEqual, 3)
	test.That(t, labelList, test.ShouldNotContain, "A")
	test.That(t, labelList, test.ShouldContain, "B")
	test.That(t, labelList, test.ShouldContain, "C")
	test.That(t, labelList, test.ShouldContain, "D")

	scoreFilt := NewScoreFilter(0.7)
	got = scoreFilt(d)
	labelList = make([]string, 4)
	for _, g := range got {
		labelList = append(labelList, g.Label())
	}
	test.That(t, len(got), test.ShouldEqual, 2)
	test.That(t, labelList, test.ShouldNotContain, "A")
	test.That(t, labelList, test.ShouldNotContain, "B")
	test.That(t, labelList, test.ShouldContain, "C")
	test.That(t, labelList, test.ShouldContain, "D")
}
