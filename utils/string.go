package utils

import (
	"math/rand"
	"strings"
	"time"
)

const alphaLowers string = "abcdefghijklmnopqrstuvwxyz"

var alphaUppers string
var randSrc rand.Source

func init() {
	alphaUppers = strings.ToUpper(alphaLowers)
	randSrc = rand.NewSource(time.Now().Unix())
}

// note: all random strings are subject to modulus bias; hope that
// does not matter to you
func RandomAlphaString(size int) string {
	if size < 0 {
		return ""
	}
	chars := make([]byte, 0, size)
	for i := 0; i < size; i++ {
		val := int(randSrc.Int63())
		switch rand.Intn(2) {
		case 0:
			chars = append(chars, alphaLowers[val%len(alphaLowers)])
		case 1:
			chars = append(chars, alphaUppers[val%len(alphaUppers)])
		}
	}
	return string(chars)
}
