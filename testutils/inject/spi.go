package inject

import (
	"go.viam.com/core/board"
)

// SPI is an injected SPI.
type SPI struct {
	board.SPI
	OpenFunc func() (board.SPIHandle, error)
}

// Open calls the injected Open or the real version.
func (s *SPI) Open() (board.SPIHandle, error) {
	if s.OpenFunc == nil {
		return s.SPI.Open()
	}
	return s.OpenFunc()
}
