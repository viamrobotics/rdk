package ml

import (
	"testing"
)

func TestSimple1(t *testing.T) {

	data := [][]float64{
		[]float64{0, 0, 5},
		[]float64{1, 0, 6},
		[]float64{0, 1, 1},
		[]float64{1, 1, 3},
		[]float64{0, 0, 4},
		[]float64{1, 0, 1},
		[]float64{0, 1, 2},
		[]float64{1, 1, 3},
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

	c := GoLearnClassifier{}
	err := c.Train(data, correct)
	if err != nil {
		panic(err)
	}

	for x, row := range data {
		got, err := c.Classify(row)
		if err != nil {
			t.Fatal(err)
		}
		if got != correct[x] {
			t.Errorf("wrong result for row %d, data: %v correct: %v got: %v", x, row, correct[x], got)
		}
	}
}

func TestSimpleNN1(t *testing.T) {

	data := [][]float64{
		[]float64{0, 0, 5},
		[]float64{1, 0, 6},
		[]float64{0, 1, 1},
		[]float64{1, 1, 3},
		[]float64{0, 0, 4},
		[]float64{1, 0, 1},
		[]float64{0, 1, 2},
		[]float64{1, 1, 3},
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

	c := GoLearnNNClassifier{}
	err := c.Train(data, correct)
	if err != nil {
		panic(err)
	}

	for x, row := range data {
		got, err := c.Classify(row)
		if err != nil {
			t.Fatal(err)
		}
		if got != correct[x] {
			t.Errorf("wrong result for row %d, data: %v correct: %v got: %v", x, row, correct[x], got)
		}
	}

	for _, r := range data {
		r[2] = r[2] + 1
	}

	for x, row := range data {
		got, err := c.Classify(row)
		if err != nil {
			t.Fatal(err)
		}
		if got != correct[x] {
			t.Errorf("wrong result for row %d, data: %v correct: %v got: %v", x, row, correct[x], got)
		}
	}

}
