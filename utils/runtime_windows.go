package utils

import "os"

func notifySignals(channel chan os.Signal) {
	println("skipping notifySignals on windows platform")
}
