// package main: this is called by the utils test suite to confirm the Testing() test is false in prod.
package main

import (
	"os"

	"go.viam.com/rdk/utils"
)

func main() {
	if utils.Testing() {
		os.Exit(1)
	}
}
