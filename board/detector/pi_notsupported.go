// +build !pi

package detector

import (
	"fmt"

	"go.viam.com/robotcore/board"
)

func init() {
	board.RegisterBoard("pi", func(cfg board.Config) (board.Board, error) {
		return nil, fmt.Errorf("not running on a pi")
	})
}
