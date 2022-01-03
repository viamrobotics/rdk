package main

import (
	"encoding/json"
	"flag"
	"image"
	"os"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision/detection/chessboard"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
)

var (
	logger = golog.NewLogger("detect_chessboard")
)

// getEdgesGrayImageFromPath - helper function to load the edges image in 8 bit Gray image format if path is defined
// else returns nil
// the pointer to this edge image will be passed to the FindChessboard function
func getEdgesGrayImageFromPath(pEdges *string, img *rimage.Image) *image.Image {
	//var edgesImg *image.Image
	if pEdges != nil {
		f, err := os.Open(*pEdges)
		defer utils.UncheckedErrorFunc(f.Close)
		if err != nil {
			logger.Info("Edges image does not exist at this location; will run Canny contour detection.")
			return nil
		}
		edgesImg, _, err := image.Decode(f)
		if err != nil {
			logger.Error(err)
		}
		return &edgesImg

	}
	return nil
}

func main() {
	var pathImg = flag.String("path", "", "path of image to detect chessboard")
	var pathEdges = flag.String("pathEdges", "", "path of edges image in which to detect chessboard")
	var conf = flag.String("conf", "", "path of configuration for chessboard detection algorithm")
	flag.Parse()
	os.Exit(RunChessBoardDetection(*pathImg, *pathEdges, *conf))
}

// RunChessBoardDetection is the global function to be called by main
func RunChessBoardDetection(pthImg, pthEdges, pthConf string) int {

	// load config
	if pthConf == "" || len(pthConf) == 0 {
		pthConf = "rimage/cmd/chessboard/conf.json"
	}

	file, err := os.Open(pthConf)
	if err != nil {
		logger.Fatal("could not open configuration file")
		return -1
	}
	defer utils.UncheckedErrorFunc(file.Close)

	// load configuration
	cfg := chessboard.DetectionConfiguration{}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		logger.Fatal(err)
		return -1
	}

	// Load image
	img, err := rimage.NewImageFromFile(pthImg)
	if err != nil {
		logger.Fatal("could not open image file")
		return -1
	}

	// Load edges image if pathEdges is not nil
	edgesGray := getEdgesGrayImageFromPath(&pthEdges, img)

	grid, err := chessboard.FindChessboard(*img, edgesGray, cfg, false)
	if err != nil {
		logger.Fatal(err)
		return -1
	}
	logger.Info(grid.M)
	return 0
}
