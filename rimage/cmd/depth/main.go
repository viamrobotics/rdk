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
	var err error

	if fn := flag.Arg(0); strings.HasSuffix(fn, ".both.gz") {
		_, dm, err = rimage.ReadBothFromFile(fn) // just extracting depth data
	} else {
		dm, err = rimage.ParseDepthMap(flag.Arg(0))
	}
	if err != nil {
		panic(err)
	}

	img := dm.ToGray16Picture()
	if err := rimage.WriteImageToFile(flag.Arg(1), img); err != nil {
		panic(err)
	}
}
