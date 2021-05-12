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

// RandomAlphaString returns a random alphabetic string of the given size.
// Note(erd): all random strings are subject to modulus bias; hope that
// does not matter to you.
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

// StringSet represents a mathematical set of string.
type StringSet map[string]struct{}

// NewStringSet returns a new string set from the given series of values
// where duplicates are okay.
func NewStringSet(values ...string) StringSet {
	set := make(StringSet, len(values))
	for _, val := range values {
		set[val] = struct{}{}
	}
	return set
}
