package inject

import (
	"braces.dev/errtrace"
	"go.viam.com/rdk/components/board/genericlinux/buses"
)

// SPI is an injected SPI.
type SPI struct {
	buses.SPI
	OpenHandleFunc func() (buses.SPIHandle, error)
}

// OpenHandle calls the injected OpenHandle or the real version.
func (s *SPI) OpenHandle() (buses.SPIHandle, error) {
	if s.OpenHandleFunc == nil {
		return errtrace.Wrap2(s.SPI.OpenHandle())
	}
	return errtrace.Wrap2(s.OpenHandleFunc())
}
