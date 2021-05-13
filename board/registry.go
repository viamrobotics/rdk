package board

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
)

// A CreateBoard creates a board from a given config.
type CreateBoard func(ctx context.Context, cfg Config, logger golog.Logger) (Board, error)

var (
	boardRegistry = map[string]CreateBoard{}
)

// RegisterBoard register a board model to a creator.
func RegisterBoard(name string, creator CreateBoard) {
	_, old := boardRegistry[name]
	if old {
		panic(fmt.Errorf("board model [%s] already registered", name))
	}

	boardRegistry[name] = creator
}

// NewBoard constructs a new board based on the given config. The model within the
// config must be associated to a registered creator.
func NewBoard(ctx context.Context, cfg Config, logger golog.Logger) (Board, error) {
	creator, have := boardRegistry[cfg.Model]
	if !have {
		return nil, fmt.Errorf("unknown board model: %v", cfg.Model)
	}
	return creator(ctx, cfg, logger)
}
