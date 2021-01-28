package base

type Move struct {
	DistanceMM int
	AngleDeg   int
	Speed      int
	Block      bool
}

func DoMove(move Move, base Base) (int, int, error) {
	if move.AngleDeg != 0 {
		if err := base.Spin(move.AngleDeg, move.Speed, move.Block); err != nil {
			// TODO(erd): Spin should report amount spun if errored
			return 0, 0, err
		}
	}

	if move.DistanceMM != 0 {
		if err := base.MoveStraight(move.DistanceMM, move.Speed, move.Block); err != nil {
			// TODO(erd): MoveStraight should report amount moved if errored
			return move.AngleDeg, 0, err
		}
	}

	return move.AngleDeg, move.DistanceMM, nil
}
