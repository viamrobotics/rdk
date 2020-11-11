package vision

import (
	"fmt"
	"testing"

	"gocv.io/x/gocv"
)

func TestWhatColor1(t *testing.T) {
	fmt.Printf("r %v\n", Red)
	fmt.Printf("y %v\n", Yellow)
	data := gocv.Vecb{200, 20, 20}
	fmt.Printf("1 r %f\n", colorDistance(data, Red))
	fmt.Printf("1 y %f\n", colorDistance(data, Yellow))
	c, _ := WhatColor(data)
	if c != "red" {
		t.Errorf("got %s instead of red", c)
	}
}
