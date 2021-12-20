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
	"go.viam.com/test"

	"go.viam.com/core/config"
)

type wrapBlocks struct {
	c ControlBlockConfig
	x int
	y int
}

func generateNInputs(n int, yMax int, baseName string) []wrapBlocks {
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
func generateNSums(n int, xMax int, yMax int, baseName string, ins []wrapBlocks) []wrapBlocks {
	b := wrapBlocks{
		c: ControlBlockConfig{
			Name: "",
			Type: "Sum",
			Attribute: config.AttributeMap{
				"SumString": "",
			},
			DependsOn: []string{},
		},
		x: xMax - 1,
		y: (yMax - 1) / 2,
	}
	b.c.Name = fmt.Sprintf("%s%d", baseName, 0)
	ins = append(ins, b)
	for i := 1; i < n; i++ {
		xR := 0
		yR := 0
		for {
			xR = rand.Intn(xMax-2) + 2
			yR = rand.Intn(yMax)
			j := 0
			for ; j < len(ins); j++ {
				if ins[j].x == xR && ins[j].y == yR {
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
			x: xR,
			y: yR,
		}
		b.c.Name = fmt.Sprintf("%s%d", baseName, i)
		ins = append(ins, b)
	}
	return ins
}

func generateNBlocks(n int, xMax int, yMax int, baseName string, ins []wrapBlocks) []wrapBlocks {
	for i := 0; i < n; i++ {
		xR := 0
		yR := 0
		for {
			xR = rand.Intn(xMax-1) + 1
			yR = rand.Intn(yMax)
			j := 0
			for ; j < len(ins); j++ {
				if ins[j].x == xR && ins[j].y == yR {
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
			x: xR,
			y: yR,
		}
		b.c.Name = fmt.Sprintf("%s%d", baseName, i)
		ins = append(ins, b)
	}
	return ins
}

func findVerticalBlock(xStart int, xMax int, yStart int, grid [][]*wrapBlocks) *wrapBlocks {
	for i := xStart + 1; i < xMax; i++ {
		if grid[i][yStart] != nil {
			return grid[i][yStart]
		}
	}
	return nil
}

func findSumHalfSquare(xMax int, yMax int, xStart int, yStart int, grid [][]*wrapBlocks) *wrapBlocks {
	for i := xStart + 1; i < int(math.Max(float64(xMax), float64(xStart+1))); i++ {
		for j := yStart - 1; j < yStart+1; j++ {
			if i > xMax-1 || j > yMax-1 || i < 0 || j < 0 {
				continue
			}
			if grid[i][j] != nil && (grid[i][j].c.Type == "Sum") {
				return grid[i][j]
			}
		}
	}
	return nil
}

func mergedAll(xMax int, yMax int, grid [][]*wrapBlocks, def *wrapBlocks) {
	for i, l := range grid {
		for j, b := range l {
			if b == nil {
				continue
			}
			n := findVerticalBlock(i, xMax, j, grid)
			if n == nil {
				n = findSumHalfSquare(xMax, yMax, i, j, grid)
				if n == nil {
					if b != def {
						def.c.DependsOn = append(def.c.DependsOn, b.c.Name)
					}
					continue
				}
			}
			n.c.DependsOn = append(n.c.DependsOn, b.c.Name)
			if n.c.Type != "Sum" {
				n = findSumHalfSquare(xMax, yMax, i, j, grid)
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
	nObjs := n
	nI := 1 + int(float64(n)*0.2)
	nObjs -= nI
	nS := 1 + int(float64(n)*0.2)
	nObjs -= nS
	nB := nObjs
	yMax := nI
	xMax := n/yMax + 2
	out := generateNInputs(nI, xMax, "Inputs")
	out = generateNSums(nS, xMax, yMax, "Sums", out)
	out = generateNBlocks(nB, xMax, yMax, "Blocks", out)
	lastSum := &out[nI]
	grid := make([][]*wrapBlocks, xMax)
	for i := range grid {
		grid[i] = make([]*wrapBlocks, yMax)
	}
	for i, b := range out {
		grid[b.x][b.y] = &out[i]
	}
	mergedAll(xMax, yMax, grid, lastSum)

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
		test.That(t, b[0].GetSignalValueAt(0), test.ShouldEqual, 8.0)
		test.That(t, err, test.ShouldBeNil)
		b, err = cLoop.OutputAt(ctx, "B")
		test.That(t, b[0].GetSignalValueAt(0), test.ShouldEqual, -3.0)
		test.That(t, err, test.ShouldBeNil)
	}
	cLoop.Stop(ctx)
}
