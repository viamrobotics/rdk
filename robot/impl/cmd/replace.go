package main

import (
	"fmt"
	"os"
	"regexp"
)

var re = regexp.MustCompile(`data/.*\.json`)

func main() {
	contents, err := os.ReadFile("robot_reconfigure_test.go")
	if err != nil {
		panic(err)
	}
	re.ReplaceAll
	fmt.Println(string(contents))
}
