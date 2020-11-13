package chess

import (
	"fmt"
)

func (b *Board) Piece(square string) {
	fmt.Printf("Piece %s\n", square)

	myRadius := 3

	corner := getMinChessCorner(square)
	for x := corner.X + 50 - myRadius; x < corner.X+50+myRadius; x++ {
		for y := corner.Y + 50 - myRadius; y < corner.Y+50+myRadius; y++ {
			data := b.color.GetVecbAt(y, x)
			fmt.Printf("\t%d,%d %v\n", x, y, data)
		}
	}

}
