package inject

import (
	"context"

	"go.viam.com/core/board"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"
)

type Board struct {
	board.Board
	MotorFunc            func(name string) board.Motor
	ServoFunc            func(name string) board.Servo
	AnalogReaderFunc     func(name string) board.AnalogReader
	DigitalInterruptFunc func(name string) board.DigitalInterrupt
	CloseFunc            func() error
	GetConfigFunc        func(ctx context.Context) (board.Config, error)
	StatusFunc           func(ctx context.Context) (*pb.BoardStatus, error)
}

func (b *Board) Motor(name string) board.Motor {
	if b.MotorFunc == nil {
		return b.Board.Motor(name)
	}
	return b.MotorFunc(name)
}

func (b *Board) Servo(name string) board.Servo {
	if b.ServoFunc == nil {
		return b.Board.Servo(name)
	}
	return b.ServoFunc(name)
}

func (b *Board) AnalogReader(name string) board.AnalogReader {
	if b.AnalogReaderFunc == nil {
		return b.Board.AnalogReader(name)
	}
	return b.AnalogReaderFunc(name)
}

func (b *Board) DigitalInterrupt(name string) board.DigitalInterrupt {
	if b.DigitalInterruptFunc == nil {
		return b.Board.DigitalInterrupt(name)
	}
	return b.DigitalInterruptFunc(name)
}

func (b *Board) Close() error {
	if b.CloseFunc == nil {
		return utils.TryClose(b.Board)
	}
	return b.CloseFunc()
}

func (b *Board) GetConfig(ctx context.Context) (board.Config, error) {
	if b.GetConfigFunc == nil {
		return b.Board.GetConfig(ctx)
	}
	return b.GetConfigFunc(ctx)
}

func (b *Board) Status(ctx context.Context) (*pb.BoardStatus, error) {
	if b.StatusFunc == nil {
		return b.Board.Status(ctx)
	}
	return b.StatusFunc(ctx)
}
