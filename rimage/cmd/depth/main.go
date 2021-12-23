// Package main is a command that takes a file and produces visual depth data.
package main

import (
	"flag"
	"fmt"
	"strings"

	"go.viam.com/rdk/rimage"
)

func main() {

	hardMin := flag.Int("min", 0, "min depth")
	hardMax := flag.Int("max", int(rimage.MaxDepth), "max depth")

	flag.Parse()

	if flag.NArg() < 2 {
		panic("need two args <in> <out>")
	}

	var dm *rimage.DepthMap
	var pc *rimage.ImageWithDepth
	var err error

	fn := flag.Arg(0)
	if strings.HasSuffix(fn, ".both.gz") {
		pc, err = rimage.ReadBothFromFile(fn, false) // just extracting depth data
		if pc != nil {
			dm = pc.Depth
		}
	} else {
		dm, err = rimage.ParseDepthMap(flag.Arg(0))
	}

	if err != nil {
		panic(err)
	}

	img := dm.ToPrettyPicture(rimage.Depth(*hardMin), rimage.Depth(*hardMax))
	if err := rimage.WriteImageToFile(flag.Arg(1), img); err != nil {
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
