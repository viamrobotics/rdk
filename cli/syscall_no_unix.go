//go:build !unix
// +build !unix

package cli

import (
	"os"
)

func sigwinchSignal() (os.Signal, bool) {
	return nil, false
}
