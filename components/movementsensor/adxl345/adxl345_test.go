package adxl345

import (
	"context"
	"sync"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/testutils/inject"
)

func TestReadInterrupts(t *testing.T) {
	ctx := context.Background()
	cancelContext, cancelFunc := context.WithCancel(ctx)
	i2cHandle := &inject.I2CHandle{}
	i2cHandle.CloseFunc = func() error { return nil }
	i2c := &inject.I2C{}
	i2c.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) {
		return i2cHandle, nil
	}

	t.Run("increments tap and freefall counts when both interrupts have gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(1<<6) + byte(1<<2)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[string]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			mu:                sync.Mutex{},
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 1)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 1)
	})

	t.Run("increments freefall count only when freefall has gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(1 << 2)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[string]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			mu:                sync.Mutex{},
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 0)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 1)
	})

	t.Run("increments tap count only when only tap has gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(1 << 6)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[string]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			mu:                sync.Mutex{},
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 1)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 0)
	})

	t.Run("does not increment counts when neither interrupt has gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(0)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[string]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			mu:                sync.Mutex{},
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 0)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 0)
	})
}
