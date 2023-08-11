package cli

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/fatih/color"
)

const asciiViam = `
@@BO..    "%@@B^%@@<      .}j.      !B@B$v'.    'nB$$$!
.*@$%l   f$$@X  %$$+      &@$$      l$$$$@@^   "B$$$$@!
  (@$$0'M@$@:   B$$~    ~B$$@$@1    !$$BQ@@@q.0@$$0@$$!
   'WB$$B@p     B$$~   qB$%-!@@@o.  l$@B..B@$@$@%; @$@!
    .u$$$!      B$$~ ;@@@& ... oB$@~!!$$@. z$$$z'. $$$!
      :h'       B$$~L%@$||     -@@$bi$@B    'M'    $$$!

`

// infof prints a message prefixed with a bold cyan "Info: ".
func infof(w io.Writer, format string, a ...interface{}) {
	// NOTE(benjirewis): for some reason, both errcheck and gosec complain about
	// Fprint's "unchecked error" here. Fatally log any errors write errors here
	// and below.
	if _, err := color.New(color.Bold, color.FgCyan).Fprint(w, "Info: "); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(w, format+"\n", a...)
}

// warningf prints a message prefixed with a bold yellow "Warning: ".
func warningf(w io.Writer, format string, a ...interface{}) {
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

// viamLogo prints an ASCII Viam logo.
func viamLogo(w io.Writer) {
	if _, err := color.New(color.Bold, color.FgWhite).Fprint(w, asciiViam); err != nil {
		log.Fatal(err)
	}
}
