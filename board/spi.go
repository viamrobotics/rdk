package board

// SPIGPIOBoard is a merger of SPIBoard and Board
type SPIGPIOBoard interface {
	SPINames() []string
	SPI(name string) SPI
	Board
}

// SPIBoard is a board that supports one or more shareable serial devices.
type SPIBoard interface {
	SPINames() []string
	SPI(name string) SPI
}

// SPI represents a shareable SPI bus on the board
// Mutex (lock/unlock) functions are provided, and should always be locked before reading/writing and unlocked afterward.
type SPI interface {
	Lock()
	Unlock()
	Xfer(baud uint, csPin string, mode uint, tx []byte) (rx []byte, err error)
}
