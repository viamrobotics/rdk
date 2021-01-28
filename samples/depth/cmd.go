package main

import (
	"flag"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
)

func main() {

	hardMin := flag.Int("min", 0, "min depth")
	hardMax := flag.Int("max", 10000, "max depth")

	flag.Parse()

	if flag.NArg() < 2 {
		panic("need two args <in> <out>")
	}

	dm, err := vision.ParseDepthMap(flag.Arg(0))
	if err != nil {
		panic(err)
	}

	img, err := dm.ToPrettyPicture(*hardMin, *hardMax)
	if err != nil {
		panic(err)
	}
	defer img.Close()

	gocv.IMWrite(flag.Arg(1), img.MatUnsafe())

}
