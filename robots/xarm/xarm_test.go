package xarm

import (
	"context"
	"fmt"
	//~ "net"
	"testing"
	//~ "time"
	
	"go.viam.com/core/arm"
	//~ "go.viam.com/core/utils"
)

func TestConnect(t *testing.T) {
	
	fmt.Println("making new arm")
	x, _ := NewXarm6()
	fmt.Println("...done")
	//~ time.Sleep(2 * time.Second)
	x.MoveToJointPositions(context.Background(), arm.JointPositionsFromRadians([]float64{-0.78, -0.78, -0.78, -0.78, 0, 0}))
	//~ time.Sleep(6 * time.Second)
	x.MoveToJointPositions(context.Background(), arm.JointPositionsFromRadians([]float64{0,0,0,0, 0, 0}))
	//~ time.Sleep(3 * time.Second)
	fmt.Println("closing")
	x.Close()
	fmt.Println("closed")
}
