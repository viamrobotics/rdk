package spatialmath

import (
	"math"
	"strconv"
	"strings"
)

// spaceDelimitedStringToSlice is a helper method to split up space-delimited fields in URDFs, such as xyz or rpy attributes.
func spaceDelimitedStringToSlice(s string) []float64 {
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
