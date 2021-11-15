package main

import (
	"encoding/json"
	"flag"
	"os"

	"github.com/edaniels/golog"
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
	grid := chessboard.FindChessboard(*img, cfg, false)
	logger.Info(grid.M)

}
