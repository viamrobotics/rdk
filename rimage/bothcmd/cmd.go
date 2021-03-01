package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/rimage"
)

func main() {
	err := realMain()
	if err != nil {
		golog.Global.Info(err)
		os.Exit(-1)
	}
}

func merge() error {
	if flag.NArg() < 4 {
		return fmt.Errorf("merge needs <color in> <depth in> <out>")
	}

	img, err := rimage.NewImageWithDepth(flag.Arg(1), flag.Arg(2))
	if err != nil {
		return err
	}

	return img.WriteTo(flag.Arg(3))
}

func realMain() error {
	flag.Parse()

	if flag.NArg() < 1 {
		return fmt.Errorf("need to specify a command")
	}

	cmd := flag.Arg(0)

	switch cmd {
	case "merge":
		return merge()
	default:
		return fmt.Errorf("unknown command: [%s]", cmd)
	}
}
