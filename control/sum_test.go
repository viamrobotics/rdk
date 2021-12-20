package control

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/core/config"
)

func TestSumConfig(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		conf ControlBlockConfig
		err  string
	}{
		{
			ControlBlockConfig{
				Name: "Sum1",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumString": "--++",
				},
				DependsOn: []string{"A", "B", "C", "D"},
			},
			"",
		},
		{
			ControlBlockConfig{
				Name: "Sum1",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumStringS": "--++",
				},
				DependsOn: []string{"A", "B", "C", "D"},
			},
			"sum block Sum1 doesn't have a SumString",
		},
		{
			ControlBlockConfig{
				Name: "Sum1",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumString": "--++",
				},
				DependsOn: []string{"B", "C", "D"},
			},
			"invalid number of inputs for sum block Sum1 expected 4 got 3",
		},
		{
			ControlBlockConfig{
				Name: "Sum1",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumString": "--+\\",
				},
				DependsOn: []string{"A", "B", "C", "D"},
			},
			"expected +/- for sum block Sum1 got \\",
		},
	} {
		var s sum

		err := s.Configure(ctx, c.conf)
		if c.err == "" {
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
	c := ControlBlockConfig{
		Name: "Sum1",
		Type: "Sum",
		Attribute: config.AttributeMap{
			"SumString": "--++",
		},
		DependsOn: []string{"A", "B", "C", "D"},
	}
	var s sum
	err := s.Configure(ctx, c)

	test.That(t, err, test.ShouldBeNil)

	signals := []Signal{
		{name: "A",
			signal:    []float64{1.0},
			time:      []int{1},
			dimension: 1,
			mu:        &sync.Mutex{},
		},
		{name: "B",
			signal:    []float64{2.0},
			time:      []int{2},
			dimension: 1,
			mu:        &sync.Mutex{},
		},
		{name: "C",
			signal:    []float64{1.0},
			time:      []int{2},
			dimension: 1,
			mu:        &sync.Mutex{},
		},
		{name: "D",
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
