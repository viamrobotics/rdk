package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/echolabsinc/robotcore/robot"
)

func main() {

	flag.Parse()

	cfgFile := flag.Arg(0)

	cfg, err := robot.ReadConfig(cfgFile)
	if err != nil {
		panic(err)
	}

	myRobot, err := robot.NewRobot(cfg)
	if err != nil {
		panic(err)
	}
	defer myRobot.Close()

	g := myRobot.Grippers[0]

	fmt.Println("ready")

	reader := bufio.NewReader(os.Stdin)
	for {
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "q" {
			return
		} else if text == "c" {
			err = g.Close()
			if err != nil {
				panic(err)
			}
		} else if text == "o" {
			err = g.Open()
			if err != nil {
				panic(err)
			}
		} else if text[0] == 's' {
			// s FOR 123
			pcs := strings.Split(text, " ")
			err := g.MultiSet([][]string{
				{"GTO", "0"},
				{"ACT", "0"},
				{pcs[1], pcs[2]},
				{"ACT", "1"}, // robot activate
				{"GTO", "1"}, // gripper activate
			})
			if err != nil {
				panic(err)
			}
		} else if text[0] == 'g' {
			// g FOR
			pcs := strings.Split(text, " ")
			val, err := g.Get(pcs[1])
			if err != nil {
				panic(err)
			}
			fmt.Println(val)
		} else {
			fmt.Printf("unknown command: %s\n", text)
		}

	}
}
