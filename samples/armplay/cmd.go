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
	"github.com/echolabsinc/robotcore/vision/chess"
)

type pos struct {
	x float64
	y float64
}

var (
	A1             = pos{.344, -.195}
	H8             = pos{A1.x + .381, A1.y + .381}
	BoardHeight    = -.010
	SafeMoveHeight = BoardHeight + .15

	wantPicture = int32(0)
)

var grossBoard *chess.Board

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

	// open before going down
	_, err = myGripper.Open()
	if err != nil {
		return err
	}

	height := grossBoard.PieceHeight(from)
	where := myArm.State.CartesianInfo
	where.Z = BoardHeight + (height / 1000) + .001
	myArm.MoveToPositionC(where)

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
		if where.Z <= BoardHeight {
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

	moveOutOfWay(myArm)
	return nil
}

func moveOutOfWay(myArm *arm.URArm) error {
	foo := getCoord("A4")
	foo.x -= .2
	foo.y -= .2

	where := myArm.State.CartesianInfo
	where.X = foo.x
	where.Y = foo.y

	return myArm.MoveToPositionC(where)
}

func initArm(myArm *arm.URArm) error {
	// temp init, what to do?
	rx := -math.Pi
	ry := 0.0
	rz := 0.0

	foo := getCoord("D4")
	err := myArm.MoveToPosition(foo.x, foo.y, SafeMoveHeight, rx, ry, rz)
	if err != nil {
		return err
	}

	return moveOutOfWay(myArm)
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
			if string(k.Name) == "X" {
				nextChessPos = ""
			} else if nextChessPos[0] == '-' && len(nextChessPos) == 3 {
				moveTo(myArm, nextChessPos[1:], 0)
				nextChessPos = ""
			} else if nextChessPos[0] == 'M' && len(nextChessPos) == 5 {
				movePiece(myArm, myGripper, nextChessPos[1:3], nextChessPos[3:])
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
		case "M":
			nextChessPos = "M"
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

			theBoard, err := chess.FindAndWarpBoard(img, depth)
			if err != nil {
				panic(err)
			}

			annotated := theBoard.Annotate()

			grossBoard = theBoard

			pcs[1], err = matToFyne(annotated)
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

	go eliotTest(myArm, myGripper)

	w.ShowAndRun()
}

func eliotTest(myArm *arm.URArm, myGripper *gripper.Gripper) {
	err := moveOutOfWay(myArm)
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Millisecond * 2000)

	if true {
		return
	}

	err = movePiece(myArm, myGripper, "E2", "E4")
	err = movePiece(myArm, myGripper, "E7", "E5")
	err = movePiece(myArm, myGripper, "G1", "F3")
	err = movePiece(myArm, myGripper, "B8", "C6")

	if err != nil {
		panic(err)
	}

}
