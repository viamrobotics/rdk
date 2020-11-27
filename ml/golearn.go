package ml

import (
	"fmt"
	"math"

	"github.com/sjwhitworth/golearn/base"
	"github.com/sjwhitworth/golearn/knn"
	"github.com/sjwhitworth/golearn/neural"
)

type GoLearnClassifier struct {
	theClassifier base.Classifier
	format        *base.DenseInstances
}

func (c *GoLearnClassifier) Classify(data []float64) (int, error) {
	di := _glMakeClassifyDataSet(c.format, data)

	res, err := c.theClassifier.Predict(di)
	if err != nil {
		return 0, err
	}

	return _glReturnSingleResult(res), nil
}

func (c *GoLearnClassifier) Train(data [][]float64, correct []int) error {
	rawData, err := _glMakeDataSet(data, correct)
	if err != nil {
		return err
	}

	c.format = base.NewStructuralCopy(rawData)

	c.theClassifier = knn.NewKnnClassifier("euclidean", "linear", 2)

	return c.theClassifier.Fit(rawData)
}

// ---------
type GoLearnNNClassifier struct {
	theClassifier *neural.MultiLayerNet
	format        *base.DenseInstances
}

func (c *GoLearnNNClassifier) Classify(data []float64) (int, error) {
	di := _glMakeClassifyDataSet(c.format, data)
	res := c.theClassifier.Predict(di)
	return _glReturnSingleResult(res), nil
}

func (c *GoLearnNNClassifier) Train(data [][]float64, correct []int) error {
	rawData, err := _glMakeDataSet(data, correct)
	if err != nil {
		return err
	}

	c.format = base.NewStructuralCopy(rawData)

	c.theClassifier = neural.NewMultiLayerNet([]int{10})

	c.theClassifier.Fit(rawData)
	return nil
}

// --------

func _glMakeClassifyDataSet(format *base.DenseInstances, data []float64) *base.DenseInstances {
	di := base.NewStructuralCopy(format)
	di.Extend(1)
	attrs := di.AllAttributes()
	for x, a := range attrs {
		if x >= len(data) {
			break
		}
		spec, err := di.GetAttribute(a)
		if err != nil {
			panic(err) // internal err
		}
		di.Set(spec, 0, base.PackFloatToBytes(data[x]))
	}

	return di
}

func _glReturnSingleResult(res base.FixedDataGrid) int {
	attrs := res.AllAttributes()
	if len(attrs) != 1 {
		panic("this sucks")
	}
	spec, err := res.GetAttribute(attrs[0])
	if err != nil {
		panic(err) // intetrnal error
	}

	raw := res.Get(spec, 0)
	if len(raw) != 8 {
		panic("wtf")
	}

	return int(math.Round(base.UnpackBytesToFloat(raw)))
}

func _glMakeDataSet(data [][]float64, correct []int) (base.FixedDataGrid, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data")
	}

	if len(data) != len(correct) {
		return nil, fmt.Errorf("data and correct not the same lenghts %d %d", len(data), len(correct))
	}

	rawData := base.NewDenseInstances()
	specs := make([]base.AttributeSpec, len(data[0])+1)
	for x, _ := range data[0] {
		a := base.NewFloatAttribute(fmt.Sprintf("v%d", x))
		specs[x] = rawData.AddAttribute(a)
	}
	ca := base.NewFloatAttribute("res")
	specs[len(data[0])] = rawData.AddAttribute(ca)
	rawData.AddClassAttribute(ca)

	rawData.Extend(len(data))
	for x, row := range data {
		for y, v := range row {
			rawData.Set(specs[y], x, base.PackFloatToBytes(v))
		}
		rawData.Set(specs[len(row)], x, base.PackFloatToBytes(float64(correct[x])))
	}

	return rawData, nil
}
