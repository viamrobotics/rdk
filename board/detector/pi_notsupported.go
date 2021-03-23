// +build !pi

package detector

import (
	"fmt"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/board"
)

func init() {
	board.RegisterBoard("pi", func(cfg board.Config, logger golog.Logger) (board.Board, error) {
		return nil, fmt.Errorf("not running on a pi")
	})
}
