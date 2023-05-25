package utils

func ValidateBaudRate(validBaudRates []uint, baudRate int) bool {
	isValid := false
	for _, val := range validBaudRates {
		if val == uint(baudRate) {
			isValid = true
		}
	}
	return isValid
}
