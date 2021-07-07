// Copyright ©2017 The go-hep Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"image/color"
	"log"
	"math"

	"go-hep.org/x/hep/fit"
	"go-hep.org/x/hep/hplot"
	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var (
	// plotter
	p = hplot.New()

	// polynomials used for each facial feature represented in map format

	// left brow
	// 1: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // right brow
	// 2: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // across nostrils
	// 3: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // top left eye
	// 4: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // bottom left eye
	// 5: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // top right eye
	// 6: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // bottom right eye
	// 7: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // bottom outer lips
	// 8: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // top outer lips
	// 9: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // bottom inner lips
	// 10: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },
	// // top inner lips
	// 11: func(x float64, ps []float64) float64 {
	// 	return ps[0]*x*x + ps[1]*x + ps[2]
	// },

)

func poly(i int) func(x float64, ps []float64) float64 {
	// face curvature
	sum := 0.0
	for i := 0.0; i < len(ps); i++ {
		sum += ps[i] * math.Pow(x, i)
	}
	return sum
}

func plotInit(p *hplot.Plot) {
	p.X.Label.Text = "f(x) = a*x*x + b*x + c"
	p.Y.Label.Text = "y-data"
	// p.X.Min = -50
	// p.X.Max = +50
	// p.Y.Min = 0
	// p.Y.Max = 220
	p.X.Min = 0
	p.X.Max = 0
	p.Y.Min = 0
	p.Y.Max = 0
}

func polyPlotCurveAndPointsToPlot(xdata, ydata []float64) *hplot.S2D {
	s := hplot.NewS2D(hplot.ZipXY(xdata, ydata))
	s.Color = color.RGBA{0, 0, 255, 255}
	return s
}

func resPolyFit(csl int, poly func(x float64, ps []float64) float64, xdata, ydata []float64) *plotter.Function {

	ps := make([]float64, csl)
	for idx := 0; idx < csl; idx++ {
		ps[csl] = 1.0
	}
	res, err := fit.Curve1D(
		fit.Func1D{
			F:  poly,
			X:  xdata,
			Y:  ydata,
			Ps: ps,
		},
		nil, &optimize.NelderMead{},
	)

	if err != nil {
		log.Fatal(err)
	}

	if err := res.Status.Err(); err != nil {
		log.Fatal(err)
	}

	f := plotter.NewFunction(func(x float64) float64 {
		return poly(x, res.X)
	})
	f.Color = color.RGBA{255, 0, 0, 255}
	f.Samples = 1000

	return f
}

func polyPlotAllCurves() error {

	{
		plotInit(p)

		face, err := parseJSON(JSON_PATH)
		if err != nil {
			return err
		}

		for i := 0; i < len(face.Features); i++ {
			// for i := 1; i < 2; i++ {
			xdata, ydata := facialFeaturePointsFromFace(face, i)
			p.Add(polyPlotCurveAndPointsToPlot(xdata, ydata))
			f := resPolyFit(i, poly[i], xdata, ydata)
			p.Add(f)
		}

		p.Add(plotter.NewGrid())

		err = p.Save(20*vg.Centimeter, -1, "../facial_feature_graphs/poly-plot.png")
		if err != nil {
			log.Fatal(err)
			return err
		}
		return nil
	}
}
