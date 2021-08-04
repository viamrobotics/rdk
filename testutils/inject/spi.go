package inject

import (
	"go.viam.com/core/board"
)

// Servo is an injected servo.
type SPI struct {
	board.SPI
	OpenFunc    func() (board.SPIHandle, error)
}

// Move calls the injected Move or the real version.
func (s *SPI) Open() (board.SPIHandle, error) {
	if s.OpenFunc == nil {
		return s.SPI.Open()
	}
	return s.OpenFunc()
}