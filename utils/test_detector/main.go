// package main: this is called by the utils test suite to confirm the Testing() test is false in prod.
package main

import (
	"os"
	"testing"
)

func main() {
	if testing.Testing() {
		os.Exit(1)
	}
}
