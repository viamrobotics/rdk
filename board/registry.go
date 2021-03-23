package board

import (
	"fmt"

	"github.com/edaniels/golog"
)

type CreateBoard func(cfg Config, logger golog.Logger) (Board, error)

var (
	boardRegistry = map[string]CreateBoard{}
)

func RegisterBoard(name string, c CreateBoard) {
	_, old := boardRegistry[name]
	if old {
		panic(fmt.Errorf("board model [%s] already registered", name))
	}

	boardRegistry[name] = c
}

func NewBoard(cfg Config, logger golog.Logger) (Board, error) {
	c, have := boardRegistry[cfg.Model]
	if !have {
		return nil, fmt.Errorf("unknown board model: %v", cfg.Model)
	}
	return c(cfg, logger)
}
