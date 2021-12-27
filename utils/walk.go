package utils

// WalkCallback is to be called for each point visited by Walk.
type WalkCallback func(x, y int) error

// Walk starts at the given middle point and walks around increasingly
// bigger squares based on the given radius growing outwards.
func Walk(middleX, middleY, maxRadius int, f WalkCallback) error {
	if err := f(middleX, middleY); err != nil {
		return err
	}

	for radius := 1; radius <= maxRadius; radius++ {
		for x := middleX - radius; x <= middleX+radius; x++ {
			if err := f(x, middleY+radius); err != nil {
				return err
			}

			if err := f(x, middleY-radius); err != nil {
				return err
			}
		}

		for y := middleY - radius + 1; y < middleY+radius; y++ {
			if err := f(middleX-radius, y); err != nil {
				return err
			}

			if err := f(middleX+radius, y); err != nil {
				return err
			}
		}
	}

	return nil
}
