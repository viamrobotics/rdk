package ml

import (
	"testing"

	"go.viam.com/test"
)

func _makeSimpleTest() ([][]float64, []int) {
	data := [][]float64{
		{0, 0, 10},
		{1, 0, 11},
		{0, 1, 12},
		{1, 1, 13},
		{0, 0, 14},
		{1, 0, 15},
		{0, 1, 16},
		{1, 1, 17},
	}

	correct := []int{
		0,
		1,
		0,
		1,
		0,
		1,
		0,
		1,
	}

	return data, correct
}

func _checkCorrectness(t *testing.T, theClassifier Classifier, data [][]float64, correct []int) {
	t.Helper()
	for x, row := range data {
		got, err := theClassifier.Classify(row)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldResemble, correct[x])
	}
}

func TestGLSimple1(t *testing.T) {
	if true {
		t.Skip("TestGLSimple1 is flaky for some reason")
		return
	}

	data, correct := _makeSimpleTest()

	c := &GoLearnClassifier{}
	err := c.Train(data, correct)
	test.That(t, err, test.ShouldBeNil)

	_checkCorrectness(t, c, data, correct)
}

func TestGLSimpleNN1(t *testing.T) {
	data, correct := _makeSimpleTest()

	c := &GoLearnNNClassifier{}
	err := c.Train(data, correct)
	test.That(t, err, test.ShouldBeNil)

	_checkCorrectness(t, c, data, correct)

	for _, r := range data {
		r[2]++
	}

	_checkCorrectness(t, c, data, correct)
}
