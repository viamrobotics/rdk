package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
)

func main() {
	err := realMain()
	if err != nil {
		panic(err)
	}
}

func realMain() error {
	ctx := context.Background()
	logger := logging.NewLogger("cmd-plan")

	flag.Parse()
	if len(flag.Args()) <= 0 {
		return fmt.Errorf("need a json file")
	}

	content, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		return err
	}

	req := armplanning.PlanRequest{}

	err = json.Unmarshal(content, &req)
	if err != nil {
		return err
	}

	start := time.Now()

	plan, err := armplanning.PlanMotion(ctx, logger, &req)
	if err != nil {
		return err
	}

	fmt.Printf("planning took %v\n", time.Since(start))

	if len(plan.Path()) != len(plan.Trajectory()) {
		return fmt.Errorf("path and trajectory not the same %d vs %s", len(plan.Path()), len(plan.Trajectory()))
	}

	fmt.Printf("steps\n")
	for idx, p := range plan.Path() {
		fmt.Printf("step %d\n", idx)

		t := plan.Trajectory()[idx]

		if len(p) != len(t) {
			return fmt.Errorf("p and t are different sizes %d vs %d", len(p), len(t))
		}

		for c, pp := range p {
			if len(t[c]) == 0 {
				continue
			}
			fmt.Printf("\t\t %s\n", c)
			fmt.Printf("\t\t\t %v\n", pp)
			fmt.Printf("\t\t\t %v\n", t[c])
			if idx > 0 {
				p := plan.Trajectory()[idx-1][c]
				fmt.Printf("\t\t\t\t distances l2: %0.4f Linf %0.4f\n",
					referenceframe.InputsL2Distance(p, t[c]),
					referenceframe.InputsLinfDistance(p, t[c]),
				)
			}
		}
	}

	return nil
}
