package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"math"
	"sync/atomic"
	"time"

	"gocv.io/x/gocv"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/widget"

	//"github.com/tonyOreglia/glee/pkg/generate"
	"github.com/tonyOreglia/glee/pkg/engine"
	"github.com/tonyOreglia/glee/pkg/moves"
	"github.com/tonyOreglia/glee/pkg/position"

	"github.com/Ernyoke/Imger/resize"

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
	var x = float64(chess[0] - 'a')
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
	foo := getCoord("a4")
	foo.x -= .2
	foo.y -= .2

	where := myArm.State.CartesianInfo
	where.X = foo.x
	where.Y = foo.y

	return myArm.MoveToPositionC(where)
}

func initArm(myArm *arm.URArm, myGripper *gripper.Gripper) error {
	// temp init, what to do?
	rx := -math.Pi
	ry := 0.0
	rz := 0.0

	foo := getCoord("d4")
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

	i, err = resize.ResizeRGBA(i.(*image.RGBA), .6, .6, resize.InterNearest)
	if err != nil {
		return nil, err
	}

	i2 := canvas.NewImageFromImage(i)
	bounds := i.Bounds()
	i2.SetMinSize(fyne.Size{bounds.Max.X, bounds.Max.Y})
	return i2, nil
}

func searchForNextMove(p *position.Position) (*position.Position, *moves.Move) {
	//mvs := generate.GenerateMoves(p)
	perft := 0
	singlePlyPerft := 0
	params := engine.SearchParams{
		Depth:          5,
		Ply:            5,
		Pos:            &p,
		Perft:          &perft,
		SinglePlyPerft: &singlePlyPerft,
		EngineMove:     &moves.Move{},
	}
	if p.IsWhitesTurn() {
		engine.AlphaBetaMax(-10000, 10000, 5, params)
	} else {
		engine.AlphaBetaMin(-10000, 10000, 5, params)
	}
	return p, params.EngineMove
}

func testChessEngine() {
	p := position.StartingPosition()
	p.Print()

	from, _ := moves.ConvertAlgebriacToIndex("e2")
	to, _ := moves.ConvertAlgebriacToIndex("e4")
	m := moves.NewMove([]int{from, to})
	//p.MakeMoveAlgebraic("e2","e4")
	if !engine.MakeValidMove(*m, &p) {
		fmt.Println("CHEATER")
	}
	p.Print()

	p, m = searchForNextMove(p)

	fmt.Printf("%v\n", m)
	p.Print()

	panic(1)
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

	err = initArm(myArm, myGripper)
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

	boardState := boardStateGuesser{}

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

			if boardState.newData(theBoard) {
				wantPicture = 1
			}

			annotated := theBoard.Annotate()

			grossBoard = theBoard

			pcs[1], err = matToFyne(annotated)
			if err != nil {
				panic(err)
			}

			w.SetContent(widget.NewHBox(pcs...))

			if atomic.LoadInt32(&wantPicture) != 0 {
				tm := time.Now().Unix()

				fn := fmt.Sprintf("data/board-%d.png", tm)
				log.Printf("saving image %s\n", fn)
				gocv.IMWrite(fn, img)

				fn = fmt.Sprintf("data/board-%d.dat", tm)
				foo := vision.NewDepthMapFromMat(depth)
				err = foo.WriteToFile(fn)
				if err != nil {
					panic(err)
				}

				atomic.StoreInt32(&wantPicture, 0)
			}
		}
	}()

	w.ShowAndRun()
}
