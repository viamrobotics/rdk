package robotimpl

import (
	"testing"
)

// func ConfigFromFile(t *testing.T, filePath string) *config.Config {
//  	t.Helper()
//  	logger := golog.NewTestLogger(t)
//  	buf, err := ioutil.ReadFile(filePath)
//  	test.That(t, err, test.ShouldBeNil)
//  	conf, err := config.FromReader(context.Background(), filePath, bytes.NewReader(buf), logger)
//  	test.That(t, err, test.ShouldBeNil)
//  	return conf
// }

// func NoErr[ResultT any](t *testing.T, lambda func() (ResultT, error)) ResultT {
//  	var (
//  		res ResultT
//  		err error
//  	)
//  	res, err = lambda()
//  	test.That(t, err, test.ShouldBeNil)
//  	return res
// }
//
// robot := NoErr(t, func() (robot.LocalRobot, error) { return New(ctx, conf, logger) })

func TestCancelDependentOps(t *testing.T) {
	// logger := golog.NewTestLogger(t)
	// conf := ConfigFromFile(t, "./data/motor_base_robot.json")
	// mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	// _ = mockAPI
	//
	// ctx := context.Background()
	// robot, err := New(ctx, conf, logger)
	// test.That(t, err, test.ShouldBeNil)
	// defer robot.Close(ctx)
	//
	// // fmt.Println("Resources:", robot.ResourceNames())
	//
	// // Get handles on the robot pieces for the base, left and right motors.
	// baseClientProd, err := base.FromRobot(robot, "base1")
	// baseClient := baseClientProd.(base.IBase)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, baseClient, test.ShouldNotBeNil)
	//
	// m1ClientProd, err := motor.FromRobot(robot, "m1")
	// m1Client := m1ClientProd.(*fake.Motor)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, m1Client, test.ShouldNotBeNil)
	//
	// m2ClientProd, err := motor.FromRobot(robot, "m2")
	// m2Client := m2ClientProd.(*fake.Motor)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, m2Client, test.ShouldNotBeNil)
	//
	// // Assert nothing is moving.
	// isMoving, err := baseClient.IsMoving(ctx)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, isMoving, test.ShouldBeFalse)
	//
	// m1Powered, _, err := m1Client.IsPowered(ctx, nil)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, m1Powered, test.ShouldBeFalse)
	//
	// m2Powered, _, err := m2Client.IsPowered(ctx, nil)
	// test.That(t, err, test.ShouldBeNil)
	// test.That(t, m2Powered, test.ShouldBeFalse)
	//
	// wg := sync.WaitGroup{}
	// defer wg.Wait()
	// // Start two goroutines. One will continually tell the robot to go forward.  The other will
	// // continually tell the robot to go backward. Each API call will cancel the other.
	// wg.Add(1)
	// go func() {
	//  	defer wg.Done()
	//  	//start := time.Now()
	//  	// for time.Since(start) < 10*time.Second {
	//  	err := baseClient.MoveStraight(ctx, 1, 1, nil)
	//  	if err != nil {
	//  		panic(err)
	//  	}
	//  	// }
	// }()
	//
	// wg.Add(1)
	// go func() {
	//  	defer wg.Done()
	//  	//start := time.Now()
	//  	//for time.Since(start) < 10*time.Second {
	//  	err := baseClient.MoveStraight(ctx, -1, 1, nil)
	//  	if err != nil {
	//  		panic(err)
	//  	}
	//  	//}
	// }()
	//
	// // Start a third goroutine to perform atomic reads of the motor settings.  Return those readings
	// // over a channel to the test thread.
	// // type Reading struct {
	// //  	left, right int
	// // }
	// // readings := make(chan Reading, 10)
	// // wg.Add(1)
	// // go func() {
	// //  	defer wg.Done()
	// //  	start := time.Now()
	// //  	ctx := context.Background()
	// //  	numReadings := 0
	// //  	for time.Since(start) < 1*time.Second {
	// //  		left, right := baseClient.ReadMotors(ctx)
	// //  		readings <- Reading{left, right}
	// //  		numReadings++
	// //  	}
	// //
	// //  	fmt.Println("NumReadings:", numReadings)
	// //  	close(readings)
	// // }()
	// //
	// // // If the "move forward" and "move backward" operations are serializable, any synchronized reading of the motors should have them agree
	// // for reading := range readings {
	// //  	test.That(t, reading.left, test.ShouldEqual, reading.right)
	// // }
	// wg.Done()
	// baseClient.ReadMotors(ctx)
	// // fmt.Printf("Left: %v Right: %v\n", left, right)
	//
	// test.That(t, robot.Close(ctx), test.ShouldBeNil)
}

/*
	// Move the robot "forward". This should set both motors on.
	test.That(t, baseClient.SetPower(ctx, r3.Vector{Y: 1}, r3.Vector{}, nil), test.ShouldBeNil)
	isMoving, err = baseClient.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isMoving, test.ShouldBeTrue)

	m1Powered, _, err = m1Client.IsPowered(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m1Powered, test.ShouldBeTrue)

	m2Powered, _, err = m2Client.IsPowered(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m2Powered, test.ShouldBeTrue)

	// Move the robot "backwards".
	test.That(t, baseClient.SetPower(ctx, r3.Vector{Y: -1}, r3.Vector{}, nil), test.ShouldBeNil)
	isMoving, err = baseClient.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isMoving, test.ShouldBeTrue)

	m1Powered, _, err = m1Client.IsPowered(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m1Powered, test.ShouldBeTrue)

	m2Powered, _, err = m2Client.IsPowered(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m2Powered, test.ShouldBeTrue)

	// Stop moving the robot.
	test.That(t, baseClient.SetPower(ctx, r3.Vector{Y: 0}, r3.Vector{}, nil), test.ShouldBeNil)
	isMoving, err = baseClient.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isMoving, test.ShouldBeFalse)

	m1Powered, _, err = m1Client.IsPowered(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m1Powered, test.ShouldBeFalse)

	m2Powered, _, err = m2Client.IsPowered(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m2Powered, test.ShouldBeFalse)
*/
