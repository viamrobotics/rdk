package inject

import (
	"go.viam.com/rdk/components/board"
)

// SPI is an injected SPI.
type SPI struct {
	board.SPI
	OpenHandleFunc func() (board.SPIHandle, error)
}

// OpenHandle calls the injected OpenHandle or the real version.
func (s *SPI) OpenHandle() (board.SPIHandle, error) {
	if s.OpenHandleFunc == nil {
		return s.SPI.OpenHandle()
	}
	return s.OpenHandleFunc()
}
