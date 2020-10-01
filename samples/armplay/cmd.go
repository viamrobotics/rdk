package main

import (
	"fmt"
	"log"
	"math"
	"sync/atomic"
	"time"

	"gocv.io/x/gocv"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/widget"

	"github.com/echolabsinc/robotcore/arm"
)

func main() {
	wantPicture := int32(0)

	myArm, err := arm.URArmConnect("192.168.2.155")
	if err != nil {
		panic(err)
	}

	time.Sleep(500 * time.Millisecond)

	rx := -math.Pi
	ry := 0.0
	rz := 0.0

	if false {
		myArm.MoveToPosition(.3, .4, 0.3, rx, ry, rz)   // desk back left
		myArm.MoveToPosition(.3, -.15, 0.3, rx, ry, rz) // desk front left
		myArm.MoveToPosition(.6, -.15, 0.3, rx, ry, rz) // desck front right
		myArm.MoveToPosition(.6, .4, 0.3, rx, ry, rz)   // desk back right

		myArm.MoveToPosition(.45, .1, 0, rx, ry, rz)   // desk middle down
		myArm.MoveToPosition(.45, .1, 0.5, rx, ry, rz) // desk middle up
	}

	// open webcam
	deviceID := 0
	webcam, err := gocv.OpenVideoCapture(0)
	if err != nil {
		panic(err)
	}
	defer webcam.Close()

	a := app.New()
	w := a.NewWindow("Hello")

	imgBox := canvas.NewImageFromFile("data.jpg")
	imgBox.SetMinSize(fyne.Size{600, 480})

	stateDisplay := arm.NewStateDisplay()

	pcs := []fyne.CanvasObject{
		imgBox,
		stateDisplay.TheContainer,
	}
	w.SetContent(widget.NewVBox(pcs...))

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
		// prepare image matrix
		img := gocv.NewMat()
		defer img.Close()

		log.Printf("start reading camera device: %v\n", deviceID)
		for {
			if ok := webcam.Read(&img); !ok {
				log.Printf("cannot read device %v\n", deviceID)
				continue
			}
			if img.Empty() {
				continue
			}

			i, err := img.ToImage()
			if err != nil {
				panic(err)
			}

			i2 := canvas.NewImageFromImage(i)
			i2.SetMinSize(fyne.Size{600, 480})
			pcs[0] = i2
			w.SetContent(widget.NewVBox(pcs...))

			if atomic.LoadInt32(&wantPicture) != 0 {
				fn := fmt.Sprintf("data/img-%d.jpg", time.Now().Unix())
				log.Printf("saving image %s\n", fn)
				gocv.IMWrite(fn, img)
				atomic.StoreInt32(&wantPicture, 0)
			}
		}
	}()

	w.ShowAndRun()
}
