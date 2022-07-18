// Package main is a chess game featuring a robot versus a human.
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

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"github.com/tonyOreglia/glee/pkg/engine"
	"github.com/tonyOreglia/glee/pkg/moves"
	"github.com/tonyOreglia/glee/pkg/position"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/chess"
)

type pos struct {
	x int64
	y int64
}

// TODO.
var (
	BoardWidth     = int64(381)
	Center         = pos{-435, 0}
	BoardHeight    = -230.0
	SafeMoveHeight = BoardHeight + 150

	wantPicture = int32(0)

	numPiecesCaptured = 0
	logger            = golog.NewDevelopmentLogger("chess")
)

func getCoord(chess string) pos {
	x := float64(chess[0] - 'a')
	y := float64(chess[1] - '1')

	if x < 0 || x > 7 || y < 0 || y > 7 {
		panic(errors.Errorf("invalid position: %s", chess))
	}

	x = (3.5 - x) / 7.0
	y = (3.5 - y) / 7.0

	return pos{Center.x + int64((x * float64(BoardWidth))), Center.y + int64((y * float64(BoardWidth)))} // HARD CODED
}

func moveTo(ctx context.Context, myArm arm.Arm, chess string, heightModMillis int64) error {
	// first make sure in safe position
	where, err := myArm.GetEndPosition(ctx, nil)
	if err != nil {
		return err
	}
	where.Z = SafeMoveHeight + float64(heightModMillis)
	err = myArm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)
	if err != nil {
		return err
	}

	// move
	if chess == "-" {
		f := getCoord("a8")
		where.X = float64(f.x - int64(60*numPiecesCaptured)) // HARD CODED
		where.Y = float64(f.y - (BoardWidth / 5))            // HARD CODED
		numPiecesCaptured++
	} else {
		f := getCoord(chess)
		where.X = float64(f.x)
		where.Y = float64(f.y)
	}
	return myArm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)
}

func movePiece(
	ctx context.Context,
	boardState boardStateGuesser,
	robot robot.Robot,
	myArm arm.Arm,
	myGripper gripper.Gripper,
	from, to string,
) error {
	if to[0] != '-' {
		toHeight, err := boardState.game.GetPieceHeight(boardState.NewestBoard(), to)
		if err != nil {
			return err
		}
		if toHeight > 0 {
			logger.Debugf("moving piece from %s to %s but occupied, going to capture", from, to)
			err = movePiece(ctx, boardState, robot, myArm, myGripper, to, "-")
			if err != nil {
				return err
			}
		}
	}

	err := moveTo(ctx, myArm, from, 0)
	if err != nil {
		return err
	}

	// open before going down
	err = myGripper.Open(ctx)
	if err != nil {
		return err
	}

	err = adjustArmInsideSquare(ctx, robot)
	if err != nil {
		return err
	}

	height := boardState.NewestBoard().SquareCenterHeight(from, 35) // TODO(erh): change to something more intelligent
	where, err := myArm.GetEndPosition(ctx, nil)
	if err != nil {
		return err
	}
	where.Z = BoardHeight + height + 10
	myArm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)

	// grab piece
	for {
		grabbedSomething, err := myGripper.Grab(ctx)
		if err != nil {
			return err
		}
		if grabbedSomething {
			logger.Debugf("got a piece at height %f", where.Z)
			// got the piece
			break
		}
		err = myGripper.Open(ctx)
		if err != nil {
			return err
		}
		logger.Debug("no piece")
		where, err = myArm.GetEndPosition(ctx, nil)
		if err != nil {
			return err
		}
		where.Z -= 10
		if where.Z <= BoardHeight {
			return errors.New("no piece")
		}
		myArm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)
	}

	saveZ := where.Z // save the height to bring the piece down to

	if to == "-throw" {
		err = moveOutOfWay(ctx, myArm)
		if err != nil {
			return err
		}

		goutils.PanicCapturingGo(func() {
			if !goutils.SelectContextOrWait(ctx, 200*time.Millisecond) {
				return
			}
			myGripper.Open(ctx)
		})
		err = moveJointDelta(ctx, myArm, 4, -1)
		if err != nil {
			return err
		}

		return initArm(ctx, myArm) // this is to get joint position right
	}

	err = moveTo(ctx, myArm, to, 100)
	if err != nil {
		return err
	}

	// drop piece
	where, err = myArm.GetEndPosition(ctx, nil)
	if err != nil {
		return err
	}

	where.Z = saveZ
	myArm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)
	myGripper.Open(ctx)

	if to != "-" {
		where, err = myArm.GetEndPosition(ctx, nil)
		if err != nil {
			return err
		}
		where.Z = SafeMoveHeight
		myArm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)
		moveOutOfWay(ctx, myArm)
	}
	return nil
}

func moveOutOfWay(ctx context.Context, myArm arm.Arm) error {
	foo := getCoord("a1")

	where, err := myArm.GetEndPosition(ctx, nil)
	if err != nil {
		return err
	}
	where.X = float64(foo.x)
	where.Y = float64(foo.y)
	where.Z = SafeMoveHeight + 300 // HARD CODED

	return myArm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)
}

func moveJointDelta(ctx context.Context, myArm arm.Arm, joint int, degAngle float64) error {
	joints, err := myArm.GetJointPositions(ctx, nil)
	if err != nil {
		return err
	}
	joints.Values[joint] += degAngle
	return myArm.MoveToJointPositions(ctx, joints, nil)
}

func initArm(ctx context.Context, myArm arm.Arm) error {
	foo := getCoord("a1")
	target := &commonpb.Pose{
		X:     float64(foo.x),
		Y:     float64(foo.y),
		Z:     SafeMoveHeight,
		Theta: math.Pi / 2,
		OX:    1,
		OY:    0,
		OZ:    0,
	}
	err := myArm.MoveToPosition(ctx, target, &commonpb.WorldState{}, nil)
	if err != nil {
		return err
	}

	return moveOutOfWay(ctx, myArm)
}

func searchForNextMove(p *position.Position) (*position.Position, *moves.Move) {
	// mvs := generate.GenerateMoves(p)
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

func getWristPicCorners(ctx context.Context, wristCam gostream.ImageSource, debugNumber int) ([]image.Point, image.Point, error) {
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
	if !goutils.SelectContextOrWait(ctx, 500*time.Millisecond) {
		return nil, imageSize, ctx.Err()
	}
	img, release, err = wristCam.Next(ctx)
	if err != nil {
		return nil, imageSize, err
	}
	defer release()

	// got picture finally
	out, corners, err := chess.FindChessCornersPinkCheat(rimage.ConvertImage(img), logger)
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

	logger.Debugf("Corners: %v", corners)

	return corners, imageSize, err
}

func lookForBoardAdjust(
	ctx context.Context,
	myArm arm.Arm,
	wristCam gostream.ImageSource,
	corners []image.Point,
	imageSize image.Point,
) error {
	debugNumber := 100
	for {
		where, err := myArm.GetEndPosition(ctx, nil)
		if err != nil {
			return err
		}
		center := rimage.Center(corners, 10000)

		xRatio := float64(center.X) / float64(imageSize.X)
		yRatio := float64(center.Y) / float64(imageSize.Y)

		xMove := (.5 - xRatio) / 8
		yMove := (.5 - yRatio) / -8

		logger.Debugf("center %v xRatio: %1.4v yRatio: %1.4v xMove: %1.4v yMove: %1.4f", center, xRatio, yRatio, xMove, yMove)

		if math.Abs(xMove) < .001 && math.Abs(yMove) < .001 {
			Center = pos{int64(where.X), int64(where.Y)}

			// These are hard coded based on camera orientation
			Center.x += 26
			Center.y -= 73

			logger.Debugf("Center: %v", Center)
			logger.Debugf("a1: %v", getCoord("a1"))
			logger.Debugf("h8: %v", getCoord("h8"))
			return nil
		}

		where.X += xMove * 1000
		where.Y += yMove * 1000
		err = myArm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)
		if err != nil {
			return err
		}

		corners, _, err = getWristPicCorners(ctx, wristCam, debugNumber)
		debugNumber++
		if err != nil {
			return err
		}
	}
}

func lookForBoard(ctx context.Context, myArm arm.Arm, myRobot robot.Robot) error {
	debugNumber := 0

	wristCam, err := camera.FromRobot(myRobot, "wristCam")
	if err != nil {
		return err
	}

	for foo := -1.0; foo <= 1.0; foo += 2 {
		// HARD CODED
		where, err := myArm.GetEndPosition(ctx, nil)
		if err != nil {
			return err
		}
		where.X = -420
		where.Y = 20
		where.Z = 600
		// Note, these are no longer accurate and should be redone. They probably need a theta
		where.OX = -2.600206
		where.OY = -0.007839
		where.OZ = -0.061827
		err = myArm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)
		if err != nil {
			return err
		}

		d := .1
		for i := 0.0; i < 1.6; i += d {
			err = moveJointDelta(ctx, myArm, 0, foo*d)
			if err != nil {
				return err
			}

			corners, imageSize, err := getWristPicCorners(ctx, wristCam, debugNumber)
			debugNumber++
			if err != nil {
				return err
			}

			if len(corners) == 4 {
				return lookForBoardAdjust(ctx, myArm, wristCam, corners, imageSize)
			}
		}
	}

	return nil
}

func adjustArmInsideSquare(ctx context.Context, robot robot.Robot) error {
	// wait for camera to focus
	if !goutils.SelectContextOrWait(ctx, 500*time.Millisecond) {
		return ctx.Err()
	}

	cam, err := camera.FromRobot(robot, "gripperCam")
	if err != nil {
		return err
	}

	arm, err := arm.FromRobot(robot, "pieceArm")
	if err != nil {
		return err
	}

	for {
		where, err := arm.GetEndPosition(ctx, nil)
		if err != nil {
			return err
		}
		rlog.Logger.Infof("starting at: %v,%v\n", where.X, where.Y)

		raw, release, err := cam.Next(ctx)
		if err != nil {
			return err
		}
		var dm *rimage.DepthMap
		func() {
			defer release()
			dm = rimage.ConvertToImageWithDepth(raw).Depth
		}()
		if dm == nil {
			return errors.New("no depth on gripperCam")
		}
		// defer img.Close() // TODO(erh): fix the leak
		logger.Debug("\t got image")

		center := image.Point{dm.Width() / 2, dm.Height() / 2}
		lowest, lowestValue, _, highestValue := findDepthPeaks(dm, center, 30)

		diff := highestValue - lowestValue

		if diff < 11 {
			return errors.Errorf("no chess piece because height is only: %v", diff)
		}

		offsetX := center.X - lowest.X
		offsetY := center.Y - lowest.Y

		if utils.AbsInt(offsetX) < 3 && utils.AbsInt(offsetY) < 3 {
			logger.Debug("success!")
			return nil
		}

		rlog.Logger.Infof("\t offsetX: %v offsetY: %v diff: %v\n", offsetX, offsetY, diff)

		where.X += float64(offsetX / -2)
		where.Y += float64(offsetY / 2)

		rlog.Logger.Infof("\t moving to %v,%v\n", where.X, where.Y)

		err = arm.MoveToPosition(ctx, where, &commonpb.WorldState{}, nil)
		if err != nil {
			return err
		}

		// wait for camera to focus
		if !goutils.SelectContextOrWait(ctx, 500*time.Millisecond) {
			return ctx.Err()
		}
	}
}

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			return err
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	cfgFile := flag.Arg(0)
	cfg, err := config.Read(ctx, cfgFile, logger)
	if err != nil {
		return err
	}
	myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(myRobot.Close(context.Background()))
	}()

	myArm, err := arm.FromRobot(myRobot, "pieceArm")
	if err != nil {
		return err
	}

	myGripper, err := gripper.FromRobot(myRobot, "grippie")
	if err != nil {
		return err
	}

	webcam, err := camera.FromRobot(myRobot, "cameraOver")
	if err != nil {
		return err
	}

	if false { // TODO(erh): put this back once we have a wrist camera again
		err = lookForBoard(ctx, myArm, myRobot)
		if err != nil {
			return err
		}
	}

	err = initArm(ctx, myArm)
	if err != nil {
		return err
	}

	if false {
		logger.Debug("ELIOT HACK")

		err = moveTo(ctx, myArm, "c3", 0)
		if err == nil {
			// wait for camera to focus
			if !goutils.SelectContextOrWait(ctx, 500*time.Millisecond) {
				return
			}
			err = adjustArmInsideSquare(ctx, myRobot)
		}

		return
	}

	boardState := boardStateGuesser{}
	defer boardState.Clear()
	currentPosition := position.StartingPosition()

	initialPositionOk := false

	goutils.PanicCapturingGo(func() {
		for {
			img, release, err := webcam.Next(ctx)
			func() {
				defer release()
				if err != nil {
					logger.Debugf("error reading device: %s", err)
					return
				}
				// TODO(DATA-237): .both will be removed
				im := rimage.ConvertImage(img)
				dm, _ := rimage.ConvertImageToDepthMap(img) // depth map optional
				theBoard, err := chess.FindAndWarpBoard(im, dm, logger)
				if err != nil {
					logger.Debug(err)
					return
				}

				annotated := theBoard.Annotate()

				if theBoard.IsBoardBlocked() {
					logger.Debug("board blocked")
					boardState.Clear()
					wantPicture = 1
				} else {
					// boardState now owns theBoard
					interessting, err := boardState.newData(theBoard)
					if err != nil {
						wantPicture = 1
						logger.Debug(err)
						boardState.Clear()
					} else if interessting {
						wantPicture = 1
					}

					if boardState.Ready() {
						if !initialPositionOk {
							bb, err := boardState.GetBitBoard()
							switch {
							case err != nil:
								logger.Debug("got inconsistency reading board, let's try again")
								boardState.Clear()
							case currentPosition.AllOccupiedSqsBb().Value() != bb.Value():
								logger.Debug("not in initial chess piece setup")
								bb.Print()
							default:
								initialPositionOk = true
								logger.Debug("GOT initial chess piece setup")
							}
						} else {
							// so we've already made sure we're safe, let's see if a move was made
							m, err := boardState.GetPrevMove(currentPosition)
							if err != nil {
								if !errors.Is(err, errNoMove) {
									// trouble reading board, let's reset
									logger.Debug("got inconsistency reading board, let's try again")
									boardState.Clear()
								}
							} else {
								logger.Debugf("we detected a move: %s", m)

								if !engine.MakeValidMove(*m, &currentPosition) {
									panic("invalid move!")
								}

								currentPosition.Print()
								currentPosition.PrintFen()

								currentPosition, m = searchForNextMove(currentPosition)
								logger.Debugf("computer will make move: %s", m)
								err = movePiece(ctx, boardState, myRobot, myArm, myGripper, m.String()[0:2], m.String()[2:])
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

				iwd := rimage.ConvertToImageWithDepth(annotated)
				if atomic.LoadInt32(&wantPicture) != 0 {
					tm := time.Now().Unix()

					fn := artifact.MustNewPath(fmt.Sprintf("samples/chess/board-%d.both.gz", tm))
					logger.Debugf("saving image %s", fn)
					if err := iwd.WriteTo(fn); err != nil {
						panic(err)
					}

					atomic.StoreInt32(&wantPicture, 0)
				}
			}()
		}
	})
	return web.RunWebWithConfig(ctx, myRobot, cfg, logger)
}
