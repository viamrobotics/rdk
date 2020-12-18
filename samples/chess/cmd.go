package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"math"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
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
	"github.com/echolabsinc/robotcore/robot"
	"github.com/echolabsinc/robotcore/vision"
	"github.com/echolabsinc/robotcore/vision/chess"
)

type pos struct {
	x float64
	y float64
}

var (
	BoardWidth     = .381
	Center         = pos{-1, -1}
	BoardHeight    = -.010
	SafeMoveHeight = BoardHeight + .15

	wantPicture = int32(0)

	numPiecesCaptured = 0
)

func getCoord(chess string) pos {
	var x = float64(chess[0] - 'a')
	var y = float64(chess[1] - '1')

	if x < 0 || x > 7 || y < 0 || y > 7 {
		panic(fmt.Errorf("invalid position: %s", chess))
	}

	x = (3.5 - x) / 7.0
	y = (3.5 - y) / 7.0

	return pos{Center.x - (x * BoardWidth), Center.y - (y * BoardWidth)}
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
	if chess == "-" {
		f := getCoord("a8")
		where.X = f.x + (.06 * float64(numPiecesCaptured))
		where.Y = f.y + (BoardWidth / 5)
		numPiecesCaptured = numPiecesCaptured + 1
	} else {
		f := getCoord(chess)
		where.X = f.x
		where.Y = f.y
	}
	return myArm.MoveToPositionC(where)
}

func movePiece(boardState boardStateGuesser, myArm *arm.URArm, myGripper *gripper.Gripper, from, to string) error {

	if to[0] != '-' {
		toHeight, err := boardState.game.GetPieceHeight(boardState.NewestBoard(), to)
		if err != nil {
			return err
		}
		if toHeight > 0 {
			fmt.Printf("moving piece from %s to %s but occupied, going to capture\n", from, to)
			err = movePiece(boardState, myArm, myGripper, to, "-")
			if err != nil {
				return err
			}
		}
	}

	err := moveTo(myArm, from, 0)
	if err != nil {
		return err
	}

	// open before going down
	err = myGripper.Open()
	if err != nil {
		return err
	}

	height := boardState.NewestBoard().SquareCenterHeight(from, 35) // TODO: change to something more intelligent
	where := myArm.State.CartesianInfo
	where.Z = BoardHeight + (height / 1000) + .001
	myArm.MoveToPositionC(where)

	// grab piece
	for {
		grabbedSomething, err := myGripper.Grab()
		if err != nil {
			return err
		}
		if grabbedSomething {
			fmt.Printf("got a piece at height %f\n", where.Z)
			// got the piece
			break
		}
		err = myGripper.Open()
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

	if to == "-throw" {

		err = moveOutOfWay(myArm)
		if err != nil {
			return err
		}

		go func() {
			time.Sleep(200 * time.Millisecond)
			myGripper.Open()
		}()
		err = myArm.JointMoveDelta(4, -1)
		if err != nil {
			return err
		}

		return initArm(myArm) // this is to get joint position right
	}

	err = moveTo(myArm, to, .1)
	if err != nil {
		return err
	}

	// drop piece
	where = myArm.State.CartesianInfo
	where.Z = saveZ
	myArm.MoveToPositionC(where)

	myGripper.Open()

	if to != "-" {
		where = myArm.State.CartesianInfo
		where.Z = SafeMoveHeight
		myArm.MoveToPositionC(where)

		moveOutOfWay(myArm)
	}
	return nil
}

func moveOutOfWay(myArm *arm.URArm) error {
	foo := getCoord("a4")
	foo.x -= .2
	foo.y -= .2

	where := myArm.State.CartesianInfo
	where.X = foo.x
	where.Y = foo.y
	where.Z = SafeMoveHeight

	return myArm.MoveToPositionC(where)
}

func initArm(myArm *arm.URArm) error {
	// temp init, what to do?
	rx := -math.Pi
	ry := 0.0
	rz := 0.0

	foo := getCoord("a1")
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
		Depth:          3,
		Ply:            3,
		Pos:            &p,
		Perft:          &perft,
		SinglePlyPerft: &singlePlyPerft,
		EngineMove:     &moves.Move{},
	}
	if p.IsWhitesTurn() {
		engine.AlphaBetaMax(-10000, 10000, params.Ply, params)
	} else {
		engine.AlphaBetaMin(-10000, 10000, params.Ply, params)
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

func getWristPicCorners(wristCam vision.MatSource, debugNumber int) ([]image.Point, image.Point, error) {
	imageSize := image.Point{}
	m, _, err := wristCam.NextColorDepthPair()
	if err != nil {
		return nil, imageSize, err
	}
	imageSize.X = m.Cols()
	imageSize.Y = m.Rows()
	m.Close()

	// wait, cause this camera sucks
	time.Sleep(500 * time.Millisecond)
	m, _, err = wristCam.NextColorDepthPair()
	if err != nil {
		return nil, imageSize, err
	}
	m.Close()

	// wait, cause this camera sucks
	time.Sleep(500 * time.Millisecond)
	m, _, err = wristCam.NextColorDepthPair()
	if err != nil {
		return nil, imageSize, err
	}
	defer m.Close()

	// got picture finally

	out := gocv.NewMatWithSize(m.Rows(), m.Cols(), gocv.MatTypeCV8UC3)
	defer out.Close()

	img, err := vision.NewImage(m)
	if err != nil {
		return nil, imageSize, err
	}

	corners, err := chess.FindChessCornersPinkCheat(img, &out)
	if err != nil {
		return nil, imageSize, err
	}

	if debugNumber >= 0 {
		gocv.IMWrite(fmt.Sprintf("/tmp/foo-%d-in.png", debugNumber), m)
		gocv.IMWrite(fmt.Sprintf("/tmp/foo-%d-out.png", debugNumber), out)
	}

	fmt.Printf("Corners: %v\n", corners)

	return corners, imageSize, err
}

func lookForBoardAdjust(myArm *arm.URArm, wristCam vision.MatSource, corners []image.Point, imageSize image.Point) error {
	debugNumber := 100
	var err error
	for {
		where := myArm.State.CartesianInfo
		center := vision.Center(corners, 10000)

		xRatio := float64(center.X) / float64(imageSize.X)
		yRatio := float64(center.Y) / float64(imageSize.Y)

		xMove := (.5 - xRatio) / 8
		yMove := (.5 - yRatio) / -8

		fmt.Printf("center %v xRatio: %1.4v yRatio: %1.4v xMove: %1.4v yMove: %1.4f\n", center, xRatio, yRatio, xMove, yMove)

		if math.Abs(xMove) < .001 && math.Abs(yMove) < .001 {
			Center = pos{where.X, where.Y}

			// These are hard coded based on camera orientation
			Center.x += .026
			Center.y -= .073

			fmt.Printf("Center: %v\n", Center)
			fmt.Printf("a1: %v\n", getCoord("a1"))
			fmt.Printf("h8: %v\n", getCoord("h8"))
			return nil
		}

		where.X += xMove
		where.Y += yMove
		err = myArm.MoveToPositionC(where)
		if err != nil {
			return err
		}

		corners, _, err = getWristPicCorners(wristCam, debugNumber)
		debugNumber = debugNumber + 1
		if err != nil {
			return err
		}
	}

}

func lookForBoard(myArm *arm.URArm, myRobot *robot.Robot) error {
	debugNumber := 0

	wristCam := myRobot.CameraByName("wristCam")
	if wristCam == nil {
		return fmt.Errorf("can't find wristCam")
	}

	for foo := -1.0; foo <= 1.0; foo += 2 {
		where := myArm.State.CartesianInfo
		where.X = 0.524658
		where.Y = 0.094951
		where.Z = 0.603430
		where.Rx = -2.600206
		where.Ry = -0.007839
		where.Rz = -0.061827
		err := myArm.MoveToPositionC(where)
		if err != nil {
			return err
		}

		d := .1
		for i := 0.0; i < 1.6; i = i + d {
			err = myArm.JointMoveDelta(0, foo*d)
			if err != nil {
				return err
			}

			corners, imageSize, err := getWristPicCorners(wristCam, debugNumber)
			debugNumber = debugNumber + 1
			if err != nil {
				return err
			}

			if len(corners) == 4 {
				return lookForBoardAdjust(myArm, wristCam, corners, imageSize)
			}
		}
	}

	return nil

}

func main() {
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	cfgFile := flag.Arg(0)

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	cfg, err := robot.ReadConfig(cfgFile)
	if err != nil {
		panic(err)
	}

	myRobot, err := robot.NewRobot(cfg)
	if err != nil {
		panic(err)
	}
	defer myRobot.Close()

	myArm := myRobot.Arms[0]
	myGripper := myRobot.Grippers[0]
	webcam := myRobot.CameraByName("cameraOver")
	if webcam == nil {
		panic("can't find cameraOver camera")
	}

	err = lookForBoard(myArm, myRobot)
	if err != nil {
		panic(err)
	}

	err = initArm(myArm)
	if err != nil {
		panic(err)
	}

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
	currentPosition := position.StartingPosition()

	initialPositionOk := false
	lastKeyPress := ""

	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		switch k.Name {
		case "Q":
			w.Close()
		default:
			lastKeyPress = string(k.Name)
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

			i2, err := vision.NewImage(img)
			if err != nil {
				fmt.Println(err)
				continue
			}

			theBoard, err := chess.FindAndWarpBoard(i2, depth)
			if err != nil {
				fmt.Println(err)
				continue
			}

			if theBoard.IsBoardBlocked() {
				fmt.Println("board blocked")
				boardState.Clear()
				continue
			}

			interessting, err := boardState.newData(theBoard)
			if err != nil {
				wantPicture = 1
				fmt.Println(err)
				boardState.Clear()
			} else if interessting {
				wantPicture = 1
			}

			if boardState.Ready() {
				if !initialPositionOk {
					bb, err := boardState.GetBitBoard()
					if err != nil {
						fmt.Println("got inconsistency reading board, let's try again")
						boardState.Clear()
					} else if currentPosition.AllOccupiedSqsBb().Value() != bb.Value() {
						fmt.Printf("not in initial chess piece setup\n")
						bb.Print()
					} else {
						initialPositionOk = true
						fmt.Printf("GOT initial chess piece setup\n")
					}
				} else {
					// so we've already made sure we're safe, let's see if a move was made
					m, err := boardState.GetPrevMove(currentPosition)
					if err != nil {
						// trouble reading board, let's reset
						fmt.Println("got inconsistency reading board, let's try again")
						boardState.Clear()
					} else if m != nil {
						fmt.Printf("we detected a move: %s\n", m)

						if !engine.MakeValidMove(*m, &currentPosition) {
							panic("invalid move!")
						}

						currentPosition.Print()
						currentPosition.PrintFen()

						currentPosition, m = searchForNextMove(currentPosition)
						fmt.Printf("computer will make move: %s\n", m)
						err = movePiece(boardState, myArm, myGripper, m.String()[0:2], m.String()[2:])
						if err != nil {
							panic(err)
						}
						if !engine.MakeValidMove(*m, &currentPosition) {
							panic("wtf - invalid move chosen by computer")
						}
						currentPosition.Print()
						boardState.Clear()
					}
				}

				if lastKeyPress == "T" {
					err := movePiece(boardState, myArm, myGripper, "a1", "-throw")
					if err != nil {
						panic(err)
					}
					lastKeyPress = ""
				}
			}

			annotated := theBoard.Annotate()

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

				fn = fmt.Sprintf("data/board-%d.dat.gz", tm)
				err = depth.WriteToFile(fn)
				if err != nil {
					panic(err)
				}

				atomic.StoreInt32(&wantPicture, 0)
			}
		}
	}()

	w.ShowAndRun()
}
