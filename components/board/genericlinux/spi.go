//go:build linux

package genericlinux

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"

	"go.viam.com/rdk/components/board"
)

func NewSpiBus(name string) board.SPI {
	bus := spiBus{}
	bus.reset(name)
	return &bus
}

type spiBus struct {
	mu         sync.Mutex
	openHandle *spiHandle
	bus        atomic.Pointer[string]
}

type spiHandle struct {
	bus      *spiBus
	isClosed bool
}

func (sb *spiBus) OpenHandle() (board.SPIHandle, error) {
	sb.mu.Lock()
	sb.openHandle = &spiHandle{bus: sb, isClosed: false}
	return sb.openHandle, nil
}

func (sb *spiBus) Close(ctx context.Context) error {
	return nil
}

func (sb *spiBus) reset(bus string) {
	sb.bus.Store(&bus)
}

func (sh *spiHandle) Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) (rx []byte, err error) {
	if sh.isClosed {
		return nil, errors.New("can't use Xfer() on an already closed SPIHandle")
	}

	busPtr := sh.bus.bus.Load()
	if busPtr == nil {
		return nil, errors.New("no bus selected")
	}

	port, err := spireg.Open(fmt.Sprintf("SPI%s.%s", *busPtr, chipSelect))
	if err != nil {
		return nil, err
	}
	defer func() {
		err = multierr.Combine(err, port.Close())
	}()
	conn, err := port.Connect(physic.Hertz*physic.Frequency(baud), spi.Mode(mode), 8)
	if err != nil {
		return nil, err
	}
	rx = make([]byte, len(tx))
	return rx, conn.Tx(tx, rx)
}

func (sh *spiHandle) Close() error {
	sh.isClosed = true
	sh.bus.mu.Unlock()
	return nil
}
