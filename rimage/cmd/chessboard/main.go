package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"os"

	"github.com/edaniels/golog"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/utils"

	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/detection/chessboard"
)

var (
	path   = flag.String("path", "rimage/cmd/chessboard/image_2021-07-16-16-10-41.png", "path of image to detect chessboard")
	conf   = flag.String("conf", "rimage/cmd/chessboard/conf.json", "path of configuration for chessboard detection algorithm")
	logger = golog.NewLogger("detect_chessboard")
)

func main() {
	flag.Parse()
	// load config
	file, err := os.Open(*conf)
	if err != nil {
		logger.Error("could not open configuration file")
	}
	defer utils.UncheckedErrorFunc(file.Close)

	// load configuration
	cfg := chessboard.DetectionConfiguration{}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		logger.Error(err)
	}

	img, err := rimage.NewImageFromFile(*path)
	if err != nil {
		logger.Error(err)
	}
	imgCopy, err := rimage.NewImageFromFile(*path)
	if err != nil {
		logger.Error(err)
	}

	logger.Info(img.Bounds().Max)
	// convert to mat
	im := rimage.ConvertColorImageToLuminanceFloat(*imgCopy)
	ih, iw := im.Dims()
	saddleMap, saddlePoints, err := chessboard.GetSaddleMapPoints(im, &cfg.Saddle)
	if err != nil {
		logger.Error(err)
	}
	// contours
	//TODO(louise): fix canny contour detection. For now, loading contour map generated with openCV
	//cannyDetector := rimage.NewCannyDericheEdgeDetectorWithParameters(cfg.Contours.CannyHigh, cfg.Contours.CannyLow, false)
	//edgesGray, _ := cannyDetector.DetectEdges(img, 0.5)

	// open edges image
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
	edges := chessboard.BinarizeMat(edgesMat, 127)
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
	prunedContours := chessboard.PruneContours(contoursSimplified, hierarchy, saddleMap, cfg.Contours.WinSize)
	err = rimage.DrawContoursSimplified(edges, prunedContours, "pruned_contours.png")
	if err != nil {
		logger.Error(err)
	}
	chessboard.PlotSaddleMap(saddlePoints, prunedContours, "polygonsWithSaddles.png", ih, iw)

	// greedy iterations to find the best homography
	//TODO(louise): fix homography iteration issue
	saddles := rimage.ConvertSliceImagePointToSliceVec(saddlePoints)
	grid, err := chessboard.GreedyIterations(prunedContours, saddles, cfg.Greedy)
	if err != nil {
		logger.Error(err)
	}
	fmt.Println(grid.M)
	fmt.Println(len(saddlePoints))
	fmt.Println(saddleMap.Dims())
	fmt.Println(mat.Max(saddleMap))
	// contours
	cannyDetector := rimage.NewCannyDericheEdgeDetectorWithParameters(cfg.Contours.CannyHigh, cfg.Contours.CannyLow,true)
	edgesGray, _ := cannyDetector.DetectEdges(img, 0.5)
