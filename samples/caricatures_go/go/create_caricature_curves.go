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
)

// initialize a plot
func plotInit(p *hplot.Plot) {
	p.X.Label.Text = "polynomial with n-degrees of freedom"
	p.Y.Label.Text = "y-data"
	p.X.Min = 0
	p.X.Max = 0
	p.Y.Min = 0
	p.Y.Max = 0
	p.Add(plotter.NewGrid())
}

// create a plot comprising all (xdata, ydata) data points
func polyPlotCurveAndPointsToPlot(xdata, ydata []float64) *hplot.S2D {
	s := hplot.NewS2D(hplot.ZipXY(xdata, ydata))
	s.Color = color.RGBA{0, 0, 255, 255}
	return s
}

// create a plotter function representative of the polynomial best-fit
// graph of a given facial feature
func resPolyFit(csl int, xdata, ydata []float64) *plotter.Function {

	// create a slice of polynomial coefficients initialized with values
	// 1.0 (of type float64)
	ps := make([]float64, csl)
	for idx := 0; idx < csl; idx++ {
		ps[idx] = 1.0
	}

	// create a polynomial function with len(ps)-degrees of freedom, assuming
	// len(ps) facial landmark coordinates are provided for each facial feature
	poly := func(x float64, ps []float64) float64 {
		sum := 0.0
		for i := 0; i < len(ps); i++ {
			sum += ps[i] * math.Pow(x, float64(i))
		}
		return sum
	}

	// create a polynomial best-fit graph representing the xdata & ydata
	// with coefficients defined in ps & optimize the graph using the
	// NelderMead optimization
	res, err := fit.Curve1D(
		fit.Func1D{
			F:  poly,
			X:  xdata,
			Y:  ydata,
			Ps: ps,
		},
		nil, &optimize.NelderMead{},
	)

	// if there are any errors, log them and exit the program
	if err != nil {
		log.Fatal(err)
	}
	if err := res.Status.Err(); err != nil {
		log.Fatal(err)
	}

	// create the best fit-graph from the result of the best-fit
	// polynomial graph that has been caculated
	f := plotter.NewFunction(func(x float64) float64 {
		return poly(x, res.X)
	})
	f.Color = color.RGBA{255, 0, 0, 255}
	f.Samples = 1000
	f.XMin = xdata[0]
	f.XMax = xdata[len(xdata)-1]
	return f
}

// plots polynomial curves, and if error arises, return that error
func polyPlotAllCurves() error {

	// initialize the plot
	plotInit(p)

	// parse json file to get face - this data struct holds information
	// needed for curve tracing
	face, err := parseJSON(jsonPath)
	if err != nil {
		return err
	}

	// add facial feature coordinates & corresponding curve traces to
	// the plot
	for i := len(face.Features) - 1; i > 0; i-- {
		xdata, ydata := facialFeaturePointsFromFace(face, i)
		p.Add(polyPlotCurveAndPointsToPlot(xdata, ydata))
		f := resPolyFit(i, xdata, ydata)
		p.Add(f)
	}

	// write the plot to local machine memory
	err = p.Save(20*vg.Centimeter, -1, "../facial_feature_graphs/poly-plot.png")
	if err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}
