// +build !pi

package detector

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/board"
)

func init() {
	board.RegisterBoard("pi", func(ctx context.Context, cfg board.Config, logger golog.Logger) (board.Board, error) {
		return nil, fmt.Errorf("not running on a pi")
	})
}
