package main

import (
	"os"
	"testing"
)

func TestMainHardware(t *testing.T) {
	main()
}

func TestMainSimulation(t *testing.T) {
	os.Args = append([]string{""}, []string{"--simulation=true"}...)
	main()
}
