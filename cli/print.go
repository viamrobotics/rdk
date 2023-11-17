package cli

import (
	"fmt"
	"io"
	"log"
	"os"
	"unicode"
	"unicode/utf8"

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

// printf prints a message with no prefix.
func printf(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, format+"\n", a...)
}

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
//
// NOTE(benjirewis): we disable the unparam linter here. Our usages of warningf
// do not currently make use of the variadic `a` parameter but may in the
// future. unparam will complain until it does.
//
//nolint:unparam
func warningf(w io.Writer, format string, a ...interface{}) {
	if _, err := color.New(color.Bold, color.FgYellow).Fprint(w, "Warning: "); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(w, format+"\n", a...)
}

// Errorf prints a message prefixed with a bold red "Error: " prefix and exits with 1.
// It also capitalizes the first letter of the message.
func Errorf(w io.Writer, format string, a ...interface{}) {
	if _, err := color.New(color.Bold, color.FgRed).Fprint(w, "Error: "); err != nil {
		log.Fatal(err)
	}

	toPrint := fmt.Sprintf(format+"\n", a...)
	r, i := utf8.DecodeRuneInString(toPrint)
	if r == utf8.RuneError {
		log.Fatal("Malformed error message:", toPrint)
	}
	upperR := unicode.ToUpper(r)
	fmt.Fprintf(w, string(upperR)+toPrint[i:])

	os.Exit(1)
}

// viamLogo prints an ASCII Viam logo.
func viamLogo(w io.Writer) {
	if _, err := color.New(color.Bold, color.FgWhite).Fprint(w, asciiViam); err != nil {
		log.Fatal(err)
	}
}
