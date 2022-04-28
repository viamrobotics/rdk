package dmc4000_test

import (
	"context"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/component/motor/dmc4000"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/testutils/inject"
)

// check is essentially test.That with tb.Error instead of tb.Fatal (Fatal exits and leaves the go routines stuck waiting).
func check(tb testing.TB, actual interface{}, assert func(actual interface{}, expected ...interface{}) string, expected ...interface{}) {
	tb.Helper()
	if result := assert(actual, expected...); result != "" {
		tb.Error(result)
	}
}

var txMu sync.Mutex

func waitTx() {
	txMu.Lock()
	defer txMu.Unlock()
}

func checkTx(t *testing.T, c chan string, expects []string) {
	t.Helper()
	defer txMu.Unlock()
	for _, expected := range expects {
		tx := <-c
		check(t, tx, test.ShouldResemble, expected)
		c <- ":"
	}
}

func checkRx(t *testing.T, c chan string, expects []string, sends []string) {
	t.Helper()
	defer txMu.Unlock()
	for i, expected := range expects {
		tx := <-c
		check(t, tx, test.ShouldResemble, expected)
		c <- sends[i]
	}
}

func TestDMC4000Motor(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	c := make(chan string)
	r := inject.Robot{}
	// r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
	// 	return b, nil
	// }

	mc := dmc4000.Config{
		SerialDevice:  "testchan",
		Axis:          "A",
		HomeRPM:       50,
		AmplifierGain: 3,
		LowCurrent:    -1,
		TestChan:      c,
		Config: motor.Config{
			MaxAcceleration:  5000,
			MaxRPM:           300,
			TicksPerRotation: 200,
		},
	}

	motorReg := registry.ComponentLookup(motor.Subtype, "DMC4000")
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	txMu.Lock()
	go checkRx(t, c,
		[]string{
			"EO 0",
			"ID",
			"MOA",
			"MTA=2",
			"AGA=3",
			"LCA=-1",
			"ACA=1066666",
			"DCA=1066666",
			"SHA",
		},
		[]string{
			":",
			"FW, DMC4183 Rev 1.3h\r\nDMC, 4103, Rev 11\r\nAMP1, 44140, Rev 3\r\nAMP2, 44140, Rev 3\r\n:",
			":",
			":",
			":",
			":",
			":",
			":",
			":",
		},
	)

	m, err := motorReg.Constructor(context.Background(), &r, config.Component{Name: "motor1", ConvertedAttributes: &mc}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		txMu.Lock()
		go checkTx(t, c, []string{
			"STA",
		})
		test.That(t, utils.TryClose(context.Background(), m), test.ShouldBeNil)
		waitTx()
	}()
	_motor, ok := m.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
	stoppableMotor, ok := _motor.(motor.LocalMotor)
	test.That(t, ok, test.ShouldBeTrue)

	waitTx()

	t.Run("motor supports position reporting", func(t *testing.T) {
		features, err := _motor.GetFeatures(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test power should fail
		test.That(t, _motor.SetPower(ctx, 0.5), test.ShouldNotBeNil)

		// Test 0 (aka "stop")
		txMu.Lock()
		go checkTx(t, c, []string{
			"STA",
		})
		test.That(t, _motor.SetPower(ctx, 0), test.ShouldBeNil)
		waitTx()
	})

	t.Run("motor Stop testing", func(t *testing.T) {
		txMu.Lock()
		go checkTx(t, c, []string{
			"STA",
		})
		test.That(t, _motor.Stop(ctx), test.ShouldBeNil)
		waitTx()
	})

	t.Run("motor position testing", func(t *testing.T) {
		// Check at 4.0 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{"RPA"},
			[]string{" 51200\r\n:"},
		)
		pos, err := _motor.GetPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 4.0)
		waitTx()
	})

	t.Run("motor GoFor with positive rpm and positive revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=40960",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, 3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=92160",
			},
			[]string{
				" 51200\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, 3.2), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=99840",
			},
			[]string{
				" 15360\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, 6.6), test.ShouldBeNil)
		waitTx()
	})

	t.Run("motor GoFor with negative rpm and positive revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=-40960",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, 3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=10239",
			},
			[]string{
				" 51200\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, 3.2), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=15360",
			},
			[]string{
				" 99840\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, 6.6), test.ShouldBeNil)
		waitTx()
	})

	t.Run("motor GoFor with positive rpm and negative revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=-40960",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, -3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=10239",
			},
			[]string{
				" 51200\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, -3.2), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=-69120",
			},
			[]string{
				" 15360\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, 50.0, -6.6), test.ShouldBeNil)
		waitTx()
	})

	t.Run("motor GoFor with negative rpm and negative revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=40960",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, -3.2), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=92160",
			},
			[]string{
				" 51200\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, -3.2), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=99840",
			},
			[]string{
				" 15360\r\n:",
				":",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, _motor.GoFor(ctx, -50.0, -6.6), test.ShouldBeNil)
		waitTx()
	})

	t.Run("motor is on testing", func(t *testing.T) {
		// Off - StopCode != special cases, TotalError = 0
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"SCA",
				"TEA",
			},
			[]string{
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		on, err := _motor.IsPowered(ctx)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, err, test.ShouldBeNil)

		// On - TE != 0
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"SCA",
				"TEA",
			},
			[]string{
				" 4\r\n:",
				" 5\r\n:",
			},
		)
		on, err = _motor.IsPowered(ctx)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, err, test.ShouldBeNil)

		// On - StopCodes = sepecial cases
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"SCA",
			},
			[]string{
				" 0\r\n:",
			},
		)
		on, err = _motor.IsPowered(ctx)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, err, test.ShouldBeNil)

		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"SCA",
			},
			[]string{
				" 30\r\n:",
			},
		)
		on, err = _motor.IsPowered(ctx)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, err, test.ShouldBeNil)

		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"SCA",
			},
			[]string{
				" 50\r\n:",
			},
		)
		on, err = _motor.IsPowered(ctx)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, err, test.ShouldBeNil)

		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"SCA",
			},
			[]string{
				" 60\r\n:",
			},
		)
		on, err = _motor.IsPowered(ctx)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, err, test.ShouldBeNil)

		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"SCA",
			},
			[]string{
				" 100\r\n:",
			},
		)
		on, err = _motor.IsPowered(ctx)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, err, test.ShouldBeNil)
		waitTx()
	})

	t.Run("motor zero testing", func(t *testing.T) {
		// No offset (and when actually off)
		txMu.Lock()
		go checkTx(t, c, []string{"DPA=0"})
		test.That(t, _motor.ResetZeroPosition(ctx, 0), test.ShouldBeNil)

		// 3.1 offset (and when actually off)
		txMu.Lock()
		go checkTx(t, c, []string{"DPA=39680"})
		test.That(t, _motor.ResetZeroPosition(ctx, 3.1), test.ShouldBeNil)
		waitTx()
	})

	t.Run("motor gotillstop testing", func(t *testing.T) {
		// No stop func
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=5333",
				"PAA=-128000000",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"TEA",
				"STA",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 4\r\n:",
				" 0\r\n:",
				":",
			},
		)
		test.That(t, stoppableMotor.GoTillStop(ctx, -25.0, nil), test.ShouldBeNil)

		// Always-false stopFunc
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=5333",
				"PAA=-128000000",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"TEA",
				"STA",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 4\r\n:",
				" 0\r\n:",
				":",
			},
		)
		test.That(t, stoppableMotor.GoTillStop(ctx, -25.0, func(ctx context.Context) bool { return false }), test.ShouldBeNil)

		// Always true stopFunc
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=5333",
				"PAA=-128000000",
				"STA",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				":",
			},
		)
		test.That(t, stoppableMotor.GoTillStop(ctx, -25.0, func(ctx context.Context) bool { return true }), test.ShouldBeNil)
		waitTx()
	})

	t.Run("motor do raw command", func(t *testing.T) {
		txMu.Lock()
		go checkRx(t, c,
			[]string{"testTX"},
			[]string{" testRX\r\n:"},
		)
		resp, err := _motor.Do(ctx, map[string]interface{}{"command": "raw", "raw_input": "testTX"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["return"], test.ShouldEqual, "testRX")
		waitTx()
	})

	t.Run("motor do home command", func(t *testing.T) {
		txMu.Lock()
		go checkRx(t, c,
			[]string{
				"SPA=10666",
				"HVA=1066",
				"HMA",
				"BGA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"SCA",
				"STA",
			},
			[]string{
				":",
				":",
				":",
				":",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 0\r\n:",
				" 10\r\n:",
				":",
			},
		)
		resp, err := _motor.Do(ctx, map[string]interface{}{"command": "home"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldBeNil)
		waitTx()
	})
}
