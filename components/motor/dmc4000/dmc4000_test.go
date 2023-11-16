package dmc4000_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/dmc4000"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// check is essentially test.That with tb.Error instead of tb.Fatal (Fatal exits and leaves the go routines stuck waiting).
func check(
	resChan chan string,
	actual interface{},
	assert func(actual interface{}, expected ...interface{}) string, expected ...interface{},
) {
	if result := assert(actual, expected...); result != "" {
		resChan <- result
	}
}

var txMu sync.Mutex

func waitTx(tb testing.TB, resChan chan string) {
	tb.Helper()
	txMu.Lock()
	defer txMu.Unlock()
	for {
		res := <-resChan
		if res == "DONE" {
			return
		}
		tb.Error(res)
	}
}

func checkTx(resChan, c chan string, expects []string) {
	defer txMu.Unlock()
	for _, expected := range expects {
		tx := <-c
		check(resChan, tx, test.ShouldResemble, expected)
		c <- ":"
	}
	resChan <- "DONE"
}

func checkRx(resChan, c chan string, expects, sends []string) {
	defer txMu.Unlock()
	for i, expected := range expects {
		tx := <-c
		check(resChan, tx, test.ShouldResemble, expected)
		c <- sends[i]
	}
	resChan <- "DONE"
}

func TestDMC4000Motor(t *testing.T) {
	ctx := context.Background()
	logger, obs := logging.NewObservedTestLogger(t)
	c := make(chan string)
	resChan := make(chan string, 1024)
	deps := make(resource.Dependencies)

	mc := dmc4000.Config{
		SerialDevice:     "testchan",
		Axis:             "A",
		HomeRPM:          50,
		AmplifierGain:    3,
		LowCurrent:       -1,
		TestChan:         c,
		MaxAcceleration:  5000,
		MaxRPM:           300,
		TicksPerRotation: 200,
	}

	motorReg, ok := resource.LookupRegistration(motor.API, resource.DefaultModelFamily.WithModel("DMC4000"))
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, motorReg, test.ShouldNotBeNil)

	// These are the setup register writes
	txMu.Lock()
	go checkRx(resChan, c,
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

	m, err := motorReg.Constructor(context.Background(), deps, resource.Config{Name: "motor1", ConvertedAttributes: &mc}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{"STA", "SCA", "TEA"},
			[]string{" :", " 4\r\n:", " 0\r\n:"},
		)
		test.That(t, m.Close(context.Background()), test.ShouldBeNil)
		waitTx(t, resChan)
	}()
	motorDep, ok := m.(motor.Motor)
	test.That(t, ok, test.ShouldBeTrue)
	waitTx(t, resChan)

	t.Run("motor supports position reporting", func(t *testing.T) {
		properties, err := motorDep.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, properties.PositionReporting, test.ShouldBeTrue)
	})

	t.Run("motor SetPower testing", func(t *testing.T) {
		// Test 0 (aka "stop")
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{"STA", "SCA", "TEA"},
			[]string{" :", "4\r\n:", "0\r\n:"},
		)
		test.That(t, motorDep.SetPower(ctx, 0, nil), test.ShouldBeNil)

		// Test almost 0
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{"STA", "SCA", "TEA"},
			[]string{" :", "4\r\n:", "0\r\n:"},
		)
		test.That(t, motorDep.SetPower(ctx, 0.05, nil), test.ShouldBeNil)
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")

		// Test 0.5 of max power
		txMu.Lock()
		go checkTx(resChan, c, []string{
			"JGA=32000",
			"BGA",
		})
		test.That(t, motorDep.SetPower(ctx, 0.5, nil), test.ShouldBeNil)

		// Test -0.5 of max power
		txMu.Lock()
		go checkTx(resChan, c, []string{
			"JGA=-32000",
			"BGA",
		})
		test.That(t, motorDep.SetPower(ctx, -0.5, nil), test.ShouldBeNil)
		waitTx(t, resChan)

		// Test max power
		txMu.Lock()
		go checkTx(resChan, c, []string{
			"JGA=64000",
			"BGA",
		})
		test.That(t, motorDep.SetPower(ctx, 1, nil), test.ShouldBeNil)
		allObs = obs.All()
		latestLoggedEntry = allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")
	})

	t.Run("motor Stop testing", func(t *testing.T) {
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{"STA", "SCA", "TEA"},
			[]string{" :", " 4\r\n:", " 0\r\n:"},
		)
		test.That(t, motorDep.Stop(ctx, nil), test.ShouldBeNil)
		waitTx(t, resChan)
	})

	t.Run("motor position testing", func(t *testing.T) {
		// Check at 4.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{"RPA"},
			[]string{" 51200\r\n:"},
		)
		pos, err := motorDep.Position(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 4.0)
		waitTx(t, resChan)
	})

	t.Run("motor GoFor with positive rpm and positive revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=40960",
				"SCA",
				"TEA",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, 50.0, 3.2, nil), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=92160",
				"SCA",
				"TEA",
			},
			[]string{
				" 51200\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, 50.0, 3.2, nil), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=99840",
				"SCA",
				"TEA",
			},
			[]string{
				" 15360\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, 50.0, 6.6, nil), test.ShouldBeNil)
		waitTx(t, resChan)
	})

	t.Run("motor GoFor with negative rpm and positive revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=-40960",
				"SCA",
				"TEA",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, -50.0, 3.2, nil), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=10239",
				"SCA",
				"TEA",
			},
			[]string{
				" 51200\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, -50.0, 3.2, nil), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=15360",
				"SCA",
				"TEA",
			},
			[]string{
				" 99840\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, -50.0, 6.6, nil), test.ShouldBeNil)
		waitTx(t, resChan)
	})

	t.Run("motor GoFor with positive rpm and negative revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=-40960",
				"SCA",
				"TEA",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, 50.0, -3.2, nil), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=10239",
				"SCA",
				"TEA",
			},
			[]string{
				" 51200\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, 50.0, -3.2, nil), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=-69120",
				"SCA",
				"TEA",
			},
			[]string{
				" 15360\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, 50.0, -6.6, nil), test.ShouldBeNil)
		waitTx(t, resChan)
	})

	t.Run("motor GoFor with negative rpm and negative revolutions", func(t *testing.T) {
		// Check with position at 0.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=40960",
				"SCA",
				"TEA",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, -50.0, -3.2, nil), test.ShouldBeNil)

		// Check with position at 4.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=92160",
				"SCA",
				"TEA",
			},
			[]string{
				" 51200\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, -50.0, -3.2, nil), test.ShouldBeNil)

		// Check with position at 1.2 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"PTA=1",
				"SPA=10666",
				"PAA=99840",
				"SCA",
				"TEA",
			},
			[]string{
				" 15360\r\n:",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, -50.0, -6.6, nil), test.ShouldBeNil)
		waitTx(t, resChan)
	})

	t.Run("motor GoFor with 0 RPM", func(t *testing.T) {
		test.That(t, motorDep.GoFor(ctx, 0, 1, nil), test.ShouldBeError, motor.NewZeroRPMError())
	})

	t.Run("motor GoFor with almost 0 RPM", func(t *testing.T) {
		test.That(t, motorDep.GoFor(ctx, 0.05, 1, nil), test.ShouldBeError, motor.NewZeroRPMError())
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly 0")
	})

	t.Run("motor GoFor after jogging", func(t *testing.T) {
		// Test max power
		txMu.Lock()
		go checkTx(resChan, c, []string{
			"JGA=64000",
			"BGA",
		})
		test.That(t, motorDep.SetPower(ctx, 1, nil), test.ShouldBeNil)
		allObs := obs.All()
		latestLoggedEntry := allObs[len(allObs)-1]
		test.That(t, fmt.Sprint(latestLoggedEntry), test.ShouldContainSubstring, "nearly the max")

		// Test 0.5 of max power
		txMu.Lock()
		go checkTx(resChan, c, []string{
			"JGA=32000",
			"BGA",
		})
		test.That(t, motorDep.SetPower(ctx, 0.5, nil), test.ShouldBeNil)

		// Check with position at 0.0 revolutions
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"RPA",
				"STA",
				"PTA=1",
				"SPA=10666",
				"PAA=40960",
				"SCA",
				"TEA",
			},
			[]string{
				" 0\r\n:",
				":",
				":",
				":",
				":",
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		test.That(t, motorDep.GoFor(ctx, -50.0, -3.2, nil), test.ShouldBeNil)
		waitTx(t, resChan)
	})

	t.Run("motor is on testing", func(t *testing.T) {
		// Off - StopCode != special cases, TotalError = 0
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"SCA",
				"TEA",
			},
			[]string{
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		on, powerPct, err := motorDep.IsPowered(ctx, nil)
		test.That(t, on, test.ShouldEqual, false)
		test.That(t, powerPct, test.ShouldEqual, 0.5)
		test.That(t, err, test.ShouldBeNil)

		// On - TE != 0
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"SCA",
				"TEA",
			},
			[]string{
				" 4\r\n:",
				" 5\r\n:",
			},
		)
		on, powerPct, err = motorDep.IsPowered(ctx, nil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, 0.5)
		test.That(t, err, test.ShouldBeNil)

		// On - StopCodes = sepecial cases
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"SCA",
			},
			[]string{
				" 0\r\n:",
			},
		)
		on, powerPct, err = motorDep.IsPowered(ctx, nil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, 0.5)
		test.That(t, err, test.ShouldBeNil)

		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"SCA",
			},
			[]string{
				" 30\r\n:",
			},
		)
		on, powerPct, err = motorDep.IsPowered(ctx, nil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, 0.5)
		test.That(t, err, test.ShouldBeNil)

		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"SCA",
			},
			[]string{
				" 50\r\n:",
			},
		)
		on, powerPct, err = motorDep.IsPowered(ctx, nil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, 0.5)
		test.That(t, err, test.ShouldBeNil)

		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"SCA",
			},
			[]string{
				" 60\r\n:",
			},
		)
		on, powerPct, err = motorDep.IsPowered(ctx, nil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, 0.5)
		test.That(t, err, test.ShouldBeNil)

		txMu.Lock()
		go checkRx(resChan, c,
			[]string{
				"SCA",
			},
			[]string{
				" 100\r\n:",
			},
		)
		on, powerPct, err = motorDep.IsPowered(ctx, nil)
		test.That(t, on, test.ShouldEqual, true)
		test.That(t, powerPct, test.ShouldEqual, 0.5)
		test.That(t, err, test.ShouldBeNil)
		waitTx(t, resChan)
	})

	t.Run("motor zero testing", func(t *testing.T) {
		// No offset (and when actually off)
		txMu.Lock()
		go checkTx(resChan, c, []string{"DPA=0"})
		test.That(t, motorDep.ResetZeroPosition(ctx, 0, nil), test.ShouldBeNil)

		// 3.1 offset (and when actually off)
		txMu.Lock()
		go checkTx(resChan, c, []string{"DPA=39680"})
		test.That(t, motorDep.ResetZeroPosition(ctx, 3.1, nil), test.ShouldBeNil)
		waitTx(t, resChan)
	})

	t.Run("motor do raw command", func(t *testing.T) {
		txMu.Lock()
		go checkRx(resChan, c,
			[]string{"testTX"},
			[]string{" testRX\r\n:"},
		)
		resp, err := motorDep.DoCommand(ctx, map[string]interface{}{"command": "raw", "raw_input": "testTX"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["return"], test.ShouldEqual, "testRX")
		waitTx(t, resChan)
	})

	t.Run("motor do home command", func(t *testing.T) {
		txMu.Lock()
		go checkRx(resChan, c,
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
				"SCA",
				"TEA",
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
				" 4\r\n:",
				" 0\r\n:",
			},
		)
		resp, err := motorDep.DoCommand(ctx, map[string]interface{}{"command": "home"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldBeNil)
		waitTx(t, resChan)
	})
}
