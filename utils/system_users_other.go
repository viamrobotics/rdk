//go:build !linux

package utils

import "go.viam.com/rdk/logging"

// ListRunnableSystemUsers returns the system users a module can be configured to run as. Running modules as
// another user is only supported on Linux, so this is always empty.
func ListRunnableSystemUsers(_ logging.Logger) []string {
	return nil
}
