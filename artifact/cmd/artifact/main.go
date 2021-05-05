// Package main provides the artifact CLI for importing and exporting artifacts.
package main

import (
	"fmt"
	"os"

	"go.viam.com/robotcore/artifact/tools"

	"github.com/edaniels/golog"
)

func main() {
	usage := "usage: artifact [clean|import|export]"
	if len(os.Args) <= 1 {
		fmt.Println(usage)
		os.Exit(0)
	}
	switch os.Args[1] {
	case "clean":
		if err := tools.Clean(); err != nil {
			golog.Global.Fatal(err)
		}
	case "import":
		if err := tools.Import(); err != nil {
			golog.Global.Fatal(err)
		}
	case "export":
		if err := tools.Export(); err != nil {
			golog.Global.Fatal(err)
		}
	default:
		fmt.Println(usage)
		os.Exit(1)
	}
}
