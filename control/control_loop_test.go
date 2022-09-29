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

	"go.viam.com/rdk/config"
)

type wrapBlocks struct {
	c BlockConfig
	x int
	y int
}

func generateNInputs(n int, baseName string) []wrapBlocks {
	out := make([]wrapBlocks, n)

	out[0].c = BlockConfig{
		Name: "",
		Type: "endpoint",
		Attribute: config.AttributeMap{
			"motor_name": "MotorFake",
		},
		DependsOn: []string{},
	}
	out[0].x = 0
	out[0].y = 0
	out[0].c.Name = fmt.Sprintf("%s%d", baseName, 0)
	for i := 1; i < n; i++ {
		out[i].c = BlockConfig{
			Name: "S1",
			Type: "constant",
			Attribute: config.AttributeMap{
				"constant_val": 3.0,
			},
			DependsOn: []string{},
		}
		out[i].x = 0
		out[i].y = i
		out[i].c.Name = fmt.Sprintf("%s%d", baseName, i)
	}
	return out
}

func generateNSums(n, xMax, yMax int, baseName string, ins []wrapBlocks) []wrapBlocks {
	b := wrapBlocks{
		c: BlockConfig{
			Name: "",
			Type: "sum",
			Attribute: config.AttributeMap{
				"sum_string": "",
			},
			DependsOn: []string{},
		},
		x: xMax - 1,
		y: (yMax - 1) / 2,
	}
	b.c.Name = fmt.Sprintf("%s%d", baseName, 0)
	ins = append(ins, b)
	for i := 1; i < n; i++ {
		var xR int
		var yR int
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
			c: BlockConfig{
				Name: "",
				Type: "sum",
				Attribute: config.AttributeMap{
					"sum_string": "",
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

func generateNBlocks(n, xMax, yMax int, baseName string, ins []wrapBlocks) []wrapBlocks {
	for i := 0; i < n; i++ {
		var xR int
		var yR int
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
			c: BlockConfig{
				Name: "C",
				Type: "gain",
				Attribute: config.AttributeMap{
					"gain": -2.0,
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

func findVerticalBlock(xStart, xMax, yStart int, grid [][]*wrapBlocks) *wrapBlocks {
	for i := xStart + 1; i < xMax; i++ {
		if grid[i][yStart] != nil {
			return grid[i][yStart]
		}
	}
	return nil
}

func findSumHalfSquare(xMax, yMax, xStart, yStart int, grid [][]*wrapBlocks) *wrapBlocks {
	for i := xStart + 1; i < int(math.Max(float64(xMax), float64(xStart+1))); i++ {
		for j := yStart - 1; j < yStart+1; j++ {
			if i > xMax-1 || j > yMax-1 || i < 0 || j < 0 {
				continue
			}
			if grid[i][j] != nil && (grid[i][j].c.Type == "sum") {
				return grid[i][j]
			}
		}
	}
	return nil
}

func mergedAll(xMax, yMax int, grid [][]*wrapBlocks, def *wrapBlocks) {
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
			if n.c.Type != "sum" {
				n = findSumHalfSquare(xMax, yMax, i, j, grid)
				if n != nil {
					n.c.DependsOn = append(n.c.DependsOn, b.c.Name)
				}
			}
		}
	}
}

func benchNBlocks(b *testing.B, n int, freq float64) {
	b.Helper()
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
	out := generateNInputs(nI, "Inputs")
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

	cfg := Config{
		Frequency: freq,
		Blocks:    []BlockConfig{},
	}
	for i := range out {
		if out[i].c.Type == "sum" {
			out[i].c.Attribute["sum_string"] = strings.Repeat("+", len(out[i].c.DependsOn))
		}
		cfg.Blocks = append(cfg.Blocks, out[i].c)
	}
	logger := golog.NewLogger("Bench")
	cloop, err := createLoop(logger, cfg, nil)
	if err == nil {
		b.ResetTimer()
		cloop.startBenchmark(b.N)
		cloop.activeBackgroundWorkers.Wait()
	}
}

func BenchmarkLoop10(b *testing.B) {
	benchNBlocks(b, 10, 100.0)
}

func BenchmarkLoop30(b *testing.B) {
	benchNBlocks(b, 30, 100.0)
}

func BenchmarkLoop100(b *testing.B) {
	benchNBlocks(b, 100, 100.0)
}

func TestControlLoop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cfg := Config{
		Blocks: []BlockConfig{
			{
				Name: "A",
				Type: "endpoint",
				Attribute: config.AttributeMap{
					"motor_name": "MotorFake",
				},
				DependsOn: []string{"E"},
			},
			{
				Name: "B",
				Type: "sum",
				Attribute: config.AttributeMap{
					"sum_string": "+-",
				},
				DependsOn: []string{"A", "S1"},
			},
			{
				Name: "S1",
				Type: "constant",
				Attribute: config.AttributeMap{
					"constant_val": 3.0,
				},
				DependsOn: []string{},
			},
			{
				Name: "C",
				Type: "gain",
				Attribute: config.AttributeMap{
					"gain": -2.0,
				},
				DependsOn: []string{"B"},
			},
			{
				Name: "D",
				Type: "sum",
				Attribute: config.AttributeMap{
					"sum_string": "+-",
				},
				DependsOn: []string{"C", "S2"},
			},
			{
				Name: "S2",
				Type: "constant",
				Attribute: config.AttributeMap{
					"constant_val": 10.0,
				},
				DependsOn: []string{},
			},
			{
				Name: "E",
				Type: "gain",
				Attribute: config.AttributeMap{
					"gain": -2.0,
				},
				DependsOn: []string{"D"},
			},
		},
		Frequency: 20.0,
	}
	cLoop, err := createLoop(logger, cfg, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cLoop, test.ShouldNotBeNil)
	cLoop.Start()
	for i := 200; i > 0; i-- {
		time.Sleep(65 * time.Millisecond)
		b, err := cLoop.OutputAt(ctx, "E")
		test.That(t, b[0].GetSignalValueAt(0), test.ShouldEqual, 8.0)
		test.That(t, err, test.ShouldBeNil)
		b, err = cLoop.OutputAt(ctx, "B")
		test.That(t, b[0].GetSignalValueAt(0), test.ShouldEqual, -3.0)
		test.That(t, err, test.ShouldBeNil)
	}
	cLoop.Stop()
}
