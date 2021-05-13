// Package main is a command providing various utilities to work with color and depth data.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/edaniels/golog"

	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"
)

var logger = golog.NewDevelopmentLogger("rimage_both")

func main() {
	err := realMain(os.Args[1:])
	if err != nil {
		logger.Fatal(err)
	}
}

func merge(flags *flag.FlagSet, aligned bool) error {
	if flags.NArg() < 4 {
		return errors.New("merge needs <color in> <depth in> [optional -aligned]")
	}

	img, err := rimage.NewImageWithDepth(flags.Arg(1), flags.Arg(2), aligned)
	if err != nil {
		return err
	}

	return img.WriteTo(flags.Arg(3))
}

func combineRGBAndZ16(flags *flag.FlagSet, aligned bool) error {
	if flags.NArg() < 4 {
		return errors.New("combineRGBAndZ16 needs <color png in> <grayscale png in> <out> [optional -aligned]")
	}

	img, err := rimage.NewImageWithDepthFromImages(flags.Arg(1), flags.Arg(2), aligned)
	if err != nil {
		return err
	}

	return img.WriteTo(flags.Arg(3))
}

func toLas(flags *flag.FlagSet, aligned bool) error {
	if flags.NArg() < 3 {
		return errors.New("to-las needs <both in> <aligner config> <las out> [optional -aligned]")
	}

	img, err := rimage.ReadBothFromFile(flags.Arg(1), aligned)
	if err != nil {
		return err
	}
	cameraMatrices, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(flags.Arg(2))
	if err != nil {
		return err
	}

	pc, err := cameraMatrices.ImageWithDepthToPointCloud(img)
	if err != nil {
		return err
	}

	return pc.WriteToFile(flags.Arg(3))
}

func realMain(args []string) error {
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	aligned := flags.Bool("aligned", false, "color and depth image are already aligned")
	err := flags.Parse(args)
	if err != nil {
		return err
	}

	if flags.NArg() < 1 {
		return errors.New("need to specify a command")
	}

	cmd := flags.Arg(0)

	switch cmd {
	case "merge":
		return merge(flags, *aligned)
	case "combineRGBAndZ16":
		return combineRGBAndZ16(flags, *aligned)
	case "to-las":
		return toLas(flags, *aligned)
	default:
		return fmt.Errorf("unknown command: [%s]", cmd)
	}
}
