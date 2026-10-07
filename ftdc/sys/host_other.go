//go:build !linux

package sys

import (
	"errors"

	"go.viam.com/rdk/ftdc"
)

func newHostUsage() (ftdc.Statser, error) {
	return nil, errors.New("host stats are only collected on linux")
}
