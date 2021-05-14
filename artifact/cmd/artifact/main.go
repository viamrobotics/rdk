// Package main provides the artifact CLI for importing and exporting artifacts.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/edaniels/golog"
	"github.com/fatih/color"
	"github.com/go-errors/errors"

	"go.viam.com/core/artifact/tools"
	"go.viam.com/core/rlog"
	"go.viam.com/core/utils"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var logger = golog.NewDevelopmentLogger("artifact")

type topArguments struct {
	Command string   `flag:"0,required,usage=<clean|pull|push|rm|status>"`
	Extra   []string `flag:",extra"` // for sub-commands
}

type pullArguments struct {
	All bool `flag:"all,usage=pull all files regardless of size"`
}

type removeArguments struct {
	Path string `flag:"0,required,usage=rm <path>"`
}

const (
	commandNameClean  = "clean"
	commandNamePull   = "pull"
	commandNamePush   = "push"
	commandNameRemove = "rm"
	commandNameStatus = "status"
)

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	var topArgsParsed topArguments
	if err := utils.ParseFlags(args, &topArgsParsed); err != nil {
		return err
	}
	switch topArgsParsed.Command {
	case commandNameClean:
		if err := tools.Clean(); err != nil {
			rlog.Logger.Fatal(err)
		}
	case commandNamePull:
		var pullArgsParsed pullArguments
		if err := utils.ParseFlags(append(args[:1], args[2:]...), &pullArgsParsed); err != nil {
			return err
		}
		if err := tools.Pull(pullArgsParsed.All); err != nil {
			rlog.Logger.Fatal(err)
		}
	case commandNamePush:
		if err := tools.Push(); err != nil {
			rlog.Logger.Fatal(err)
		}
	case commandNameRemove:
		var removeArgsParsed removeArguments
		if err := utils.ParseFlags(append(args[:1], args[2:]...), &removeArgsParsed); err != nil {
			return err
		}
		filePath := removeArgsParsed.Path
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
	case commandNameStatus:
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
		return errors.New("usage: artifact <clean|pull|push|rm|status>")
	}
	return nil
}
