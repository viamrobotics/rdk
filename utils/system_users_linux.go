package utils

import (
	"os"

	"go.viam.com/rdk/logging"
)

// ListRunnableSystemUsers returns the system users a module can be configured to run as. It is empty unless
// viam-server runs as root, since only root can start a process as a different user.
func ListRunnableSystemUsers(logger logging.Logger) []string {
	if os.Geteuid() != 0 {
		return nil
	}
	//nolint:gosec
	f, err := os.Open("/etc/passwd")
	if err != nil {
		logger.Debugw("unable to read system users", "error", err)
		return nil
	}
	defer func() {
		if err := f.Close(); err != nil {
			logger.Debugw("error closing /etc/passwd", "error", err)
		}
	}()
	return parseRunnableUsersFromPasswd(f)
}
