package main

import (
	"flag"
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
	"github.com/echolabsinc/robotcore/vision"
)

type pos struct {
	x float64
	y float64
}

var (
	A1             = pos{.344, -.195}
	H8             = pos{A1.x + .381, A1.y + .381}
	SafeMoveHeight = .092
	BottomHeight   = .01

	wantPicture = int32(0)
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

func moveTo(myArm *arm.URArm, chess string, heightMod float64) error {
	// first make sure in safe position
	where := myArm.State.CartesianInfo
	where.Z = SafeMoveHeight + heightMod
	err := myArm.MoveToPositionC(where)
	if err != nil {
		return err
	}

	// move
	f := getCoord(chess)
	where.X = f.x
	where.Y = f.y
	return myArm.MoveToPositionC(where)
}

func movePiece(myArm *arm.URArm, myGripper *gripper.Gripper, from, to string) error {

	err := moveTo(myArm, from, 0)
	if err != nil {
		return err
	}

	where := myArm.State.CartesianInfo

	// grab piece
	for {
		closed, err := myGripper.Close()
		if err != nil {
			return err
		}
		if !closed {
			fmt.Printf("got a piece at height %f\n", where.Z)
			// got the piece
			break
		}
		_, err = myGripper.Open()
		if err != nil {
			return err
		}
		fmt.Println("no piece")
		where = myArm.State.CartesianInfo
		where.Z = where.Z - .01
		if where.Z <= BottomHeight {
			return fmt.Errorf("no piece")
		}
		myArm.MoveToPositionC(where)
	}

	saveZ := where.Z // save the height to bring the piece down to

	moveTo(myArm, to, .1)

	// drop piece
	where = myArm.State.CartesianInfo
	where.Z = saveZ
	myArm.MoveToPositionC(where)

	myGripper.Open()

	where = myArm.State.CartesianInfo
	where.Z = SafeMoveHeight
	myArm.MoveToPositionC(where)

	return nil
}

func doTraining(myArm *arm.URArm, myGripper *gripper.Gripper) {
	for x := 'A'; x <= 'H'; x = x + 1 {
		for y := '1'; y <= '8'; y = y + 1 {
			spot := string(x) + string(y)
			fmt.Println(spot)

			err := moveTo(myArm, spot, 0)
			if err != nil {
				panic(err)
			}

			if true {
				time.Sleep(time.Millisecond * 300) // let camera focus
				// just take a picture at the top
				atomic.StoreInt32(&wantPicture, 1)
				for {
					time.Sleep(time.Millisecond * 2)
					if atomic.LoadInt32(&wantPicture) == 0 {
						break
					}
				}
			} else {
				// find the pieces
				for {
					where := myArm.State.CartesianInfo
					closed, err := myGripper.Close()
					if err != nil {
						panic(err)
					}
					if !closed {
						fmt.Printf("\t got a piece at height %f\n", where.Z)
						break
					}
					_, err = myGripper.Open()
					if err != nil {
						panic(err)
					}

					where.Z = where.Z - .01
					if where.Z <= BottomHeight {
						fmt.Printf("\t no piece")
						break
					}
					myArm.MoveToPositionC(where)
				}

				myGripper.Open()

				err = moveTo(myArm, spot, 0)
				if err != nil {
					panic(err)
				}
			}

		}
	}

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

	err = initArm(myArm)
	if err != nil {
		panic(err)
	}

	myGripper, err := gripper.NewGripper(robotIp)
	if err != nil {
		panic(err)
	}

	if false { // test 1
		for _, s := range []string{"A1", "B1", "G5", "F4"} {
			c := getCoord(s)

			where := myArm.State.CartesianInfo
			where.X = c.x
			where.Y = c.y
			where.Z = SafeMoveHeight

			fmt.Printf("%s %f %f\n", s, c.x, c.y)

			myArm.MoveToPositionC(where)
		}
	}
	if false { // test 2
		err = movePiece(myArm, myGripper, "E2", "E4")
		if err != nil {
			panic(err)
		}
		return
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

	nextChessPos := ""

	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		c := myArm.State.CartesianInfo
		pre := c.SimpleString()

		if len(nextChessPos) > 0 {
			nextChessPos = nextChessPos + string(k.Name)
			fmt.Printf("nextChessPos [%s]\n", nextChessPos)
			if len(nextChessPos) == 3 {
				moveTo(myArm, nextChessPos[1:], 0)
				nextChessPos = ""
			} else if len(nextChessPos) > 3 {
				fmt.Printf("broken")
				nextChessPos = ""
			}
			return
		}

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

		case "/":
			nextChessPos = "-"
			return
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
			img, depth, err := webcam.NextColorDepthPair()
			defer img.Close()
			if err != nil || img.Empty() {
				log.Printf("error reading device: %s\n", err)
				continue
			}

			pcs[0], err = matToFyne(img)
			if err != nil {
				panic(err)
			}

			corners, err := vision.FindChessCorners(img)
			if err != nil {
				panic(err)
			}
			warped, _, err := vision.WarpColorAndDepthToChess(img, depth, corners)
			if err != nil {
				panic(err)
			}

			pcs[1], err = matToFyne(warped)
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
		}
	}()

	//go doTraining(myArm, myGripper)

	w.ShowAndRun()
}
