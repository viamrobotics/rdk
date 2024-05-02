package classification

import (
	"testing"

	"go.viam.com/test"
)

func TestLabelConfidencePostprocessor(t *testing.T) {
	d := []Classification{
		NewClassification(0.5, "A"),
		NewClassification(0.1, "a"),
		NewClassification(0.1, "B"),
		NewClassification(0.6, "b"),
		NewClassification(1, "C"),
		NewClassification(0.8773934448, "D"),
	}

	postNoFilter := NewLabelConfidenceFilter(nil) // no filtering
	results := postNoFilter(d)
	test.That(t, len(results), test.ShouldEqual, len(d))

	label := map[string]float64{"a": 0.5, "B": 0.5}
	postFilter := NewLabelConfidenceFilter(label)
	results = postFilter(d)
	test.That(t, len(results), test.ShouldEqual, 2)
	labelList := make([]string, 2)
	for _, g := range results {
		labelList = append(labelList, g.Label())
	}
	test.That(t, labelList, test.ShouldContain, "A")
	test.That(t, labelList, test.ShouldContain, "b")
}
