package vforcematrixwithmux

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/sensor/forcematrix"
	"go.viam.com/rdk/testutils/inject"
)

func createExpectedMatrix(c *ForceMatrixConfig) [][]int {
	numCols := len(c.ColumnGPIOPins)
	numRows := len(c.IOPins)
	expectedMatrix := make([][]int, numRows)
	for row := 0; row < numRows; row++ {
		expectedMatrix[row] = make([]int, numCols)
		for col := 0; col < numCols; col++ {
			expectedMatrix[row][col] = row + numRows*col
		}
	}
	return expectedMatrix
}

func TestNewForceMatrix(t *testing.T) {
	logger := golog.NewTestLogger(t)

	validConfig := &ForceMatrixConfig{
		BoardName:           "board",
		ColumnGPIOPins:      []string{"io10", "io11", "io12"},
		MuxGPIOPins:         []string{"s2", "s1", "s0"},
		IOPins:              []int{1, 2},
		AnalogChannel:       "a1",
		SlipDetectionWindow: 10,
		NoiseThreshold:      5,
	}

	t.Run("valid", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeAnalogReader := &inject.AnalogReader{}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return fakeAnalogReader, true
		}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		fsm, err := newForceMatrix(context.Background(), fakeRobot, validConfig, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fsm, test.ShouldNotBeNil)
		test.That(t, fsm.columnGpioPins, test.ShouldResemble, validConfig.ColumnGPIOPins)
		test.That(t, fsm.muxGpioPins, test.ShouldResemble, validConfig.MuxGPIOPins)
		test.That(t, fsm.ioPins, test.ShouldResemble, validConfig.IOPins)
		test.That(t, fsm.analogChannel, test.ShouldEqual, validConfig.AnalogChannel)
		test.That(t, len(fsm.previousMatrices), test.ShouldBeZeroValue)
		test.That(t, fsm.slipDetectionWindow, test.ShouldEqual, validConfig.SlipDetectionWindow)
		test.That(t, fsm.noiseThreshold, test.ShouldEqual, validConfig.NoiseThreshold)
	})

	t.Run("board not found", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return nil, false
		}
		_, err := newForceMatrix(context.Background(), fakeRobot, validConfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("analog reader not found", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return nil, false
		}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		_, err := newForceMatrix(context.Background(), fakeRobot, validConfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})
}
func TestValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		validConfig := &ForceMatrixConfig{
			BoardName:           "board",
			ColumnGPIOPins:      []string{"io10", "io11", "io12"},
			MuxGPIOPins:         []string{"s2", "s1", "s0"},
			IOPins:              []int{1, 2},
			AnalogChannel:       "a1",
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := validConfig.Validate("path")
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no board", func(t *testing.T) {
		invalidConfig := &ForceMatrixConfig{
			BoardName:           "",
			ColumnGPIOPins:      []string{"io10", "io11", "io12"},
			MuxGPIOPins:         []string{"s2", "s1", "s0"},
			IOPins:              []int{1, 2},
			AnalogChannel:       "a1",
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `"board" is required`)
	})

	t.Run("no ColumnGPIOPins", func(t *testing.T) {
		invalidConfig := &ForceMatrixConfig{
			BoardName:           "board",
			ColumnGPIOPins:      []string{},
			MuxGPIOPins:         []string{"s2", "s1", "s0"},
			IOPins:              []int{1, 2},
			AnalogChannel:       "a1",
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `error validating "path"`)
	})

	t.Run("invalid MuxGPIOPins", func(t *testing.T) {
		invalidConfig := &ForceMatrixConfig{
			BoardName:           "board",
			ColumnGPIOPins:      []string{"io10", "io11", "io12"},
			MuxGPIOPins:         []string{"s1", "s0"},
			IOPins:              []int{1, 2},
			AnalogChannel:       "a1",
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `error validating "path"`)
	})

	t.Run("no IOPins", func(t *testing.T) {
		invalidConfig := &ForceMatrixConfig{
			BoardName:           "board",
			ColumnGPIOPins:      []string{"io10", "io11", "io12"},
			MuxGPIOPins:         []string{"s2", "s1", "s0"},
			IOPins:              []int{},
			AnalogChannel:       "a1",
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `error validating "path"`)
	})

	t.Run("no AnalogChannel", func(t *testing.T) {
		invalidConfig := &ForceMatrixConfig{
			BoardName:           "board",
			ColumnGPIOPins:      []string{"io10", "io11", "io12"},
			MuxGPIOPins:         []string{"s2", "s1", "s0"},
			IOPins:              []int{1, 2},
			AnalogChannel:       "",
			SlipDetectionWindow: 10,
			NoiseThreshold:      5,
		}
		err := invalidConfig.Validate("path")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, `"analog_channel" is required`)
	})

	t.Run("invalid SlipDetectionWindow", func(t *testing.T) {
		t.Run("too small", func(t *testing.T) {
			invalidConfig := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"io10", "io11", "io12"},
				MuxGPIOPins:         []string{"s2", "s1", "s0"},
				IOPins:              []int{1, 2},
				AnalogChannel:       "a1",
				SlipDetectionWindow: 0,
				NoiseThreshold:      5,
			}
			err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `error validating "path"`)
		})
		t.Run("too big", func(t *testing.T) {
			invalidConfig := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"io10", "io11", "io12"},
				MuxGPIOPins:         []string{"s2", "s1", "s0"},
				IOPins:              []int{1, 2},
				AnalogChannel:       "a1",
				SlipDetectionWindow: forcematrix.MatrixStorageSize + 1,
				NoiseThreshold:      5,
			}
			err := invalidConfig.Validate("path")
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, `error validating "path"`)
		})
	})
}

func TestSetMuxGpioPins(t *testing.T) {
	logger := golog.NewTestLogger(t)

	validConfig := &ForceMatrixConfig{
		BoardName:           "board",
		ColumnGPIOPins:      []string{"io10", "io11", "io12"},
		MuxGPIOPins:         []string{"s2", "s1", "s0"},
		IOPins:              []int{1, 2},
		AnalogChannel:       "a1",
		SlipDetectionWindow: 10,
		NoiseThreshold:      5,
	}
	t.Run("bad ioPin", func(t *testing.T) {
		fakeRobot := &inject.Robot{}
		fakeBoard := &inject.Board{}
		fakeAR := &inject.AnalogReader{}
		fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
			return fakeAR, true
		}
		fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
			return fakeBoard, true
		}
		mux, _ := newForceMatrix(context.Background(), fakeRobot, validConfig, logger)
		err := mux.setMuxGpioPins(context.Background(), -1)
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestMatrixAndSlip(t *testing.T) {
	logger := golog.NewTestLogger(t)
	t.Run("correct shape", func(t *testing.T) {
		t.Run("4x4", func(t *testing.T) {
			fakeRobot := &inject.Robot{}
			fakeBoard := &inject.Board{}

			analogValue := 0
			fakeAR := &inject.AnalogReader{}
			fakeAR.ReadFunc = func(ctx context.Context) (int, error) {
				defer func() { analogValue++ }()
				return analogValue, nil
			}
			fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
				return fakeAR, true
			}
			fakeBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
				return nil
			}
			fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
				return fakeBoard, true
			}
			config := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"1", "2", "3", "4"},
				MuxGPIOPins:         []string{"s2", "s1", "s0"},
				IOPins:              []int{0, 2, 6, 7},
				AnalogChannel:       "fake",
				SlipDetectionWindow: 2,
				NoiseThreshold:      150,
			}
			expectedMatrix := createExpectedMatrix(config)

			mux, _ := newForceMatrix(context.Background(), fakeRobot, config, logger)
			actualMatrix, err := mux.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("slip detection", func(t *testing.T) {
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				isSlipping, err := mux.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})

		t.Run("2x6", func(t *testing.T) {
			fakeRobot := &inject.Robot{}
			fakeBoard := &inject.Board{}

			fakeAR := &inject.AnalogReader{}
			analogValue := 0
			fakeAR.ReadFunc = func(ctx context.Context) (int, error) {
				defer func() { analogValue++ }()
				return analogValue, nil
			}
			fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
				return fakeAR, true
			}
			fakeBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
				return nil
			}
			fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
				return fakeBoard, true
			}
			config := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"1", "2", "3", "4", "5", "6"},
				MuxGPIOPins:         []string{"s2", "s1", "s0"},
				IOPins:              []int{0, 2},
				AnalogChannel:       "fake",
				SlipDetectionWindow: 2,
				NoiseThreshold:      150,
			}
			expectedMatrix := createExpectedMatrix(config)

			mux, _ := newForceMatrix(context.Background(), fakeRobot, config, logger)
			actualMatrix, err := mux.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, actualMatrix, test.ShouldResemble, expectedMatrix)

			t.Run("slip detection", func(t *testing.T) {
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				isSlipping, err := mux.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})

		t.Run("5x3", func(t *testing.T) {
			fakeRobot := &inject.Robot{}
			fakeBoard := &inject.Board{}

			fakeAR := &inject.AnalogReader{}
			analogValue := 0
			fakeAR.ReadFunc = func(ctx context.Context) (int, error) {
				defer func() { analogValue++ }()
				return analogValue, nil
			}
			fakeBoard.AnalogReaderByNameFunc = func(name string) (board.AnalogReader, bool) {
				return fakeAR, true
			}
			fakeBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
				return nil
			}
			fakeRobot.BoardByNameFunc = func(name string) (board.Board, bool) {
				return fakeBoard, true
			}
			config := &ForceMatrixConfig{
				BoardName:           "board",
				ColumnGPIOPins:      []string{"1", "2", "3"},
				MuxGPIOPins:         []string{"s2", "s1", "s0"},
				IOPins:              []int{0, 2, 6, 7, 3},
				AnalogChannel:       "fake",
				SlipDetectionWindow: 2,
				NoiseThreshold:      150,
			}
			expectedMatrix := createExpectedMatrix(config)

			mux, _ := newForceMatrix(context.Background(), fakeRobot, config, logger)
			matrix, err := mux.Matrix(context.Background())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, matrix, test.ShouldResemble, expectedMatrix)

			t.Run("slip detection", func(t *testing.T) {
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				mux.Matrix(context.Background())
				isSlipping, err := mux.IsSlipping(context.Background())
				test.That(t, isSlipping, test.ShouldBeFalse)
				test.That(t, err, test.ShouldBeNil)
			})
		})
	})
}
