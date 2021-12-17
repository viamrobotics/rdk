// Get the coordinates of a depth pixel in the depth map in the reference frame of the color image
package main

import (
	"flag"
	"fmt"
	"strconv"

	"go.viam.com/core/rimage/transform"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
)

var logger = golog.NewLogger("depth_to_color")

func main() {
	confPtr := flag.String("conf", "", "path to intrinsic/extrinsic JSON config")
	flag.Parse()
	if flag.NArg() != 3 {
		err := errors.Errorf("need 3 numbers for a depth map point. Have %d", flag.NArg())
		logger.Fatal(err)
	}
	x, err := strconv.ParseFloat(flag.Arg(0), 64)
	if err != nil {
		logger.Fatal(err)
	}
	y, err := strconv.ParseFloat(flag.Arg(1), 64)
	if err != nil {
		logger.Fatal(err)
	}
	z, err := strconv.ParseFloat(flag.Arg(2), 64)
	if err != nil {
		logger.Fatal(err)
	}
	fmt.Printf("depth: x: %.3f, y: %.3f, z:%.3f\n", x, y, z)

	// load the inputs from the config file
	params, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(*confPtr)
	if err != nil {
		err = errors.Errorf("path=%q: %w", *confPtr, err)
		logger.Fatal(err)
	}
	cx, cy, _ := params.DepthPixelToColorPixel(x, y, z)
	fmt.Printf("color: x: %.3f, y: %.3f\n", cx, cy)
}
