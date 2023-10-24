package robot_test

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	echopb "go.viam.com/api/component/testecho/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/robottestutils"
)

var someBaseName1 = base.Named("base1")

var echoAPI = resource.APINamespaceRDK.WithComponentType("echo")

func init() {
	resource.RegisterAPI(echoAPI, resource.APIRegistration[resource.Resource]{
		RPCServiceServerConstructor: func(apiResColl resource.APIResourceCollection[resource.Resource]) interface{} {
			return &echoServer{coll: apiResColl}
		},
		RPCServiceHandler: echopb.RegisterTestEchoServiceHandlerFromEndpoint,
		RPCServiceDesc:    &echopb.TestEchoService_ServiceDesc,
		RPCClient: func(
			ctx context.Context,
			conn rpc.ClientConn,
			remoteName string,
			name resource.Name,
			logger logging.ZapCompatibleLogger,
		) (resource.Resource, error) {
			return NewClientFromConn(ctx, conn, remoteName, name, logger), nil
		},
	})
}

func TestSessions(t *testing.T) {
	for _, windowSize := range []time.Duration{
		config.DefaultSessionHeartbeatWindow,
		time.Second * 5,
	} {
		t.Run(fmt.Sprintf("window size=%s", windowSize), func(t *testing.T) {
			logger := logging.NewTestLogger(t)

			stopChs := map[string]*StopChan{
				"motor1": {make(chan struct{}), "motor1"},
				"motor2": {make(chan struct{}), "motor2"},
				"echo1":  {make(chan struct{}), "echo1"},
				"base1":  {make(chan struct{}), "base1"},
			}
			stopChNames := make([]string, 0, len(stopChs))
			for name := range stopChs {
				stopChNames = append(stopChNames, name)
			}

			ensureStop := makeEnsureStop(stopChs)

			motor1Name := motor.Named("motor1")
			motor2Name := motor.Named("motor2")
			base1Name := base.Named("base1")
			echo1Name := resource.NewName(echoAPI, "echo1")

			model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
			streamModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
			dummyMotor1 := dummyMotor{Named: motor1Name.AsNamed(), stopCh: stopChs["motor1"].Chan}
			dummyMotor2 := dummyMotor{Named: motor2Name.AsNamed(), stopCh: stopChs["motor2"].Chan}
			dummyEcho1 := dummyEcho{
				Named:  echo1Name.AsNamed(),
				stopCh: stopChs["echo1"].Chan,
			}
			dummyBase1 := dummyBase{Named: base1Name.AsNamed(), stopCh: stopChs["base1"].Chan}
			resource.RegisterComponent(
				motor.API,
				model,
				resource.Registration[motor.Motor, resource.NoNativeConfig]{Constructor: func(
					ctx context.Context,
					deps resource.Dependencies,
					conf resource.Config,
					logger logging.ZapCompatibleLogger,
				) (motor.Motor, error) {
					if conf.Name == "motor1" {
						return &dummyMotor1, nil
					}
					return &dummyMotor2, nil
				}})
			resource.RegisterComponent(
				echoAPI,
				streamModel,
				resource.Registration[resource.Resource, resource.NoNativeConfig]{
					Constructor: func(
						ctx context.Context,
						_ resource.Dependencies,
						conf resource.Config,
						logger logging.ZapCompatibleLogger,
					) (resource.Resource, error) {
						return &dummyEcho1, nil
					},
				},
			)
			resource.RegisterComponent(
				base.API,
				model,
				resource.Registration[base.Base, resource.NoNativeConfig]{
					Constructor: func(
						ctx context.Context,
						_ resource.Dependencies,
						conf resource.Config,
						logger logging.ZapCompatibleLogger,
					) (base.Base, error) {
						return &dummyBase1, nil
					},
				},
			)

			roboConfig := fmt.Sprintf(`{
				"network":{
					"sessions": {
						"heartbeat_window": %[1]q
					}
				},
				"components": [
					{
						"model": "%[2]s",
						"name": "motor1",
						"type": "motor"
					},
					{
						"model": "%[2]s",
						"name": "motor2",
						"type": "motor"
					},
					{
						"model": "%[3]s",
						"name": "echo1",
						"type": "echo"
					},
					{
						"model": "%[2]s",
						"name": "base1",
						"type": "base"
					}
				]
			}
			`, windowSize, model, streamModel)

			cfg, err := config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger)
			test.That(t, err, test.ShouldBeNil)

			ctx := context.Background()
			r, err := robotimpl.New(ctx, cfg, logger)
			test.That(t, err, test.ShouldBeNil)

			options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
			err = r.StartWeb(ctx, options)
			test.That(t, err, test.ShouldBeNil)

			roboClient, err := client.New(ctx, addr, logger)
			test.That(t, err, test.ShouldBeNil)

			motor1, err := motor.FromRobot(roboClient, "motor1")
			test.That(t, err, test.ShouldBeNil)

			t.Log("get position of motor1 which will not be safety monitored")
			pos, err := motor1.Position(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos, test.ShouldEqual, 2.0)

			test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

			// kind of racy but it's okay
			ensureStop(t, "", stopChNames)

			roboClient, err = client.New(ctx, addr, logger)
			test.That(t, err, test.ShouldBeNil)

			motor1, err = motor.FromRobot(roboClient, "motor1")
			test.That(t, err, test.ShouldBeNil)

			t.Log("set power of motor1 which will be safety monitored")
			test.That(t, motor1.SetPower(ctx, 50, nil), test.ShouldBeNil)

			startAt := time.Now()
			test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

			ensureStop(t, "motor1", stopChNames)

			test.That(t,
				time.Since(startAt),
				test.ShouldBeBetweenOrEqual,
				float64(windowSize)*.75,
				float64(windowSize)*1.5,
			)

			dummyMotor1.mu.Lock()
			stopChs["motor1"].Chan = make(chan struct{})
			dummyMotor1.stopCh = stopChs["motor1"].Chan
			dummyMotor1.mu.Unlock()

			roboClient, err = client.New(ctx, addr, logger)
			test.That(t, err, test.ShouldBeNil)

			motor2, err := motor.FromRobot(roboClient, "motor2")
			test.That(t, err, test.ShouldBeNil)

			t.Log("set power of motor2 which will be safety monitored")
			test.That(t, motor2.SetPower(ctx, 50, nil), test.ShouldBeNil)

			echo1Client, err := roboClient.ResourceByName(echo1Name)
			test.That(t, err, test.ShouldBeNil)
			echo1Conn := echo1Client.(*dummyClient)

			t.Log("echo multiple of echo1 which will be safety monitored")
			echoMultiClient, err := echo1Conn.client.EchoMultiple(ctx, &echopb.EchoMultipleRequest{Name: "echo1"})
			test.That(t, err, test.ShouldBeNil)
			_, err = echoMultiClient.Recv() // EOF; okay
			test.That(t, err, test.ShouldBeError, io.EOF)

			startAt = time.Now()
			test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

			checkAgainst := []string{"motor1"}
			ensureStop(t, "motor2", checkAgainst)
			ensureStop(t, "echo1", checkAgainst)
			ensureStop(t, "base1", checkAgainst)

			test.That(t,
				time.Since(startAt),
				test.ShouldBeBetweenOrEqual,
				float64(windowSize)*.75,
				float64(windowSize)*1.5,
			)

			test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

			test.That(t, r.Close(ctx), test.ShouldBeNil)
		})
	}
}

func TestSessionsWithRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)

	stopChs := map[string]*StopChan{
		"remMotor1": {make(chan struct{}), "remMotor1"},
		"remMotor2": {make(chan struct{}), "remMotor2"},
		"remEcho1":  {make(chan struct{}), "remEcho1"},
		"remBase1":  {make(chan struct{}), "remBase1"},
		"motor1":    {make(chan struct{}), "motor1"},
		"base1":     {make(chan struct{}), "base1"},
	}
	stopChNames := make([]string, 0, len(stopChs))
	for name := range stopChs {
		stopChNames = append(stopChNames, name)
	}

	ensureStop := makeEnsureStop(stopChs)

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	streamModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	motor1Name := motor.Named("motor1")
	motor2Name := motor.Named("motor2")
	base1Name := base.Named("base1")
	echo1Name := resource.NewName(echoAPI, "echo1")
	dummyRemMotor1 := dummyMotor{Named: motor1Name.AsNamed(), stopCh: stopChs["remMotor1"].Chan}
	dummyRemMotor2 := dummyMotor{Named: motor2Name.AsNamed(), stopCh: stopChs["remMotor2"].Chan}
	dummyRemEcho1 := dummyEcho{Named: echo1Name.AsNamed(), stopCh: stopChs["remEcho1"].Chan}
	dummyRemBase1 := dummyBase{Named: base1Name.AsNamed(), stopCh: stopChs["remBase1"].Chan}
	dummyMotor1 := dummyMotor{Named: motor1Name.AsNamed(), stopCh: stopChs["motor1"].Chan}
	dummyBase1 := dummyBase{Named: base1Name.AsNamed(), stopCh: stopChs["base1"].Chan}
	resource.RegisterComponent(
		motor.API,
		model,
		resource.Registration[motor.Motor, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.ZapCompatibleLogger,
			) (motor.Motor, error) {
				if conf.Attributes.Bool("rem", false) {
					if conf.Name == "motor1" {
						return &dummyRemMotor1, nil
					}
					return &dummyRemMotor2, nil
				}
				return &dummyMotor1, nil
			},
		})
	resource.RegisterComponent(
		echoAPI,
		streamModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger logging.ZapCompatibleLogger,
			) (resource.Resource, error) {
				return &dummyRemEcho1, nil
			},
		},
	)
	resource.RegisterComponent(
		base.API,
		model,
		resource.Registration[base.Base, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger logging.ZapCompatibleLogger,
			) (base.Base, error) {
				if conf.Attributes.Bool("rem", false) {
					return &dummyRemBase1, nil
				}
				return &dummyBase1, nil
			},
		},
	)

	remoteConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%[1]s",
				"name": "motor1",
				"type": "motor",
				"attributes": {
					"rem": true
				}
			},
			{
				"model": "%[1]s",
				"name": "motor2",
				"type": "motor",
				"attributes": {
					"rem": true
				}
			},
			{
				"model": "%[2]s",
				"name": "echo1",
				"type": "echo",
				"attributes": {
					"rem": true
				}
			},
			{
				"model": "%[1]s",
				"name": "base1",
				"type": "base",
				"attributes": {
					"rem": true
				}
			}
		]
	}
	`, model, streamModel)

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(remoteConfig), logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	remoteRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	options, _, remoteAddr := robottestutils.CreateBaseOptionsAndListener(t)
	err = remoteRobot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	roboConfig := fmt.Sprintf(`{
		"remotes": [
			{
				"name": "rem1",
				"address": %q
			}
		],
		"components": [
			{
				"model": "%[2]s",
				"name": "motor1",
				"type": "motor"
			},
			{
				"model": "%[2]s",
				"name": "base1",
				"type": "base"
			}
		]
	}
	`, remoteAddr, model)

	cfg, err = config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	roboClient, err := client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	motor1, err := motor.FromRobot(roboClient, "rem1:motor1")
	if err != nil {
		bufSize := 1 << 20
		traces := make([]byte, bufSize)
		traceSize := runtime.Stack(traces, true)
		message := fmt.Sprintf("error accessing remote from roboClient: %s. logging stack trace for debugging purposes", err.Error())
		if traceSize == bufSize {
			message = fmt.Sprintf("%s (warning: backtrace truncated to %v bytes)", message, bufSize)
		}
		logger.Errorf("%s,\n %s", message, traces)
	}
	test.That(t, err, test.ShouldBeNil)

	t.Log("get position of rem1:motor1 which will not be safety monitored")
	pos, err := motor1.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2.0)

	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	// kind of racy but it's okay
	ensureStop(t, "", stopChNames)

	roboClient, err = client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	motor1, err = motor.FromRobot(roboClient, "rem1:motor1")
	test.That(t, err, test.ShouldBeNil)

	// this should cause safety monitoring
	t.Log("set power of rem1:motor1 which will be safety monitored")
	test.That(t, motor1.SetPower(ctx, 50, nil), test.ShouldBeNil)

	startAt := time.Now()
	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	ensureStop(t, "remMotor1", stopChNames)
	test.That(t,
		time.Since(startAt),
		test.ShouldBeBetweenOrEqual,
		float64(config.DefaultSessionHeartbeatWindow)*.5,
		float64(config.DefaultSessionHeartbeatWindow)*2.5,
	)

	dummyRemMotor1.mu.Lock()
	stopChs["remMotor1"].Chan = make(chan struct{})
	dummyRemMotor1.stopCh = stopChs["remMotor1"].Chan
	dummyRemMotor1.mu.Unlock()

	logger.Info("close robot instead of client")

	roboClient, err = client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	motor1, err = motor.FromRobot(roboClient, "rem1:motor1")
	test.That(t, err, test.ShouldBeNil)

	t.Log("set power of rem1:motor1 which will be safety monitored")
	test.That(t, motor1.SetPower(ctx, 50, nil), test.ShouldBeNil)

	startAt = time.Now()
	test.That(t, r.Close(ctx), test.ShouldBeNil)

	ensureStop(t, "remMotor1", stopChNames)

	test.That(t,
		time.Since(startAt),
		test.ShouldBeBetweenOrEqual,
		float64(config.DefaultSessionHeartbeatWindow)*.5,
		float64(config.DefaultSessionHeartbeatWindow)*2.5,
	)

	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	dummyRemMotor1.mu.Lock()
	stopChs["remMotor1"].Chan = make(chan struct{})
	dummyRemMotor1.stopCh = stopChs["remMotor1"].Chan
	dummyRemMotor1.mu.Unlock()

	r, err = robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	options, _, addr = robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	roboClient, err = client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	motor2, err := motor.FromRobot(roboClient, "rem1:motor2")
	if err != nil {
		bufSize := 1 << 20
		traces := make([]byte, bufSize)
		traceSize := runtime.Stack(traces, true)
		message := fmt.Sprintf("error accessing remote from roboClient: %s. logging stack trace for debugging purposes", err.Error())
		if traceSize == bufSize {
			message = fmt.Sprintf("%s (warning: backtrace truncated to %v bytes)", message, bufSize)
		}
		logger.Errorf("%s,\n %s", message, traces)
	}
	test.That(t, err, test.ShouldBeNil)

	t.Log("set power of rem1:motor2 which will be safety monitored")
	test.That(t, motor2.SetPower(ctx, 50, nil), test.ShouldBeNil)

	dummyName := resource.NewName(echoAPI, "echo1")
	echo1Client, err := roboClient.ResourceByName(dummyName)
	test.That(t, err, test.ShouldBeNil)
	echo1Conn := echo1Client.(*dummyClient)

	t.Log("echo multiple of remEcho1 which will be safety monitored")
	echoMultiClient, err := echo1Conn.client.EchoMultiple(ctx, &echopb.EchoMultipleRequest{Name: "echo1"})
	test.That(t, err, test.ShouldBeNil)
	_, err = echoMultiClient.Recv() // EOF; okay
	test.That(t, err, test.ShouldBeError, io.EOF)

	startAt = time.Now()
	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	checkAgainst := []string{"remMotor1", "motor1", "base1"}
	ensureStop(t, "remMotor2", checkAgainst)
	ensureStop(t, "remBase1", checkAgainst)
	ensureStop(t, "remEcho1", checkAgainst)

	test.That(t,
		time.Since(startAt),
		test.ShouldBeBetweenOrEqual,
		float64(config.DefaultSessionHeartbeatWindow)*.5,
		float64(config.DefaultSessionHeartbeatWindow)*2.5,
	)

	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	test.That(t, r.Close(ctx), test.ShouldBeNil)
	test.That(t, remoteRobot.Close(ctx), test.ShouldBeNil)
}

func TestSessionsMixedClients(t *testing.T) {
	logger := logging.NewTestLogger(t)
	stopChMotor1 := make(chan struct{})

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	motor1Name := motor.Named("motor1")
	dummyMotor1 := dummyMotor{Named: motor1Name.AsNamed(), stopCh: stopChMotor1}
	resource.RegisterComponent(
		motor.API,
		model,
		resource.Registration[motor.Motor, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.ZapCompatibleLogger,
			) (motor.Motor, error) {
				return &dummyMotor1, nil
			},
		})

	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%s",
				"name": "motor1",
				"type": "motor"
			}
		]
	}
	`, model)

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	roboClient1, err := client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)
	roboClient2, err := client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	motor1Client1, err := motor.FromRobot(roboClient1, "motor1")
	test.That(t, err, test.ShouldBeNil)
	motor1Client2, err := motor.FromRobot(roboClient2, "motor1")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, motor1Client1.SetPower(ctx, 50, nil), test.ShouldBeNil)
	time.Sleep(time.Second)
	// now client 2 is the last caller
	test.That(t, motor1Client2.GoFor(ctx, 1, 2, nil), test.ShouldBeNil)

	test.That(t, roboClient1.Close(ctx), test.ShouldBeNil)

	timer := time.NewTimer(config.DefaultSessionHeartbeatWindow * 2)
	select {
	case <-stopChMotor1:
		panic("unexpected")
	case <-timer.C:
		timer.Stop()
	}

	startAt := time.Now()
	test.That(t, roboClient2.Close(ctx), test.ShouldBeNil)

	select {
	case <-stopChMotor1:
		panic("unexpected; too fast")
	default:
	}

	<-stopChMotor1

	test.That(t,
		time.Since(startAt),
		test.ShouldBeBetweenOrEqual,
		float64(config.DefaultSessionHeartbeatWindow)*.75,
		float64(config.DefaultSessionHeartbeatWindow)*1.5,
	)

	test.That(t, r.Close(ctx), test.ShouldBeNil)
}

func TestSessionsMixedOwnersNoAuth(t *testing.T) {
	logger := logging.NewTestLogger(t)
	stopChMotor1 := make(chan struct{})

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	motor1Name := motor.Named("motor1")
	dummyMotor1 := dummyMotor{Named: motor1Name.AsNamed(), stopCh: stopChMotor1}
	resource.RegisterComponent(
		motor.API,
		model,
		resource.Registration[motor.Motor, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.ZapCompatibleLogger,
			) (motor.Motor, error) {
				return &dummyMotor1, nil
			},
		})

	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%s",
				"name": "motor1",
				"type": "motor"
			}
		]
	}
	`, model)

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// with no auth turned on, we will have no session owner, meaning mixing sessions technically works, for now
	roboClient1, err := client.New(ctx, addr, logger, client.WithDialOptions(rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
		Disable: true,
	})))
	test.That(t, err, test.ShouldBeNil)

	roboClientConn2, err := grpc.Dial(ctx, addr, logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
		Disable: true,
	}))
	test.That(t, err, test.ShouldBeNil)

	motor1Client1, err := motor.FromRobot(roboClient1, "motor1")
	test.That(t, err, test.ShouldBeNil)
	motor1Client2, err := motor.NewClientFromConn(ctx, roboClientConn2, "", motor.Named("motor1"), logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, motor1Client1.SetPower(ctx, 50, nil), test.ShouldBeNil)
	time.Sleep(time.Second)

	sessions := r.SessionManager().All()
	test.That(t, sessions, test.ShouldHaveLength, 1)
	sessID := sessions[0].ID().String()

	// now client 2 is the last caller but the sessions are the same
	client2Ctx := metadata.AppendToOutgoingContext(ctx, session.IDMetadataKey, sessID)
	test.That(t, motor1Client2.GoFor(client2Ctx, 1, 2, nil), test.ShouldBeNil)

	// this would just heartbeat it
	resp, err := robotpb.NewRobotServiceClient(roboClientConn2).StartSession(ctx, &robotpb.StartSessionRequest{
		Resume: sessID,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Id, test.ShouldEqual, sessID)

	// this is the only one heartbeating so we expect a stop
	startAt := time.Now()
	test.That(t, roboClient1.Close(ctx), test.ShouldBeNil)

	select {
	case <-stopChMotor1:
		panic("unexpected; too fast")
	default:
	}

	<-stopChMotor1

	test.That(t,
		time.Since(startAt),
		test.ShouldBeBetweenOrEqual,
		float64(config.DefaultSessionHeartbeatWindow)*.75,
		float64(config.DefaultSessionHeartbeatWindow)*1.5,
	)

	test.That(t, roboClientConn2.Close(), test.ShouldBeNil)
	test.That(t, r.Close(ctx), test.ShouldBeNil)
}

// TODO(RSDK-890): add explicit auth test once subjects are actually unique.
func TestSessionsMixedOwnersImplicitAuth(t *testing.T) {
	logger := logging.NewTestLogger(t)
	stopChMotor1 := make(chan struct{})

	model := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(8))
	motor1Name := motor.Named("motor1")
	dummyMotor1 := dummyMotor{Named: motor1Name.AsNamed(), stopCh: stopChMotor1}
	resource.RegisterComponent(
		motor.API,
		model,
		resource.Registration[motor.Motor, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.ZapCompatibleLogger,
			) (motor.Motor, error) {
				return &dummyMotor1, nil
			},
		})

	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%s",
				"name": "motor1",
				"type": "motor"
			}
		]
	}
	`, model)

	cfg, err := config.FromReader(context.Background(), "", strings.NewReader(roboConfig), logger)
	test.That(t, err, test.ShouldBeNil)

	ctx := context.Background()
	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// TODO(RSDK-890): using WebRTC (the default) gives us an implicit auth subject, for now
	roboClient1, err := client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	roboClientConn2, err := grpc.Dial(ctx, addr, logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
		Disable: true,
	}))
	test.That(t, err, test.ShouldBeNil)

	motor1Client1, err := motor.FromRobot(roboClient1, "motor1")
	test.That(t, err, test.ShouldBeNil)
	motor1Client2, err := motor.NewClientFromConn(ctx, roboClientConn2, "", motor.Named("motor1"), logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, motor1Client1.SetPower(ctx, 50, nil), test.ShouldBeNil)
	time.Sleep(time.Second)

	sessions := r.SessionManager().All()
	test.That(t, sessions, test.ShouldHaveLength, 1)
	sessID := sessions[0].ID().String()

	// cannot share here
	client2Ctx := metadata.AppendToOutgoingContext(ctx, session.IDMetadataKey, sessID)
	err = motor1Client2.GoFor(client2Ctx, 1, 2, nil)
	test.That(t, err, test.ShouldNotBeNil)
	statusErr, ok := status.FromError(err)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, statusErr.Code(), test.ShouldEqual, session.StatusNoSession.Code())
	test.That(t, statusErr.Message(), test.ShouldEqual, session.StatusNoSession.Message())

	// this should give us a new session instead since we cannot see it
	resp, err := robotpb.NewRobotServiceClient(roboClientConn2).StartSession(ctx, &robotpb.StartSessionRequest{
		Resume: sessID,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Id, test.ShouldNotEqual, sessID)
	test.That(t, resp.Id, test.ShouldNotEqual, "")

	// this is the only one heartbeating so we expect a stop
	startAt := time.Now()
	test.That(t, roboClient1.Close(ctx), test.ShouldBeNil)

	select {
	case <-stopChMotor1:
		panic("unexpected; too fast")
	default:
	}

	<-stopChMotor1

	test.That(t,
		time.Since(startAt),
		test.ShouldBeBetweenOrEqual,
		float64(config.DefaultSessionHeartbeatWindow)*.75,
		float64(config.DefaultSessionHeartbeatWindow)*1.5,
	)

	test.That(t, roboClientConn2.Close(), test.ShouldBeNil)
	test.That(t, r.Close(ctx), test.ShouldBeNil)
}

type dummyMotor struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	mu     sync.Mutex
	stopCh chan struct{}
}

func (dm *dummyMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 2, nil
}

func (dm *dummyMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	close(dm.stopCh)
	return nil
}

func (dm *dummyMotor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{}, nil
}

func (dm *dummyMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	return false, 0, nil
}

func (dm *dummyMotor) IsMoving(context.Context) (bool, error) {
	return false, nil
}

type dummyBase struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	mu     sync.Mutex
	stopCh chan struct{}
}

func (db *dummyBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	return nil
}

func (db *dummyBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	close(db.stopCh)
	return nil
}

func (db *dummyBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	return nil
}

func (db *dummyBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	return nil
}

func (db *dummyBase) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	return nil
}

func (db *dummyBase) IsMoving(context.Context) (bool, error) {
	return false, nil
}

func (db *dummyBase) Properties(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
	return base.Properties{}, nil
}

func (db *dummyBase) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return []spatialmath.Geometry{}, nil
}

// NewClientFromConn constructs a new client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.ZapCompatibleLogger,
) resource.Resource {
	c := echopb.NewTestEchoServiceClient(conn)
	return &dummyClient{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: c,
	}
}

type dummyClient struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	name   string
	client echopb.TestEchoServiceClient
}

func (c *dummyClient) Stop(ctx context.Context, extra map[string]interface{}) error {
	_, err := c.client.Stop(ctx, &echopb.StopRequest{Name: c.name})
	return err
}

func (c *dummyClient) IsMoving(context.Context) (bool, error) {
	return false, nil
}

type dummyEcho struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	mu     sync.Mutex
	stopCh chan struct{}
}

func (e *dummyEcho) EchoMultiple(ctx context.Context) error {
	session.SafetyMonitorResourceName(ctx, someBaseName1)
	return nil
}

func (e *dummyEcho) Stop(ctx context.Context, extra map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	close(e.stopCh)
	return nil
}

func (e *dummyEcho) IsMoving(context.Context) (bool, error) {
	return false, nil
}

type echoServer struct {
	echopb.UnimplementedTestEchoServiceServer
	coll resource.APIResourceCollection[resource.Resource]
}

func (srv *echoServer) EchoMultiple(
	req *echopb.EchoMultipleRequest,
	server echopb.TestEchoService_EchoMultipleServer,
) error {
	res, err := srv.coll.Resource(req.Name)
	if err != nil {
		return err
	}

	switch actual := res.(type) {
	case *dummyEcho:
		return actual.EchoMultiple(server.Context())
	case *dummyClient:
		echoClient, err := actual.client.EchoMultiple(server.Context(), &echopb.EchoMultipleRequest{Name: actual.name})
		if err != nil {
			return err
		}
		if _, err := echoClient.Recv(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		return nil
	default:
		// force an error
		return actual.(*dummyEcho).EchoMultiple(server.Context())
	}
}

func (srv *echoServer) Stop(ctx context.Context, req *echopb.StopRequest) (*echopb.StopResponse, error) {
	res, err := srv.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if actuator, ok := res.(resource.Actuator); ok {
		if err := actuator.Stop(ctx, nil); err != nil {
			return nil, err
		}
	}
	return &echopb.StopResponse{}, nil
}

type StopChan struct {
	Chan chan struct{}
	Name string
}

func makeEnsureStop(stopChs map[string]*StopChan) func(t *testing.T, name string, checkAgainst []string) {
	return func(t *testing.T, name string, checkAgainst []string) {
		t.Helper()
		stopCases := make([]reflect.SelectCase, 0, len(checkAgainst))
		for _, checkName := range checkAgainst {
			test.That(t, stopChs, test.ShouldContainKey, checkName)
			if stopChs[checkName].Name == name {
				continue
			}
			stopCases = append(stopCases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(stopChs[checkName].Chan),
			})
		}

		if name == "" {
			t.Log("checking nothing stops")
			stopCases = append(stopCases, reflect.SelectCase{
				Dir: reflect.SelectDefault,
			})
		} else {
			test.That(t, stopChs, test.ShouldContainKey, name)
			expectedCh := stopChs[name]

			stopCases = append(stopCases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(expectedCh.Chan),
			})
			t.Logf("waiting for %q to stop", name)
		}

		choice, _, _ := reflect.Select(stopCases)
		if choice == len(stopCases)-1 {
			return
		}
		for _, ch := range stopChs {
			if ch.Chan == stopCases[choice].Chan.Interface() {
				t.Fatalf("expected %q to stop but got %q", name, ch.Name)
			}
		}
		t.Fatal("unreachable; bug")
	}
}
