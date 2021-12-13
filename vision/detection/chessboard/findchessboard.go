package chessboard

import (
	"image"
	"image/draw"
	"math"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"

	"go.viam.com/core/rimage"
)

var (
	logger = golog.NewLogger("detect_chessboard")
)

// OutputsConfiguration stores the parameters needed for contour precessing in chessboard detection
type OutputsConfiguration struct {
	OutputFolder string `json:"out-folder"` // low threshold for Canny contours detection
	BaseName     string `json:"base-name"`  // high threshold for Canny contours detection
}

// DetectionConfiguration stores the parameters necessary for chessboard detection in an image
type DetectionConfiguration struct {
	Saddle   SaddleConfiguration        `json:"saddle"`
	Contours ChessContoursConfiguration `json:"contours"`
	Greedy   ChessGreedyConfiguration   `json:"greedy"`
	Output   OutputsConfiguration       `json:"output"`
}

// getMinSaddleDistance returns the saddle point that minimizes the distance with r2.Point pt, as well as this minimum
// distance
func getMinSaddleDistance(saddlePoints []r2.Point, pt r2.Point) (r2.Point, float64) {
	bestDist := math.Inf(1)
	bestPt := pt
	for _, saddlePt := range saddlePoints {
		diff := pt.Sub(saddlePt)
		dist := diff.Norm()
		if dist < bestDist {
			bestDist = dist
			bestPt = saddlePt

		}
	}
	return bestPt, bestDist
}

// FindChessboard is the function that finds the chessboard given a rimage.Image
func FindChessboard(img rimage.Image, edgesGray *image.Image, cfg DetectionConfiguration, plot bool) (*ChessGrid, error) {
	cfgOut := cfg.Output
	// convert to mat
	im := rimage.ConvertColorImageToLuminanceFloat(img)
	ih, iw := im.Dims()
	saddleMap, saddlePoints, err := GetSaddleMapPoints(im, &cfg.Saddle)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	// contours; if edges image is nil, perform Canny contour detection, otherwise use loaded image
	b := img.Bounds()
	edgesImgGray := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))

	if edgesGray == nil {
		//TODO(louise): fix canny contour detection.
		cannyDetector := rimage.NewCannyDericheEdgeDetectorWithParameters(cfg.Contours.CannyHigh, cfg.Contours.CannyLow, false)
		edgesImgGray, err = cannyDetector.DetectEdges(&img, 0.5)

		if err != nil {
			logger.Error(err)
			return nil, err
		}
	} else {
		draw.Draw(edgesImgGray, edgesImgGray.Bounds(), *edgesGray, b.Min, draw.Src)
	}

	// convert to float mat for further operations
	edgesImg := rimage.ConvertImage(edgesImgGray)
	edgesMat := rimage.ConvertColorImageToLuminanceFloat(*edgesImg)
	// make image binary
	edges := BinarizeMat(edgesMat, 127)
	// extract contours
	contours, hierarchy := rimage.FindContours(edges)
	if plot {
		outPath := cfgOut.OutputFolder + cfgOut.BaseName + "contours16.png"
		err = rimage.DrawContours(edges, contours, outPath)
		if err != nil {
			logger.Error(err)
		}
	}

	// Approximate contours with polygons
	contoursSimplified := rimage.SimplifyContours(contours)
	if plot {
		outPath := cfgOut.OutputFolder + cfgOut.BaseName + "contours_polygons.png"
		err = rimage.DrawContoursSimplified(edges, contoursSimplified, outPath)
		if err != nil {
			logger.Error(err)
		}
	}

	// select only contours that correspond to convex quadrilateral
	prunedContours := PruneContours(contoursSimplified, hierarchy, saddleMap, cfg.Contours.WinSize, cfg.Contours.MinContourArea, cfg.Contours.MinSidePolygon)
	if plot {
		outPath := cfgOut.OutputFolder + cfgOut.BaseName + "pruned_contours.png"
		err = rimage.DrawContoursSimplified(edges, prunedContours, outPath)
		if err != nil {
			logger.Error(err)
		}
		outPath = cfgOut.OutputFolder + cfgOut.BaseName + "polygonsWithSaddles.png"
		err = PlotSaddleMap(saddlePoints, prunedContours, outPath, ih, iw)
		if err != nil {
			logger.Error(err)
		}
	}

	// greedy iterations to find the best homography
	saddles := rimage.ConvertContourIntToContourFloat(saddlePoints)
	grid, err := GreedyIterations(prunedContours, saddles, cfg.Greedy)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	m, err := GenerateNewBestFit(grid.IdealGrid, grid.Grid, grid.GoodPoints)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	grid.M = m
	return grid, nil
}
