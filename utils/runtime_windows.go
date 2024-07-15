package utils

import "os"

// copied from goutils
func notifySignals(channel chan os.Signal) {
	println("skipping notifySignals on windows platform")
}
