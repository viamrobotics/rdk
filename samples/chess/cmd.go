package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"math"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"sync/atomic"
	"time"

	"go.viam.com/robotcore/arm"
	"go.viam.com/robotcore/gripper"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
	"go.viam.com/robotcore/utils"
	"go.viam.com/robotcore/vision/chess"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/tonyOreglia/glee/pkg/engine"
	"github.com/tonyOreglia/glee/pkg/moves"
	"github.com/tonyOreglia/glee/pkg/position"
)

type pos struct {
	x float64
	y float64
}

var (
	BoardWidth     = .381
	Center         = pos{-.435, .0}
	BoardHeight    = -.23
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

	return pos{Center.x + (x * BoardWidth), Center.y + (y * BoardWidth)} // HARD CODED
}

func moveTo(myArm arm.Arm, chess string, heightMod float64) error {
	// first make sure in safe position
	where, err := myArm.CurrentPosition()
	if err != nil {
		return err
	}
	where.Z = SafeMoveHeight + heightMod
	err = myArm.MoveToPosition(where)
	if err != nil {
		return err
	}

	// move
	if chess == "-" {
		f := getCoord("a8")
		where.X = f.x - (.06 * float64(numPiecesCaptured)) // HARD CODED
		where.Y = f.y - (BoardWidth / 5)                   // HARD CODED
		numPiecesCaptured = numPiecesCaptured + 1
	} else {
		f := getCoord(chess)
		where.X = f.x
		where.Y = f.y
	}
	return myArm.MoveToPosition(where)
}

func movePiece(boardState boardStateGuesser, robot *robot.Robot, myArm arm.Arm, myGripper gripper.Gripper, from, to string) error {

	if to[0] != '-' {
		toHeight, err := boardState.game.GetPieceHeight(boardState.NewestBoard(), to)
		if err != nil {
			return err
		}
		if toHeight > 0 {
			golog.Global.Debugf("moving piece from %s to %s but occupied, going to capture", from, to)
			err = movePiece(boardState, robot, myArm, myGripper, to, "-")
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

	err = adjustArmInsideSquare(robot)
	if err != nil {
		return err
	}

	height := boardState.NewestBoard().SquareCenterHeight(from, 35) // TODO(erh): change to something more intelligent
	where, err := myArm.CurrentPosition()
	if err != nil {
		return err
	}
	where.Z = BoardHeight + (height / 1000) + .01
	myArm.MoveToPosition(where)

	// grab piece
	for {
		grabbedSomething, err := myGripper.Grab()
		if err != nil {
			return err
		}
		if grabbedSomething {
			golog.Global.Debugf("got a piece at height %f", where.Z)
			// got the piece
			break
		}
		err = myGripper.Open()
		if err != nil {
			return err
		}
		golog.Global.Debug("no piece")
		where, err = myArm.CurrentPosition()
		if err != nil {
			return err
		}
		where.Z = where.Z - .01
		if where.Z <= BoardHeight {
			return fmt.Errorf("no piece")
		}
		myArm.MoveToPosition(where)
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
	where, err = myArm.CurrentPosition()
	if err != nil {
		return err
	}

	where.Z = saveZ
	myArm.MoveToPosition(where)

	myGripper.Open()

	if to != "-" {
		where, err = myArm.CurrentPosition()
		if err != nil {
			return err
		}
		where.Z = SafeMoveHeight
		myArm.MoveToPosition(where)

		moveOutOfWay(myArm)
	}
	return nil
}

func moveOutOfWay(myArm arm.Arm) error {
	foo := getCoord("a1")

	where, err := myArm.CurrentPosition()
	if err != nil {
		return err
	}
	where.X = foo.x
	where.Y = foo.y
	where.Z = SafeMoveHeight + .3 // HARD CODED

	return myArm.MoveToPosition(where)
}

func initArm(myArm arm.Arm) error {
	foo := getCoord("a1")
	err := myArm.MoveToPosition(arm.Position{
		X:  foo.x,
		Y:  foo.y,
		Z:  SafeMoveHeight,
		Rx: -180,
		Ry: 0,
		Rz: 0,
	})

	if err != nil {
		return err
	}

	return moveOutOfWay(myArm)
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

func getWristPicCorners(wristCam gostream.ImageSource, debugNumber int) ([]image.Point, image.Point, error) {
	ctx := context.TODO()
	imageSize := image.Point{}
	img, release, err := wristCam.Next(ctx)
	if err != nil {
		return nil, imageSize, err
	}
	defer release()
	imgBounds := img.Bounds()
	imageSize.X = imgBounds.Max.X
	imageSize.Y = imgBounds.Max.Y

	// wait, cause this camera sucks
	time.Sleep(500 * time.Millisecond)
	img, release, err = wristCam.Next(ctx)
	if err != nil {
		return nil, imageSize, err
	}
	defer release()

	// got picture finally
	out, corners, err := chess.FindChessCornersPinkCheat(rimage.ConvertToImageWithDepth(img))
	if err != nil {
		return nil, imageSize, err
	}

	if debugNumber >= 0 {
		if err := rimage.WriteImageToFile(fmt.Sprintf("/tmp/foo-%d-in.png", debugNumber), img); err != nil {
			panic(err)
		}
		if err := rimage.WriteImageToFile(fmt.Sprintf("/tmp/foo-%d-out.png", debugNumber), out); err != nil {
			panic(err)
		}
	}

	golog.Global.Debugf("Corners: %v", corners)

	return corners, imageSize, err
}

func lookForBoardAdjust(myArm arm.Arm, wristCam gostream.ImageSource, corners []image.Point, imageSize image.Point) error {
	debugNumber := 100
	for {
		where, err := myArm.CurrentPosition()
		if err != nil {
			return err
		}
		center := rimage.Center(corners, 10000)

		xRatio := float64(center.X) / float64(imageSize.X)
		yRatio := float64(center.Y) / float64(imageSize.Y)

		xMove := (.5 - xRatio) / 8
		yMove := (.5 - yRatio) / -8

		golog.Global.Debugf("center %v xRatio: %1.4v yRatio: %1.4v xMove: %1.4v yMove: %1.4f", center, xRatio, yRatio, xMove, yMove)

		if math.Abs(xMove) < .001 && math.Abs(yMove) < .001 {
			Center = pos{where.X, where.Y}

			// These are hard coded based on camera orientation
			Center.x += .026
			Center.y -= .073

			golog.Global.Debugf("Center: %v", Center)
			golog.Global.Debugf("a1: %v", getCoord("a1"))
			golog.Global.Debugf("h8: %v", getCoord("h8"))
			return nil
		}

		where.X += xMove
		where.Y += yMove
		err = myArm.MoveToPosition(where)
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

func lookForBoard(myArm arm.Arm, myRobot *robot.Robot) error {
	debugNumber := 0

	wristCam := myRobot.CameraByName("wristCam")
	if wristCam == nil {
		return fmt.Errorf("can't find wristCam")
	}

	for foo := -1.0; foo <= 1.0; foo += 2 {
		// HARD CODED
		where, err := myArm.CurrentPosition()
		if err != nil {
			return err
		}
		where.X = -0.42
		where.Y = 0.02
		where.Z = 0.6
		where.Rx = -2.600206
		where.Ry = -0.007839
		where.Rz = -0.061827
		err = myArm.MoveToPosition(where)
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

func adjustArmInsideSquare(robot *robot.Robot) error {
	time.Sleep(500 * time.Millisecond) // wait for camera to focus

	cam := robot.CameraByName("gripperCam")
	if cam == nil {
		return fmt.Errorf("can't find gripperCam")
	}

	arm := robot.Arms[0]

	for {
		where, err := arm.CurrentPosition()
		if err != nil {
			return err
		}
		fmt.Printf("starting at: %0.3f,%0.3f\n", where.X, where.Y)

		raw, release, err := cam.Next(context.TODO())
		if err != nil {
			return err
		}
		var dm *rimage.DepthMap
		func() {
			defer release()
			dm = rimage.ConvertToImageWithDepth(raw).Depth
		}()
		if dm == nil {
			return fmt.Errorf("no depthj on gripperCam")
		}
		//defer img.Close() // TODO(erh): fix the leak
		fmt.Printf("\t got image\n")

		center := image.Point{dm.Width() / 2, dm.Height() / 2}
		lowest, lowestValue, _, highestValue := findDepthPeaks(dm, center, 30)

		diff := highestValue - lowestValue

		if diff < 11 {
			return fmt.Errorf("no chess piece because height is only: %v", diff)
		}

		offsetX := center.X - lowest.X
		offsetY := center.Y - lowest.Y

		if utils.AbsInt(offsetX) < 3 && utils.AbsInt(offsetY) < 3 {
			fmt.Printf("success!\n")
			return nil
		}

		fmt.Printf("\t offsetX: %v offsetY: %v diff: %v\n", offsetX, offsetY, diff)

		where.X += float64(offsetX) / -2000
		where.Y += float64(offsetY) / 2000

		fmt.Printf("\t moving to %0.3f,%0.3f\n", where.X, where.Y)

		err = arm.MoveToPosition(where)
		if err != nil {
			return err
		}

		time.Sleep(500 * time.Millisecond) // wait for camera to focus
	}

}

func main() {
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	cfgFile := flag.Arg(0)

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			golog.Global.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	cfg, err := robot.ReadConfig(cfgFile)
	if err != nil {
		panic(err)
	}

	myRobot, err := robot.NewRobot(context.Background(), cfg)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := myRobot.Close(context.Background()); err != nil {
			panic(err)
		}
	}()

	if len(myRobot.Arms) == 0 {
		panic("need an arm")
	}
	myArm := myRobot.Arms[0]

	if len(myRobot.Grippers) == 0 {
		panic("need a gripper")
	}
	myGripper := myRobot.Grippers[0]

	webcam := myRobot.CameraByName("cameraOver")
	if webcam == nil {
		panic("can't find cameraOver camera")
	}

	if false { // TODO(erh): put this back once we have a wrist camera again
		err = lookForBoard(myArm, myRobot)
		if err != nil {
			panic(err)
		}
	}

	err = initArm(myArm)
	if err != nil {
		panic(err)
	}

	if false {
		fmt.Printf("ELIOT HACK\n")

		err := moveTo(myArm, "c3", 0)
		if err == nil {
			time.Sleep(500 * time.Millisecond) // wait for camera to focus
			err = adjustArmInsideSquare(myRobot)
		}

		if err != nil {
			fmt.Printf("err: %s\n", err)
		}
		os.Exit(-1)
		return
	}

	boardState := boardStateGuesser{}
	defer boardState.Clear()
	currentPosition := position.StartingPosition()

	initialPositionOk := false

	annotatedImageHolder := &rimage.StaticSource{}
	myRobot.AddCamera(annotatedImageHolder, robot.Component{})

	go func() {
		for {
			img, release, err := webcam.Next(context.TODO())
			func() {
				defer release()
				if err != nil {
					golog.Global.Debugf("error reading device: %s", err)
					return
				}

				theBoard, err := chess.FindAndWarpBoard(rimage.ConvertToImageWithDepth(img))
				if err != nil {
					golog.Global.Debug(err)
					return
				}

				annotated := theBoard.Annotate()

				if theBoard.IsBoardBlocked() {
					golog.Global.Debug("board blocked")
					boardState.Clear()
					wantPicture = 1
				} else {
					// boardState now owns theBoard
					interessting, err := boardState.newData(theBoard)
					if err != nil {
						wantPicture = 1
						golog.Global.Debug(err)
						boardState.Clear()
					} else if interessting {
						wantPicture = 1
					}
					theBoard = nil // indicate theBoard is no longer owned

					if boardState.Ready() {
						if !initialPositionOk {
							bb, err := boardState.GetBitBoard()
							if err != nil {
								golog.Global.Debug("got inconsistency reading board, let's try again")
								boardState.Clear()
							} else if currentPosition.AllOccupiedSqsBb().Value() != bb.Value() {
								golog.Global.Debugf("not in initial chess piece setup")
								bb.Print()
							} else {
								initialPositionOk = true
								golog.Global.Debugf("GOT initial chess piece setup")
							}
						} else {
							// so we've already made sure we're safe, let's see if a move was made
							m, err := boardState.GetPrevMove(currentPosition)
							if err != nil {
								// trouble reading board, let's reset
								golog.Global.Debug("got inconsistency reading board, let's try again")
								boardState.Clear()
							} else if m != nil {
								golog.Global.Debugf("we detected a move: %s", m)

								if !engine.MakeValidMove(*m, &currentPosition) {
									panic("invalid move!")
								}

								currentPosition.Print()
								currentPosition.PrintFen()

								currentPosition, m = searchForNextMove(currentPosition)
								golog.Global.Debugf("computer will make move: %s", m)
								err = movePiece(boardState, myRobot, myArm, myGripper, m.String()[0:2], m.String()[2:])
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

					}
				}

				annotatedImageHolder.Img = rimage.ConvertToImageWithDepth(annotated)
				if atomic.LoadInt32(&wantPicture) != 0 {
					tm := time.Now().Unix()

					fn := fmt.Sprintf("data/board-%d.both.gz", tm)
					golog.Global.Debugf("saving image %s", fn)
					if err := annotatedImageHolder.Img.WriteTo(fn); err != nil {
						panic(err)
					}

					atomic.StoreInt32(&wantPicture, 0)
				}
			}()
		}
	}()

	err = web.RunWeb(myRobot)
	if err != nil {
		panic(err)
	}

}
