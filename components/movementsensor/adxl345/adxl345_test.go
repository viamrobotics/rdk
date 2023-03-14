package adxl345

import (
	"context"
	"sync"
	"testing"

	"go.viam.com/rdk/components/board"
	"go.viam.com/test"
)

const (
	testBoardName = "fake_board"
)

type mockI2CHandle struct{}

func (m *mockI2CHandle) Read(ctx context.Context, count int) ([]byte, error) {
	return []byte{}, nil
}

func (m *mockI2CHandle) Write(ctx context.Context, tx []byte) error {
	return nil
}

func (m *mockI2CHandle) ReadByteData(ctx context.Context, register byte) (byte, error) {
	return 0, nil
}

func (m *mockI2CHandle) WriteByteData(ctx context.Context, register, data byte) error {
	return nil
}

func (m *mockI2CHandle) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	intSourceRegister := byte(1<<6 + 1<<2)
	return []byte{intSourceRegister}, nil
}

func (m *mockI2CHandle) WriteBlockData(ctx context.Context, register byte, data []byte) error {
	return nil
}
func (m *mockI2CHandle) Close() error { return nil }

type mockI2C struct{ handleCount int }

func (mock *mockI2C) OpenHandle(addr byte) (board.I2CHandle, error) {
	return &mockI2CHandle{}, nil
}

func TestReadInterruptsBothInterrupts(t *testing.T) {
	ctx := context.Background()
	cancelContext, cancelFunc := context.WithCancel(ctx)
	sensor := &adxl345{
		bus:               &mockI2C{},
		interruptsFound:   map[string]int{},
		cancelContext:     cancelContext,
		cancelFunc:        cancelFunc,
		mu:                sync.Mutex{},
		interruptsEnabled: byte(1<<6 + 1<<2),
	}
	sensor.readInterrupts(sensor.cancelContext)
	test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 1)
	test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 1)
}

type mockI2CHandleSingleTap struct{}

func (m *mockI2CHandleSingleTap) Read(ctx context.Context, count int) ([]byte, error) {
	return []byte{}, nil
}

func (m *mockI2CHandleSingleTap) Write(ctx context.Context, tx []byte) error {
	return nil
}

func (m *mockI2CHandleSingleTap) ReadByteData(ctx context.Context, register byte) (byte, error) {
	return 0, nil
}

func (m *mockI2CHandleSingleTap) WriteByteData(ctx context.Context, register, data byte) error {
	return nil
}

func (m *mockI2CHandleSingleTap) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	intSourceRegister := byte(1 << 6)
	return []byte{intSourceRegister}, nil
}

func (m *mockI2CHandleSingleTap) WriteBlockData(ctx context.Context, register byte, data []byte) error {
	return nil
}
func (m *mockI2CHandleSingleTap) Close() error { return nil }

type mockI2CSingleTap struct{ handleCount int }

func (mock *mockI2CSingleTap) OpenHandle(addr byte) (board.I2CHandle, error) {
	return &mockI2CHandleSingleTap{}, nil
}

func TestReadInterruptsSingleTap(t *testing.T) {
	ctx := context.Background()
	cancelContext, cancelFunc := context.WithCancel(ctx)
	sensor := &adxl345{
		bus:               &mockI2CSingleTap{},
		interruptsFound:   map[string]int{},
		cancelContext:     cancelContext,
		cancelFunc:        cancelFunc,
		mu:                sync.Mutex{},
		interruptsEnabled: byte(1<<6 + 1<<2),
	}
	sensor.readInterrupts(sensor.cancelContext)
	test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 1)
	test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 0)
}

type mockI2CHandleFreeFall struct{}

func (m *mockI2CHandleFreeFall) Read(ctx context.Context, count int) ([]byte, error) {
	return []byte{}, nil
}

func (m *mockI2CHandleFreeFall) Write(ctx context.Context, tx []byte) error {
	return nil
}

func (m *mockI2CHandleFreeFall) ReadByteData(ctx context.Context, register byte) (byte, error) {
	return 0, nil
}

func (m *mockI2CHandleFreeFall) WriteByteData(ctx context.Context, register, data byte) error {
	return nil
}

func (m *mockI2CHandleFreeFall) ReadBlockData(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
	intSourceRegister := byte(1 << 2)
	return []byte{intSourceRegister}, nil
}

func (m *mockI2CHandleFreeFall) WriteBlockData(ctx context.Context, register byte, data []byte) error {
	return nil
}
func (m *mockI2CHandleFreeFall) Close() error { return nil }

type mockI2CFreeFall struct{ handleCount int }

func (mock *mockI2CFreeFall) OpenHandle(addr byte) (board.I2CHandle, error) {
	return &mockI2CHandleFreeFall{}, nil
}

func TestReadInterruptsFreeFall(t *testing.T) {
	ctx := context.Background()
	cancelContext, cancelFunc := context.WithCancel(ctx)
	sensor := &adxl345{
		bus:               &mockI2CFreeFall{},
		interruptsFound:   map[string]int{},
		cancelContext:     cancelContext,
		cancelFunc:        cancelFunc,
		mu:                sync.Mutex{},
		interruptsEnabled: byte(1<<6 + 1<<2),
	}
	sensor.readInterrupts(sensor.cancelContext)
	test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 0)
	test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 1)
}

func TestReadingsFreeFall(t *testing.T) {
	ctx := context.Background()
	cancelContext, cancelFunc := context.WithCancel(ctx)
	sensor := &adxl345{
		bus:               &mockI2CFreeFall{},
		interruptsFound:   map[string]int{},
		cancelContext:     cancelContext,
		cancelFunc:        cancelFunc,
		mu:                sync.Mutex{},
		interruptsEnabled: byte(1<<6 + 1<<2),
	}
	sensor.echoInterrupt1 = &board.BasicDigitalInterrupt{}

	extra := make(map[string]interface{})
	response, err := sensor.Readings(ctx, extra)
	test.That(t, err, test.ShouldEqual, 0)
	test.That(t, response["single_tap_count"], test.ShouldBeNil)
	test.That(t, response["freefall_count"], test.ShouldEqual, 1)
}
