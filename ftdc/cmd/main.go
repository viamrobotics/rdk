// main provides a CLI tool for viewing `.ftdc` files emitted by the `viam-server`.
package main

import (
	"os"

	"go.viam.com/rdk/ftdc/parser"
)

func main() {
	if len(os.Args) < 2 {
		parser.NolintPrintln("Expected an FTDC filename. E.g: go run parser.go <path-to>/viam-server.ftdc")
		return
	}

	parser.LaunchREPL(os.Args[1])
}
