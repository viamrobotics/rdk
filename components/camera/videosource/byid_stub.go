//go:build !linux

package videosource

import "github.com/pkg/errors"

// resolveByIDName is a no-op on non-Linux platforms. /dev/v4l/by-id names are created by
// udev, so there is nothing to reconstruct elsewhere. See byid_linux.go.
func resolveByIDName(_ string) (string, error) {
	return "", errors.New("by-id name resolution is only supported on linux")
}
