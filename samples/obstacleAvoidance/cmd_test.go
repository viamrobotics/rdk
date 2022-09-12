package main

import (
	"os"
	"testing"
)

func TestMainSimulation(t *testing.T) {
	t.Skip()
	os.Args = append([]string{""}, []string{"--simulation", "--visualize"}...)
	main()
}
