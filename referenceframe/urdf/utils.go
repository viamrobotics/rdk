package urdf

import (
	"math"
	"strconv"
	"strings"
)

// spaceDelimitedStringToFloatSlice is a helper method to split up space-delimited fields in a string and converts them to floats.
func spaceDelimitedStringToFloatSlice(s string) []float64 {
	var converted []float64
	slice := strings.Fields(s)
	for _, value := range slice {
		value, err := strconv.ParseFloat(value, 64)
		if err != nil {
			value = math.NaN()
		}
		converted = append(converted, value)
	}
	return converted
}
