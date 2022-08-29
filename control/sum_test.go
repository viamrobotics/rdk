package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestSumConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	for _, c := range []struct {
		conf BlockConfig
		err  string
	}{
		{
			BlockConfig{
				Name: "Sum1",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"sum_string": "--++",
				},
				DependsOn: []string{"A", "B", "C", "D"},
			},
			"",
		},
		{
			BlockConfig{
				Name: "Sum1",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"sum_stringS": "--++",
				},
				DependsOn: []string{"A", "B", "C", "D"},
			},
			"sum block Sum1 doesn't have a sum_string",
		},
		{
			BlockConfig{
				Name: "Sum1",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"sum_string": "--++",
				},
				DependsOn: []string{"B", "C", "D"},
			},
			"invalid number of inputs for sum block Sum1 expected 4 got 3",
		},
		{
			BlockConfig{
				Name: "Sum1",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"sum_string": "--+\\",
				},
				DependsOn: []string{"A", "B", "C", "D"},
			},
			"expected +/- for sum block Sum1 got \\",
		},
	} {
		b, err := newSum(c.conf, logger)
		if c.err == "" {
			s := b.(*sum)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(s.y), test.ShouldEqual, 1)
		} else {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldResemble, c.err)
		}
	}
}

func TestSumNext(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := BlockConfig{
		Name: "Sum1",
		Type: "Sum",
		Attribute: config.AttributeMap{
			"sum_string": "--++",
		},
		DependsOn: []string{"A", "B", "C", "D"},
	}
	s, err := newSum(c, logger)

	test.That(t, err, test.ShouldBeNil)

	signals := []Signal{
		{
			name:      "A",
			signal:    []float64{1.0},
			time:      []int{1},
			dimension: 1,
			mu:        &sync.Mutex{},
		},
		{
			name:      "B",
			signal:    []float64{2.0},
			time:      []int{2},
			dimension: 1,
			mu:        &sync.Mutex{},
		},
		{
			name:      "C",
			signal:    []float64{1.0},
			time:      []int{2},
			dimension: 1,
			mu:        &sync.Mutex{},
		},
		{
			name:      "D",
			signal:    []float64{1.0},
			time:      []int{1},
			dimension: 1,
			mu:        &sync.Mutex{},
		},
	}
	out, ok := s.Next(ctx, signals, time.Millisecond*1)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, out[0].GetSignalValueAt(0), test.ShouldEqual, -1.0)
}
