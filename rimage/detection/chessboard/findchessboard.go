package chessboard

import (
	"fmt"
	"image"
	"os"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/core/rimage"
	"go.viam.com/utils"
)

var (
	logger = golog.NewLogger("detect_chessboard")
)

// DetectionConfiguration stores the parameters necessary for chessboard detection in an image
type DetectionConfiguration struct {
	Saddle   SaddleConfiguration        `json:"saddle"`
	Contours ChessContoursConfiguration `json:"contours"`
	Greedy   ChessGreedyConfiguration   `json:"greedy"`
}

// getMinSaddleDistance returns the saddle point that minimizes the distance with r2.Point pt, as well as this minimum
// distance
func getMinSaddleDistance(saddlePoints []r2.Point, pt r2.Point) (r2.Point, float64) {
	bestDist := 100000.
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

func FindChessboard(img rimage.Image, cfg DetectionConfiguration) *ChessGrid {
	// convert to mat
	im := rimage.ConvertColorImageToLuminanceFloat(img)
	ih, iw := im.Dims()
	saddleMap, saddlePoints, err := GetSaddleMapPoints(im, &cfg.Saddle)
	if err != nil {
		logger.Error(err)
	}
	// contours
	//TODO(louise): fix canny contour detection. For now, loading contour map generated with openCV
	//cannyDetector := rimage.NewCannyDericheEdgeDetectorWithParameters(cfg.Contours.CannyHigh, cfg.Contours.CannyLow, false)
	//edgesGray, _ := cannyDetector.DetectEdges(img, 0.5)

	// open edges image
	f, err := os.Open("rimage/cmd/chessboard/edges.png")
	if err != nil {
		logger.Error(err)
	}
	defer utils.UncheckedErrorFunc(f.Close)

	edgesGray, _, err := image.Decode(f)
	if err != nil {
		logger.Error(err)
	}

	// convert to float mat for further operations
	edgesImg := rimage.ConvertImage(edgesGray)
	edgesMat := rimage.ConvertColorImageToLuminanceFloat(*edgesImg)
	fmt.Println("Edges : ")
	fmt.Println(mat.Max(edgesMat))
	// make image binary
	edges := BinarizeMat(edgesMat, 127)
	// extract contours
	contours, hierarchy := rimage.FindContours(edges)
	_ = rimage.DrawContours(edges, contours, "contours16.png")
	// Approximate contours with polygons
	contoursSimplified := rimage.SimplifyContours(contours)
	err = rimage.DrawContoursSimplified(edges, contoursSimplified, "contours_polygons.png")
	if err != nil {
		logger.Error(err)
	}
	// select only contours that correspond to convex quadrilateral
	prunedContours := PruneContours(contoursSimplified, hierarchy, saddleMap, cfg.Contours.WinSize)
	err = rimage.DrawContoursSimplified(edges, prunedContours, "pruned_contours.png")
	if err != nil {
		logger.Error(err)
	}
	PlotSaddleMap(saddlePoints, prunedContours, "polygonsWithSaddles.png", ih, iw)

	// greedy iterations to find the best homography
	//TODO(louise): fix homography iteration issue
	saddles := rimage.ConvertSliceImagePointToSliceVec(saddlePoints)
	grid, err := GreedyIterations(prunedContours, saddles, cfg.Greedy)
	if err != nil {
		logger.Error(err)
	}
	fmt.Println(grid.M)
	return grid
}
