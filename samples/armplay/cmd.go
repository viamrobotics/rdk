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
	"github.com/echolabsinc/robotcore/gripper"
)

type pos struct {
	x float64
	y float64
}

var (
	A1             = pos{.31, -.08}
	H8             = pos{.69, .3}
	SafeMoveHeight = .1
)

func getCoord(chess string) pos {
	var x = float64(chess[0] - 'A')
	var y = float64(chess[1] - '1')

	if x < 0 || x > 7 || y < 0 || y > 7 {
		panic(fmt.Errorf("invalid position: %s", chess))
	}

	x = A1.x + (x * (H8.x - A1.x) / 7)
	y = A1.y + (y * (H8.y - A1.y) / 7)

	return pos{x, y}
}

func movePiece(myArm *arm.URArm, from, to string) error {

	// first make sure in safe position
	where := myArm.State.CartesianInfo
	where.Z = SafeMoveHeight

	myArm.MoveToPositionC(where)

	// move to from
	f := getCoord(from)
	where.X = f.x
	where.Y = f.y
	myArm.MoveToPositionC(where)

	// grab piece
	// TODO

	// move to where
	t := getCoord(to)
	where.X = t.x
	where.Y = t.y
	myArm.MoveToPositionC(where)

	// drop piece
	// TODO
	return nil
}

func initArm(myArm *arm.URArm) error {
	// temp init, what to do?
	rx := -math.Pi
	ry := 0.0
	rz := 0.0

	foo := getCoord("D4")
	myArm.MoveToPosition(foo.x, foo.y, SafeMoveHeight, rx, ry, rz)

	return nil
}

func main() {
	wantPicture := int32(0)

	myArm, err := arm.URArmConnect("192.168.2.155")
	if err != nil {
		panic(err)
	}

	err = initArm(myArm)
	if err != nil {
		panic(err)
	}

	myGripper, err := gripper.NewGripper("192.168.2.155")
	if err != nil {
		panic(err)
	}

	if false {
		for _, s := range []string{"A1", "B1", "G5", "F4"} {
			c := getCoord(s)

			where := myArm.State.CartesianInfo
			where.X = c.x
			where.Y = c.y
			where.Z = SafeMoveHeight

			fmt.Printf("%s %f %f\n", s, c.x, c.y)

			myArm.MoveToPositionC(where)
		}

		movePiece(myArm, "F3", "G5")
		return
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

		case "O":
			_, err := myGripper.Open()
			if err != nil {
				panic(err)
			}
			changed = false
		case "C":
			_, err := myGripper.Close()
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
