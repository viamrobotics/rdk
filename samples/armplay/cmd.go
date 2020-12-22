package main

import (
	"flag"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"gocv.io/x/gocv"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/widget"

	"github.com/echolabsinc/robotcore/arm"
	"github.com/echolabsinc/robotcore/gripper"
	"github.com/echolabsinc/robotcore/vision"
)

type pos struct {
	x float64
	y float64
}

var (
	wantPicture = int32(0)
)

func matToFyne(img gocv.Mat) (*canvas.Image, error) {
	i, err := img.ToImage()
	if err != nil {
		return nil, err
	}

	i2 := canvas.NewImageFromImage(i)
	i2.SetMinSize(fyne.Size{img.Cols(), img.Rows()})
	return i2, nil
}

func main() {
	robotIp := "192.168.2.2"

	webcamDeviceId := 0

	flag.IntVar(&webcamDeviceId, "webcam", 0, "which webcam to use")
	flag.Parse()

	myArm, err := arm.URArmConnect(robotIp)
	if err != nil {
		panic(err)
	}

	myGripper, err := gripper.NewGripper(robotIp)
	if err != nil {
		panic(err)
	}

	//webcam, err := vision.NewWebcamSource(webcamDeviceId)
	webcam := vision.NewHttpSourceIntelEliot("127.0.0.1:8181")
	if err != nil {
		panic(err)
	}
	defer webcam.Close()

	a := app.New()
	w := a.NewWindow("Hello")

	stateDisplay := arm.NewStateDisplay()

	pcs := []fyne.CanvasObject{
		widget.NewLabel("Hello Fyne!"),
		widget.NewLabel("Hello Fyne!"),
		stateDisplay.TheContainer,
	}
	w.SetContent(widget.NewHBox(pcs...))

	go func() {
		for {
			time.Sleep(10 * time.Millisecond)
			stateDisplay.Update(myArm.State)
		}
	}()

	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		c := myArm.State.CartesianInfo
		pre := c.SimpleString()

		changed := true
		switch k.Name {
		case "Up":
			c.Y += .01
		case "Down":
			c.Y -= .01
		case "Left":
			c.X -= .01
		case "Right":
			c.X += .01
		case "U":
			c.Z += .01
		case "D":
			c.Z -= .01
		case ".":
			myArm.JointMoveDelta(5, .05)
			changed = false
		case ",":
			myArm.JointMoveDelta(5, -.05)
			changed = false
		case "P":
			// take a picture
			atomic.StoreInt32(&wantPicture, 1)
			changed = false

		case "O":
			err := myGripper.Open()
			if err != nil {
				panic(err)
			}
			changed = false
		case "C":
			err := myGripper.Close()
			if err != nil {
				panic(err)
			}
			changed = false

		case "Q":
			w.Close()

		default:
			log.Printf("unknown: %s\n", k.Name)
			changed = false
		}
		if changed {
			log.Printf("moving\n-%s\n-%s\n", pre, c.SimpleString())
			myArm.MoveToPositionC(c)
		}
	})

	go func() {
		for {
			img, _, err := webcam.NextColorDepthPair()
			func() {
				defer img.Close()
				if err != nil || img.Empty() {
					log.Printf("error reading device: %s\n", err)
					return
				}

				pcs[0], err = matToFyne(img)
				if err != nil {
					panic(err)
				}

				w.SetContent(widget.NewHBox(pcs...))

				if atomic.LoadInt32(&wantPicture) != 0 {
					fn := fmt.Sprintf("data/img-%d.jpg", time.Now().Unix())
					log.Printf("saving image %s\n", fn)
					gocv.IMWrite(fn, img)
					atomic.StoreInt32(&wantPicture, 0)
				}
			}()

		}
	}()

	w.ShowAndRun()
}
