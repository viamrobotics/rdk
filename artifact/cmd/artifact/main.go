// Package main provides the artifact CLI for importing and exporting artifacts.
package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"

	"go.viam.com/core/artifact/tools"
	"go.viam.com/core/rlog"
	"go.viam.com/core/utils"
)

func main() {
	usage := "usage: artifact [clean|pull|push|status]"
	if len(os.Args) <= 1 {
		fmt.Println(usage)
		os.Exit(0)
	}
	switch os.Args[1] {
	case "clean":
		if err := tools.Clean(); err != nil {
			rlog.Logger.Fatal(err)
		}
	case "pull":
		if err := tools.Pull(); err != nil {
			rlog.Logger.Fatal(err)
		}
	case "push":
		if err := tools.Push(); err != nil {
			rlog.Logger.Fatal(err)
		}
	case "status":
		status, err := tools.Status()
		if err != nil {
			utils.PrintStackErr(err)
			rlog.Logger.Fatal(err)
		}
		if len(status.Modified) != 0 {
			fmt.Println("Modified:")
			for _, name := range status.Modified {
				fmt.Print("\t")
				color.Yellow(name)
			}
		}
		if len(status.Unstored) != 0 {
			fmt.Println("Unstored:")
			for _, name := range status.Unstored {
				fmt.Print("\t")
				color.Red(name)
			}
		}
	default:
		fmt.Println(usage)
		os.Exit(1)
	}
}
