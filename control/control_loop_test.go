package control

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/core/config"
	"go.viam.com/test"
)

type wrapBlocks struct {
	c ControlBlockConfig
	x int
	y int
}

func generateNInputs(n int, y_max int, baseName string) []wrapBlocks {
	out := make([]wrapBlocks, n)

	out[0].c = ControlBlockConfig{
		Name: "",
		Type: "Endpoint",
		Attribute: config.AttributeMap{
			"MotorName": "MotorFake",
		},
		DependsOn: []string{},
	}
	out[0].x = 0
	out[0].y = 0
	out[0].c.Name = fmt.Sprintf("%s%d", baseName, 0)
	for i := 1; i < n; i++ {
		out[i].c = ControlBlockConfig{Name: "S1",
			Type: "Constant",
			Attribute: config.AttributeMap{
				"ConstantVal": 3.0,
			},
			DependsOn: []string{},
		}
		out[i].x = 0
		out[i].y = i
		out[i].c.Name = fmt.Sprintf("%s%d", baseName, i)
	}
	return out
}
func generateNSums(n int, x_max int, y_max int, baseName string, ins []wrapBlocks) []wrapBlocks {
	b := wrapBlocks{
		c: ControlBlockConfig{
			Name: "",
			Type: "Sum",
			Attribute: config.AttributeMap{
				"SumString": "",
			},
			DependsOn: []string{},
		},
		x: x_max - 1,
		y: (y_max - 1) / 2,
	}
	b.c.Name = fmt.Sprintf("%s%d", baseName, 0)
	ins = append(ins, b)
	for i := 1; i < n; i++ {
		x_r := 0
		y_r := 0
		for {
			x_r = rand.Intn(x_max-2) + 2
			y_r = rand.Intn(y_max)
			j := 0
			for ; j < len(ins); j++ {
				if ins[j].x == x_r && ins[j].y == y_r {
					j = 0
					break
				}
			}
			if j == len(ins) {
				break
			}
		}
		b = wrapBlocks{
			c: ControlBlockConfig{
				Name: "",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumString": "",
				},
				DependsOn: []string{},
			},
			x: x_r,
			y: y_r,
		}
		b.c.Name = fmt.Sprintf("%s%d", baseName, i)
		ins = append(ins, b)
	}
	return ins
}

func generateNBlocks(n int, x_max int, y_max int, baseName string, ins []wrapBlocks) []wrapBlocks {
	for i := 0; i < n; i++ {
		x_r := 0
		y_r := 0
		for {
			x_r = rand.Intn(x_max-1) + 1
			y_r = rand.Intn(y_max)
			j := 0
			for ; j < len(ins); j++ {
				if ins[j].x == x_r && ins[j].y == y_r {
					j = 0
					break
				}
			}
			if j == len(ins) {
				break
			}
		}
		b := wrapBlocks{
			c: ControlBlockConfig{
				Name: "C",
				Type: "Gain",
				Attribute: config.AttributeMap{
					"Gain": -2.0,
				},
				DependsOn: []string{},
			},
			x: x_r,
			y: y_r,
		}
		b.c.Name = fmt.Sprintf("%s%d", baseName, i)
		ins = append(ins, b)
	}
	return ins
}

func findVerticalBlock(x_start int, x_max int, y_start int, grid [][]*wrapBlocks) *wrapBlocks {
	for i := x_start + 1; i < x_max; i++ {
		if grid[i][y_start] != nil {
			return grid[i][y_start]
		}
	}
	return nil
}

func findSumHalfSquare(x_max int, y_max int, x_start int, y_start int, grid [][]*wrapBlocks) *wrapBlocks {
	for i := x_start + 1; i < int(math.Max(float64(x_max), float64(x_start+1))); i++ {
		for j := y_start - 1; j < y_start+1; j++ {
			if i > x_max-1 || j > y_max-1 || i < 0 || j < 0 {
				continue
			}
			if grid[i][j] != nil && (grid[i][j].c.Type == "Sum") {
				return grid[i][j]
			}
		}
	}
	return nil
}

func mergedAll(x_max int, y_max int, grid [][]*wrapBlocks, def *wrapBlocks) {
	for i, l := range grid {
		for j, b := range l {
			if b == nil {
				continue
			}
			n := findVerticalBlock(i, x_max, j, grid)
			if n == nil {
				n = findSumHalfSquare(x_max, y_max, i, j, grid)
				if n == nil {
					if b != def {
						def.c.DependsOn = append(def.c.DependsOn, b.c.Name)
					}
					continue
				}
			}
			n.c.DependsOn = append(n.c.DependsOn, b.c.Name)
			if n.c.Type != "Sum" {
				n = findSumHalfSquare(x_max, y_max, i, j, grid)
				if n != nil {
					n.c.DependsOn = append(n.c.DependsOn, b.c.Name)
				}
			}
		}
	}
}

func benchNBlocks(n int, freq float64, b *testing.B) {
	rand.Seed(time.Now().UTC().UnixNano())
	if n < 10 {
		return
	}
	n_obj := n
	n_I := 1 + int(float64(n)*0.2)
	n_obj -= n_I
	n_S := 1 + int(float64(n)*0.2)
	n_obj -= n_S
	n_B := n_obj
	y_max := n_I
	x_max := n/y_max + 2
	out := generateNInputs(n_I, x_max, "Inputs")
	out = generateNSums(n_S, x_max, y_max, "Sums", out)
	out = generateNBlocks(n_B, x_max, y_max, "Blocks", out)
	lastSum := &out[n_I]
	grid := make([][]*wrapBlocks, x_max)
	for i := range grid {
		grid[i] = make([]*wrapBlocks, y_max)
	}
	for i, b := range out {
		grid[b.x][b.y] = &out[i]
	}
	mergedAll(x_max, y_max, grid, lastSum)

	cfg := ControlConfig{
		Frequency: freq,
		Blocks:    []ControlBlockConfig{},
	}
	for i := range out {
		if out[i].c.Type == "Sum" {
			out[i].c.Attribute["SumString"] = strings.Repeat("+", len(out[i].c.DependsOn))
		}
		cfg.Blocks = append(cfg.Blocks, out[i].c)
	}
	logger := golog.NewLogger("Bench")
	ctx := context.Background()
	cloop, err := createControlLoop(ctx, logger, cfg, nil)
	if err == nil {
		b.ResetTimer()
		cloop.StartBenchmark(ctx, b.N)
		cloop.activeBackgroundWorkers.Wait()
	}

}

func BenchmarkLoop10(b *testing.B) {
	benchNBlocks(10, 100.0, b)
}
func BenchmarkLoop30(b *testing.B) {
	benchNBlocks(30, 100.0, b)
}
func BenchmarkLoop100(b *testing.B) {
	benchNBlocks(100, 100.0, b)
}
func TestControlLoop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cfg := ControlConfig{
		Blocks: []ControlBlockConfig{
			{
				Name: "A",
				Type: "Endpoint",
				Attribute: config.AttributeMap{
					"MotorName": "MotorFake",
				},
				DependsOn: []string{"E"},
			},
			{
				Name: "B",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumString": "+-",
				},
				DependsOn: []string{"A", "S1"},
			},
			{
				Name: "S1",
				Type: "Constant",
				Attribute: config.AttributeMap{
					"ConstantVal": 3.0,
				},
				DependsOn: []string{},
			},
			{
				Name: "C",
				Type: "Gain",
				Attribute: config.AttributeMap{
					"Gain": -2.0,
				},
				DependsOn: []string{"B"},
			},
			{
				Name: "D",
				Type: "Sum",
				Attribute: config.AttributeMap{
					"SumString": "+-",
				},
				DependsOn: []string{"C", "S2"},
			},
			{
				Name: "S2",
				Type: "Constant",
				Attribute: config.AttributeMap{
					"ConstantVal": 10.0,
				},
				DependsOn: []string{},
			},
			{
				Name: "E",
				Type: "Gain",
				Attribute: config.AttributeMap{
					"Gain": -2.0,
				},
				DependsOn: []string{"D"},
			},
		},
		Frequency: 20.0,
	}
	cLoop, err := createControlLoop(ctx, logger, cfg, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cLoop, test.ShouldNotBeNil)
	cLoop.Start(ctx)
	for i := 200; i > 0; i-- {
		time.Sleep(65 * time.Millisecond)
		b, err := cLoop.OutputAt(ctx, "E")
		test.That(t, b[0].signal[0], test.ShouldEqual, 8.0)
		test.That(t, err, test.ShouldBeNil)
		b, err = cLoop.OutputAt(ctx, "B")
		test.That(t, b[0].signal[0], test.ShouldEqual, -3.0)
		test.That(t, err, test.ShouldBeNil)
	}
	cLoop.Stop(ctx)
}
