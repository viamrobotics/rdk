package vision

type WalkCallback func(x, y int) error

func Walk(middleX, middleY, maxRadius int, f WalkCallback) error {

	err := f(middleX, middleY)
	if err != nil {
		return err
	}

	for radius := 1; radius <= maxRadius; radius++ {
		for x := middleX - radius; x <= middleX+radius; x++ {
			err = f(x, middleY+radius)
			if err != nil {
				return err
			}

			err = f(x, middleY-radius)
			if err != nil {
				return err
			}
		}

		for y := middleY - radius + 1; y < middleY+radius; y++ {
			err = f(middleX-radius, y)
			if err != nil {
				return err
			}

			err = f(middleX+radius, y)
			if err != nil {
				return err
			}
		}

	}

	return nil
}
