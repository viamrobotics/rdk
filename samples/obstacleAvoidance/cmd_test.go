package main

import (
	"os"
	"testing"
)

func TestMainSimulation(t *testing.T) {
	os.Args = append([]string{""}, []string{"--visualize"}...)
	_ = t
	main()
}
