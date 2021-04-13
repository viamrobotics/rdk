package artifact

import (
	"fmt"
)

func Path(to string) (string, error) {
	cache, err := GlobalCache()
	if err != nil {
		return "", err
	}
	actualPath, err := cache.Ensure(to)
	if err != nil {
		return "", fmt.Errorf("error ensuring %q: %w", to, err)
	}
	return actualPath, nil
}

func MustPath(to string) string {
	resolved, err := Path(to)
	if err != nil {
		panic(err)
	}
	return resolved
}

func NewPath(to string) (string, error) {
	cache, err := GlobalCache()
	if err != nil {
		return "", err
	}
	return cache.NewPath(to), nil
}

func MustNewPath(to string) string {
	resolved, err := NewPath(to)
	if err != nil {
		panic(err)
	}
	return resolved
}
