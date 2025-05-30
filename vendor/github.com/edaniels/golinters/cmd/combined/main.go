package main

import (
	"github.com/edaniels/golinters/deferfor"
	"github.com/edaniels/golinters/errresp"
	"github.com/edaniels/golinters/fatal"
	"github.com/edaniels/golinters/mustcheck"
	"github.com/edaniels/golinters/printf"
	"github.com/edaniels/golinters/println"
	"github.com/edaniels/golinters/uselessf"

	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	unitchecker.Main(
		println.Analyzer,
		printf.Analyzer,
		mustcheck.Analyzer,
		deferfor.Analyzer,
		errresp.Analyzer,
		fatal.Analyzer,
		uselessf.Analyzer,
	)
}
