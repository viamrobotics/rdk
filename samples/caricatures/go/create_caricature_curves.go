// Copyright ©2017 The go-hep Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"math"
	"os"
	"sort"

	"github.com/disintegration/imaging"
	"go-hep.org/x/hep/fit"
	"go-hep.org/x/hep/hplot"
	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var (
	// plotter
	p = hplot.New()

	// colors
	red              = color.RGBA{255, 0, 0, 255}
	violet           = color.RGBA{127, 0, 255, 255}
	highlighterGreen = color.RGBA{0, 240, 0, 255}
	darkGreen        = color.RGBA{0, 127, 0, 255}
	blue             = color.RGBA{0, 0, 255, 255}

	// number of features mapped to facial feature
	numEyesNosePoints        = 4
	numBrowsOrNostrilsPoints = 5
	numInnerMouthPoints      = 7
	numOuterMouthPoints      = 9
	numCurvaturePoints       = 17

	// name of plot
	plotFileType = "jpeg"
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
func polyPlotCurveAndPointsToPlot(csl int, xdata, ydata []float64) *hplot.S2D {
	s := hplot.NewS2D(hplot.ZipXY(xdata, ydata))
	s.Color = color.RGBA{0, 0, 255, 255}
	return s
}

// create a plotter function representative of the polynomial best-fit
// graph of a given facial feature
func resPolyFit(name string, csl int, xdata, ydata []float64) *plotter.Function {

	// create a slice of polynomial coefficients initialized with values
	// 1.0 (of type float64)
	ps := make([]float64, csl)
	for idx := 0; idx < csl; idx++ {
		if name == "down_nose" {
			ps[idx] = -10
		} else if csl == numEyesNosePoints || csl == numBrowsOrNostrilsPoints {
			ps[idx] = 0
		} else if csl == numInnerMouthPoints {
			ps[idx] = .000001
		} else if csl == numOuterMouthPoints {
			ps[idx] = .0001
		} else {
			ps[idx] = .001
		}
	}

	// create a polynomial function with len(ps)-degrees of freedom, assuming
	// len(ps) facial landmark coordinates are provided for each facial feature
	poly := func(x float64, ps []float64) float64 {
		sum := 0.0
		degree := len(ps)
		if name == "down_nose" {
			degree = 4
		}
		if degree == numOuterMouthPoints {
			degree = degree / 2
		} else if degree == numCurvaturePoints {
			degree = 3
		}
		for i := 0; i < degree; i++ {
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
	if csl == 4 {
		f.Color = red
	} else if csl == 5 {
		f.Color = violet
	} else if csl == 7 {
		f.Color = highlighterGreen
	} else if csl == 9 {
		f.Color = darkGreen
	} else {
		f.Color = blue
	}
	f.Samples = 10
	if name == "bottom_left_eye" || name == "bottom_right_eye" {
		f.XMin = sort.Float64Slice(xdata)[len(xdata)-3]
		f.XMax = sort.Float64Slice(xdata)[0]
	} else if name == "bottom_outer_lips" {
		f.XMin = sort.Float64Slice(xdata)[len(xdata)-8]
		f.XMax = sort.Float64Slice(xdata)[0]
	} else if name == "bottom_inner_lips" {
		f.XMin = sort.Float64Slice(xdata)[len(xdata)-6]
		f.XMax = sort.Float64Slice(xdata)[0]
	} else if name == "down_nose" {
		max := 0.0
		min := 1000.0
		for i := 0; i < len(xdata); i++ {
			if max < xdata[i] {
				max = xdata[i]
			}
			if min > xdata[i] {
				min = xdata[i]
			}
		}
		f.XMax = max
		f.XMin = min

	} else {
		f.XMax = xdata[0]
		f.XMin = xdata[len(xdata)-1]
	}

	return f
}

// plots polynomial curves, and if error arises, return that error
func polyPlotAllCurves(person string) error {

	// initialize the plot
	plotInit(p)

	// parse json file to get face - this data struct holds information
	// needed for curve tracing
	jsonFilePath := "../json/selfie_" + person + ".json"
	face, err := parseJSON(jsonFilePath)
	if err != nil {
		return err
	}

	// add facial feature coordinates & corresponding curve traces to
	// the plot
	for i := 0; i < len(face.Features); i++ {
		name := face.Features[i].Name
		print("\n", name)
		csl := len(face.Features[i].Points)
		xdata, ydata := facialFeaturePointsFromFace(face, i)
		s := polyPlotCurveAndPointsToPlot(csl, xdata, ydata)
		p.Add(s)
		f := resPolyFit(name, csl, xdata, ydata)
		p.Add(f)
	}

	// save the plot (naturally created upside down) and flip it vertically
	plotFilePath := ("../facial_feature_graphs/poly_plot_" + person + "." + plotFileType)
	f, err := os.Create(plotFilePath)
	if err != nil {
		return err
	}
	if err := p.Save(17.5*vg.Centimeter, -1,
		plotFilePath); err != nil {
		log.Fatal("[!] error creating plot", err)
	}
	fileImg, _ := os.Open(plotFilePath)
	srcImg, _, _ := image.Decode(fileImg)
	flippedImg := imaging.FlipV(srcImg)
	rect := flippedImg.Bounds()
	newImg := flippedImg.SubImage(rect)
	jpeg.Encode(f, newImg, &jpeg.Options{Quality: jpeg.DefaultQuality})

	return nil
}
