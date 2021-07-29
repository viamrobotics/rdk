package forcesensor

import (
	"context"
	"fmt"
	"github.com/edaniels/golog"
	"go.viam.com/core/board"
)

type ForceMatrixController struct {
	gpioPins []string
	analogChannels []int
	
	analogReaders []board.AnalogReader
	board board.Board
}

func NewForceMatrixController(ctx context.Context, gpioPins []string, analogChannels []int, logger golog.Logger) (*ForceMatrixController, error) {
	fmc := ForceMatrixController{
		gpioPins: gpioPins,
		analogChannels: analogChannels,
	}

	if err := fmc.createBoard(ctx, logger); err != nil {
		return nil, err
	}

	return &fmc, nil
}

func (fmc *ForceMatrixController) Matrix(ctx context.Context) ([][]int, error) {
	matrix := make([][]int, len(fmc.gpioPins), len(fmc.gpioPins))
	for i := 0; i < len(fmc.gpioPins); i++ {
		if err := fmc.board.GPIOSet(fmc.gpioPins[i], true); err != nil {
			return nil, err
		}

		for j, pin := range fmc.gpioPins {
			if i != j {
				err := fmc.board.GPIOSet(pin, false)
				if err != nil {
					return nil, err
				}
			}
		}

		for _, analogReader := range fmc.analogReaders {
			val, err := analogReader.Read(ctx)
			if err != nil {
				return nil, err
			}
			matrix[i] = append(matrix[i], val)
		}
	}

	return matrix, nil
}

func (fmc *ForceMatrixController) createBoard(ctx context.Context, logger golog.Logger) error {
	cfg := board.Config{
		Model: "pi",
	}

	for _, ac := range fmc.analogChannels {
		cfg.Analogs = append(cfg.Analogs, board.AnalogConfig{
			Name: fmt.Sprintf("a%d", ac),
			Pin:  fmt.Sprintf("%d", ac),
		})
	}

	var err error
	fmc.board, err = board.NewBoard(ctx, cfg, logger)
	if err != nil {
		return err
	}

	for _, a := range cfg.Analogs {
		fmc.analogReaders = append(fmc.analogReaders, fmc.board.AnalogReader(a.Name))
	}

	return nil
}

func (fmc *ForceMatrixController) Close() {
	fmc.board.Close()
}