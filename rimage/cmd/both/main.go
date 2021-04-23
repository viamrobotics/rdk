package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/rimage"
)

var logger = golog.NewDevelopmentLogger("rimage_both")

func main() {
	err := realMain(os.Args[1:])
	if err != nil {
		logger.Fatal(err)
	}
}

func merge(flags *flag.FlagSet) error {
	if flags.NArg() < 4 {
		return fmt.Errorf("merge needs <color in> <depth in> <out>")
	}

	img, err := rimage.NewImageWithDepth(flags.Arg(1), flags.Arg(2))
	if err != nil {
		return err
	}

	return img.WriteTo(flags.Arg(3))
}

func combineRGBAndZ16(flags *flag.FlagSet) error {
	if flags.NArg() < 4 {
		return fmt.Errorf("combineRGBAndZ16 needs <color png in> <grayscale png in> <out>")
	}

	img, err := rimage.NewImageWithDepthFromImages(flags.Arg(1), flags.Arg(2))
	if err != nil {
		return err
	}

	return img.WriteTo(flags.Arg(3))
}

func toLas(flags *flag.FlagSet) error {
	if flags.NArg() < 3 {
		return fmt.Errorf("to-las needs <both in> <las out>")
	}

	img, err := rimage.BothReadFromFile(flags.Arg(1))
	if err != nil {
		return err
	}

	pc, err := img.ToPointCloud()
	if err != nil {
		return err
	}

	return pc.WriteToFile(flags.Arg(2))
}

func realMain(args []string) error {
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	err := flags.Parse(args)
	if err != nil {
		return err
	}

	if flags.NArg() < 1 {
		return fmt.Errorf("need to specify a command")
	}

	cmd := flags.Arg(0)

	switch cmd {
	case "merge":
		return merge(flags)
	case "combineRGBAndZ16":
		return combineRGBAndZ16(flags)
	case "to-las":
		return toLas(flags)
	default:
		return fmt.Errorf("unknown command: [%s]", cmd)
	}
}
