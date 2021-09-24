package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/edaniels/golog"
	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/detection/chessboard"
	"gonum.org/v1/gonum/mat"
)

var (
	path   = flag.String("path", "rimage/cmd/chessboard/image_2021-07-16-15-52-35.png", "path of image to detect chessboard")
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
	defer file.Close()
	// load configuration
	cfg := chessboard.ChessboardDetectionConfiguration{}

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
	saddleMap, saddlePoints, err := chessboard.GetSaddleMapPoints(im, &cfg.Saddle)
	if err != nil {
		logger.Error(err)
	}
	saddles := rimage.ConvertSliceImagePointToSliceVec(saddlePoints)
	fmt.Println(len(saddlePoints))
	fmt.Println(saddleMap.Dims())
	fmt.Println(mat.Max(saddleMap))
	// contours
	cannyDetector := rimage.NewCannyDericheEdgeDetectorWithParameters(cfg.Contours.CannyHigh, cfg.Contours.CannyLow, true)
	edgesGray, _ := cannyDetector.DetectEdges(img, 0.5)
	// convert to float mat for further operations
	edgesImg := rimage.ConvertImage(edgesGray)
	edgesMat := rimage.ConvertColorImageToLuminanceFloat(*edgesImg)
	// make image binary
	edges := chessboard.BinarizeMat(edgesMat, 127.0)
	fmt.Println(mat.Max(edges))
	// extract contours
	edgesMorpho, err := rimage.MorphoGradientCross(edges)
	if err != nil {
		logger.Error(err)
	}
	contours, hierarchy := rimage.FindContoursSuzuki(edgesMorpho)
	contoursSimplified := rimage.SimplifyContours(contours)
	fmt.Println(len(contoursSimplified))
	fmt.Println(len(hierarchy))
	prunedContours := chessboard.PruneContours(contoursSimplified, hierarchy, saddleMap, cfg.Contours.WinSize)
	fmt.Println(len(prunedContours))
	// greedy iterations
	grid,err := chessboard.GreedyIterations(prunedContours, saddles, cfg.Greedy)
	if err != nil {
		logger.Error(err)
	}
	fmt.Println(grid.M)
}
