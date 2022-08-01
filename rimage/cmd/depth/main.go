// Package main is a command that takes a file and produces visual depth data.
package main

import (
	"flag"
	"strings"

	"go.viam.com/rdk/rimage"
)

func main() {
	flag.Parse()

	if flag.NArg() < 2 {
		panic("need two args <in> <out>")
	}

	var dm *rimage.DepthMap
	var img *rimage.Image
	var err error

	if fn := flag.Arg(0); strings.HasSuffix(fn, ".both.gz") {
		img, dm, err = rimage.ReadBothFromFile(fn)
	} else {
		dm, err = rimage.ParseDepthMap(flag.Arg(0))
	}
	if err != nil {
		panic(err)
	}

	depth := dm.ToGray16Picture()
	if err := rimage.WriteImageToFile(flag.Arg(1)+".png", depth); err != nil {
		panic(err)
	}
	if img != nil {
		fn2 := flag.Arg(1) + "-color.png"
		err = img.WriteTo(fn2)
		if err != nil {
			panic(err)
		}
	}
}
