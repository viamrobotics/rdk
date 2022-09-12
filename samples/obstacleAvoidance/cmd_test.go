package main

import (
	"os"
	"testing"
)

func TestMainSimulation(_ *testing.T) {
	os.Args = append([]string{""}, []string{"--simulation", "--visualize"}...)
	main()
}
