package cli

import "path/filepath"

// mapOver applies fn() to a slice of items and returns a slice of the return values.
func mapOver[T, U any](items []T, fn func(T) (U, error)) ([]U, error) {
	ret := make([]U, 0, len(items))
	for _, item := range items {
		newItem, err := fn(item)
		if err != nil {
			return nil, err
		}
		ret = append(ret, newItem)
	}
	return ret, nil
}

// samePath returns true if abs(path1) and abs(path2) are the same.
func samePath(path1, path2 string) (bool, error) {
	abs1, err := filepath.Abs(path1)
	if err != nil {
		return false, err
	}
	abs2, err := filepath.Abs(path2)
	if err != nil {
		return false, err
	}
	return abs1 == abs2, nil
}

// getMapString is a helper that returns map_[key] if it exists and is a string, otherwise empty string.
func getMapString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case []byte:
			return string(v)
		default:
			return ""
		}
	}
	return ""
}
