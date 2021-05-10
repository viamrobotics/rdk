// +build !pi

package detector

import (
	"context"
	"errors"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/board"
)

// init registers a failing pi board since this can only be compiled on non-pi systems.
func init() {
	board.RegisterBoard("pi", func(ctx context.Context, cfg board.Config, logger golog.Logger) (board.Board, error) {
		return nil, errors.New("not running on a pi")
	})
}
