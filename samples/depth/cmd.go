package main

import (
	"flag"
	"fmt"
	"strings"

	"go.viam.com/robotcore/vision"
)

func main() {

	hardMin := flag.Int("min", 0, "min depth")
	hardMax := flag.Int("max", 10000, "max depth")

	flag.Parse()

	if flag.NArg() < 2 {
		panic("need two args <in> <out>")
	}

	var dm *vision.DepthMap
	var pc *vision.PointCloud
	var err error

	fn := flag.Arg(0)
	if strings.HasSuffix(fn, ".both.gz") {
		pc, err = vision.NewPointCloudFromBoth(fn)
		if pc != nil {
			dm = pc.Depth
		}
	} else {
		dm, err = vision.ParseDepthMap(flag.Arg(0))
	}

	if err != nil {
		panic(err)
	}

	img, err := dm.ToPrettyPicture(*hardMin, *hardMax)
	if err != nil {
		panic(err)
	}

	if err := img.WriteTo(flag.Arg(1)); err != nil {
		panic(err)
	}

	if pc != nil {
		fn2 := flag.Arg(1) + "-color.png"
		fmt.Println(fn2)
		err = pc.Color.WriteTo(fn2)
		if err != nil {
			panic(err)
		}

	}
}
