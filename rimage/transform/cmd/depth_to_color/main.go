// Get the coordinates of a depth pixel in the depth map in the reference frame of the color image
// $./depth_to_color -conf=/path/to/intrinsics/extrinsic/file X Y Z
package main

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage/transform"
)

var logger = golog.NewLogger("depth_to_color")

func main() {
	confPtr := flag.String("conf", "", "path to intrinsic/extrinsic JSON config")
	flag.Parse()
	if flag.NArg() != 3 {
		err := errors.Errorf("need 3 numbers for a depth map point. Have %d", flag.NArg())
		// TODO(RSDK-548): remove fatal?
		logger.Fatal(err)
	}
	x, err := strconv.ParseFloat(flag.Arg(0), 64)
	if err != nil {
		// TODO(RSDK-548): remove fatal?
		logger.Fatal(err)
	}
	y, err := strconv.ParseFloat(flag.Arg(1), 64)
	if err != nil {
		// TODO(RSDK-548): remove fatal?
		logger.Fatal(err)
	}
	z, err := strconv.ParseFloat(flag.Arg(2), 64)
	if err != nil {
		// TODO(RSDK-548): remove fatal?
		logger.Fatal(err)
	}
	logger.Infof("depth: x: %.3f, y: %.3f, z:%.3f\n", x, y, z)

	// load the inputs from the config file
	params, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(*confPtr)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("path=%q", *confPtr))
		// TODO(RSDK-548): remove fatal?
		logger.Fatal(err)
	}
	cx, cy, _ := params.DepthPixelToColorPixel(x, y, z)
	logger.Infof("color: x: %.3f, y: %.3f\n", cx, cy)
}
