// Package main provides the artifact CLI for importing and exporting artifacts.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"

	"go.viam.com/core/artifact/tools"
	"go.viam.com/core/rlog"
	"go.viam.com/core/utils"
)

func main() {
	usage := "usage: artifact [clean|pull|push|rm|status]"
	if len(os.Args) <= 1 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	switch os.Args[1] {
	case "clean":
		if err := tools.Clean(); err != nil {
			rlog.Logger.Fatal(err)
		}
	case "pull":
		var all bool
		if len(os.Args) == 3 && os.Args[2] == "--all" {
			all = true
		}
		if err := tools.Pull(all); err != nil {
			rlog.Logger.Fatal(err)
		}
	case "push":
		if err := tools.Push(); err != nil {
			rlog.Logger.Fatal(err)
		}
	case "rm":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: artifact rm <path>")
			os.Exit(1)
		}
		filePath := os.Args[2]
		if !filepath.IsAbs(filePath) {
			wd, err := os.Getwd()
			if err != nil {
				rlog.Logger.Fatal(err)
			}
			absPath, err := filepath.Abs(wd)
			if err != nil {
				rlog.Logger.Fatal(err)
			}
			filePath = filepath.Join(absPath, filePath)
		}

		if err := tools.Remove(filePath); err != nil {
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
