package framesystem

import "fmt"

func wrongNumberOfResourcesError(count int, name string) error {
	return fmt.Errorf("got %d resources instead of 1 for (%s)", count, name)
}
