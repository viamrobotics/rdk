package cli

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/fatih/color"
)

// Infof prints a message prefixed with a bold cyan "Info: ".
func Infof(w io.Writer, format string, a ...interface{}) {
	// NOTE(benjirewis): for some reason, both errcheck and gosec complain about
	// Fprint's "unchecked error" here. Fatally log any errors write errors here
	// and below.
	if _, err := color.New(color.Bold, color.FgCyan).Fprint(w, "Info: "); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(w, format+"\n", a...)
}

// Warningf prints a message prefixed with a bold yellow "Warning: ".
func Warningf(w io.Writer, format string, a ...interface{}) {
	if _, err := color.New(color.Bold, color.FgYellow).Fprint(w, "Warning: "); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(w, format+"\n", a...)
}

// Errorf prints a message prefixed with a bold red "Error: " prefix and exits with 1.
func Errorf(w io.Writer, format string, a ...interface{}) {
	if _, err := color.New(color.Bold, color.FgRed).Fprint(w, "Error: "); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(w, format+"\n", a...)
	os.Exit(1)
}
