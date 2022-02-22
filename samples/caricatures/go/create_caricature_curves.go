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

	"go.viam.com/rdk/rlog"
)

var (
	// plotter.
	p = hplot.New()

	// colors.
	red              = color.RGBA{255, 0, 0, 255}
	violet           = color.RGBA{127, 0, 255, 255}
	highlighterGreen = color.RGBA{0, 240, 0, 255}
	darkGreen        = color.RGBA{0, 127, 0, 255}
	blue             = color.RGBA{0, 0, 255, 255}

	// number of features mapped to facial feature.
	numEyesNosePoints        = 4
	numBrowsOrNostrilsPoints = 5
	numInnerMouthPoints      = 7
	numOuterMouthPoints      = 9
	numCurvaturePoints       = 17

	// name of plot.
	plotFileType = "jpeg"
)

// initialize a plot.
func plotInit(p *hplot.Plot) {
	p.X.Label.Text = "polynomial with n-degrees of freedom"
	p.Y.Label.Text = "y-data"
	p.X.Min = 0
	p.X.Max = 0
	p.Y.Min = 0
	p.Y.Max = 0
	p.Add(plotter.NewGrid())
}

// create a plot comprising all (xdata, ydata) data points.
func polyPlotCurveAndPointsToPlot(xdata, ydata []float64) *hplot.S2D {
	s := hplot.NewS2D(hplot.ZipXY(xdata, ydata))
	s.Color = color.RGBA{0, 0, 255, 255}
	return s
}

// create a plotter function representative of the polynomial best-fit
// graph of a given facial feature.
func resPolyFit(name string, csl int, xdata, ydata []float64) *plotter.Function {
	// create a slice of polynomial coefficients initialized with values
	// 1.0 (of type float64)
	ps := make([]float64, csl)
	for idx := 0; idx < csl; idx++ {
		switch {
		case name == "down_nose":
			ps[idx] = -10
		case csl == numEyesNosePoints || csl == numBrowsOrNostrilsPoints:
			ps[idx] = 0
		case csl == numInnerMouthPoints:
			ps[idx] = .000001
		case csl == numOuterMouthPoints:
			ps[idx] = .0001
		default:
			ps[idx] = .001
		}
	}

	// create a polynomial function with len(ps)-degrees of freedom, assuming
	// len(ps) facial landmark coordinates are provided for each facial feature
	poly := func(x float64, ps []float64) float64 {
		sum := 0.0
		degree := degreesOfFreedom(len(ps), name)
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

	f.Color = facialFeatureColor(csl)

	f.XMin, f.XMax = xMinXMax(xdata, name)

	f.Samples = numSamples(name)

	return f
}

// returns color of facial feature to be displayed in caricature.
func facialFeatureColor(csl int) color.RGBA {
	switch csl {
	case 4:
		return red
	case 5:
		return violet
	case 7:
		return highlighterGreen
	case 9:
		return darkGreen
	default:
		return blue
	}
}

// returns the XMin & XMax of each facial feature, providing bounds
// in the plot for polynomial representations of each feature.
func xMinXMax(xdata []float64, name string) (float64, float64) {
	sorted := sort.Float64Slice(xdata)
	switch name {
	case "bottom_left_eye", "bottom_right_eye":
		return sorted[len(xdata)-3], sorted[0]
	case "bottom_outer_lips":
		return sorted[len(xdata)-8], sorted[0]
	case "bottom_inner_lips":
		return sorted[len(xdata)-6], sorted[0]
	case "down_nose":
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
		return max, min
	default:
		return xdata[0], xdata[len(xdata)-1]
	}
}

// returns the number of points to be estimated between two
// x-coordinates as part of function interpolation.
func numSamples(name string) int {
	switch name {
	case "face_curvature":
		return 250
	case "left_brow", "right_brow":
		return 250
	case "down_nose":
		return 10
	case "across_nostrils":
		return 250
	case "top_left_eye", "top_right_eye", "bottom_left_eye", "bottom_right_eye":
		return 250
	case "top_outer_lips", "bottom_outer_lips":
		return 250
	case "top_inner_lips", "bottom_inner_lips":
		return 10
	default:
		return 0
	}
}

// returns degree of polynomial graphing a certain facial feature.
func degreesOfFreedom(lenPs int, name string) int {
	switch {
	case name == "down_nose":
		return 4
	case lenPs == numOuterMouthPoints || lenPs == numInnerMouthPoints:
		return lenPs / 2
	case lenPs == numCurvaturePoints:
		return 3
	default:
		return lenPs
	}
}

// plots polynomial curves, and if error arises, return that error.
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
		rlog.Logger.Info("\n", name)
		csl := len(face.Features[i].Points)
		xdata, ydata := facialFeaturePointsFromFace(face, i)
		s := polyPlotCurveAndPointsToPlot(xdata, ydata)
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
