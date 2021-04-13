package artifact

import (
	"fmt"
)

func Path(to string) (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}

	cache, err := NewCache(config)
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
