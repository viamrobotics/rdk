package robot_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
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
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/robottestutils"
)

var someBaseName1 = base.Named("base1")

var echoSubType = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	resource.SubtypeName("echo"),
)

func init() {
	registry.RegisterResourceSubtype(echoSubType, registry.ResourceSubtype{
		Reconfigurable: wrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&echopb.TestEchoService_ServiceDesc,
				&echoServer{s: subtypeSvc},
				echopb.RegisterTestEchoServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &echopb.TestEchoService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

func TestSessions(t *testing.T) {
	for _, windowSize := range []time.Duration{
		config.DefaultSessionHeartbeatWindow,
		time.Second * 5,
	} {
		t.Run(fmt.Sprintf("window size=%s", windowSize), func(t *testing.T) {
			logger := golog.NewTestLogger(t)
			stopChMotor1 := make(chan struct{})
			stopChMotor2 := make(chan struct{})
			stopChEcho1 := make(chan struct{})
			stopChBase1 := make(chan struct{})

			modelName := utils.RandomAlphaString(8)
			streamModelName := utils.RandomAlphaString(8)
			dummyMotor1 := dummyMotor{stopCh: stopChMotor1}
			dummyMotor2 := dummyMotor{stopCh: stopChMotor2}
			dummyEcho1 := dummyEcho{stopCh: stopChEcho1}
			dummyBase1 := dummyBase{stopCh: stopChBase1}
			registry.RegisterComponent(
				motor.Subtype,
				modelName,
				registry.Component{Constructor: func(
					ctx context.Context,
					deps registry.Dependencies,
					config config.Component,
					logger golog.Logger,
				) (interface{}, error) {
					if config.Name == "motor1" {
						return &dummyMotor1, nil
					}
					return &dummyMotor2, nil
				}})
			registry.RegisterComponent(
				echoSubType,
				streamModelName,
				registry.Component{
					Constructor: func(
						ctx context.Context,
						_ registry.Dependencies,
						config config.Component,
						logger golog.Logger,
					) (interface{}, error) {
						return &dummyEcho1, nil
					},
				},
			)
			registry.RegisterComponent(
				base.Subtype,
				modelName,
				registry.Component{
					Constructor: func(
						ctx context.Context,
						_ registry.Dependencies,
						config config.Component,
						logger golog.Logger,
					) (interface{}, error) {
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
				"name": "dummy1",
				"type": "echo"
			},
			{
				"model": "%[2]s",
				"name": "base1",
				"type": "base"
			}
		]
	}
	`, windowSize, modelName, streamModelName)

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

			// this kind of method doesn't cause safety monitoring
			pos, err := motor1.Position(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pos, test.ShouldEqual, 2.0)

			test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

			// kind of racy but it's okay
			select {
			case <-stopChMotor1:
				panic("unexpected")
			case <-stopChMotor2:
				panic("unexpected")
			case <-stopChEcho1:
				panic("unexpected")
			case <-stopChBase1:
				panic("unexpected")
			default:
			}

			roboClient, err = client.New(ctx, addr, logger)
			test.That(t, err, test.ShouldBeNil)

			motor1, err = motor.FromRobot(roboClient, "motor1")
			test.That(t, err, test.ShouldBeNil)

			// this should cause safety monitoring
			test.That(t, motor1.SetPower(ctx, 50, nil), test.ShouldBeNil)

			startAt := time.Now()
			test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

			select {
			case <-stopChMotor1:
				panic("unexpected; too fast")
			case <-stopChMotor2:
				panic("unexpected; too fast")
			case <-stopChEcho1:
				panic("unexpected; too fast")
			case <-stopChBase1:
				panic("unexpected; too fast")
			default:
			}

			select {
			case <-stopChMotor1:
				select {
				case <-stopChMotor2:
					panic("unexpected; wrong stop")
				case <-stopChEcho1:
					panic("unexpected; wrong stop")
				case <-stopChBase1:
					panic("unexpected; wrong stop")
				default:
				}
			case <-stopChMotor2:
				panic("unexpected; wrong stop")
			case <-stopChEcho1:
				panic("unexpected; wrong stop")
			case <-stopChBase1:
				panic("unexpected; wrong stop")
			}

			test.That(t,
				time.Since(startAt),
				test.ShouldBeBetweenOrEqual,
				float64(windowSize)*.75,
				float64(windowSize)*1.5,
			)

			dummyMotor1.mu.Lock()
			stopChMotor1 = make(chan struct{})
			dummyMotor1.mu.Unlock()

			roboClient, err = client.New(ctx, addr, logger)
			test.That(t, err, test.ShouldBeNil)

			motor2, err := motor.FromRobot(roboClient, "motor2")
			test.That(t, err, test.ShouldBeNil)

			// this should cause safety monitoring
			test.That(t, motor2.SetPower(ctx, 50, nil), test.ShouldBeNil)

			dummyName := resource.NameFromSubtype(echoSubType, "dummy1")
			dummy1Client, err := roboClient.ResourceByName(dummyName)
			test.That(t, err, test.ShouldBeNil)
			dummy1Conn := dummy1Client.(*reconfigurableClient).ProxyFor().(echopb.TestEchoServiceClient)

			echoMultiClient, err := dummy1Conn.EchoMultiple(ctx, &echopb.EchoMultipleRequest{Name: "dummy1"})
			test.That(t, err, test.ShouldBeNil)
			_, err = echoMultiClient.Recv() // EOF; okay
			test.That(t, err, test.ShouldBeError, io.EOF)

			startAt = time.Now()
			test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

			select {
			case <-stopChMotor1:
				panic("unexpected; too fast")
			case <-stopChMotor2:
				panic("unexpected; too fast")
			case <-stopChEcho1:
				panic("unexpected; too fast")
			case <-stopChBase1:
				panic("unexpected; too fast")
			default:
			}

			for idx, ch := range []chan struct{}{
				stopChMotor2, stopChEcho1, stopChBase1,
			} {
				logger.Info("check stop on ", idx)
				select {
				case <-stopChMotor1:
					panic("unexpected; wrong stop")
				case <-ch:
					select {
					case <-stopChMotor1:
						panic("unexpected; wrong stop")
					default:
					}
				}
			}

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
	logger := golog.NewTestLogger(t)
	stopChRemMotor1 := make(chan struct{})
	stopChRemMotor2 := make(chan struct{})
	stopChRemEcho1 := make(chan struct{})
	stopChRemBase1 := make(chan struct{})
	stopChMotor1 := make(chan struct{})
	stopChBase1 := make(chan struct{})

	modelName := utils.RandomAlphaString(8)
	streamModelName := utils.RandomAlphaString(8)
	dummyRemMotor1 := dummyMotor{stopCh: stopChRemMotor1}
	dummyRemMotor2 := dummyMotor{stopCh: stopChRemMotor2}
	dummyRemEcho1 := dummyEcho{stopCh: stopChRemEcho1}
	dummyRemBase1 := dummyBase{stopCh: stopChRemBase1}
	dummyMotor1 := dummyMotor{stopCh: stopChMotor1}
	dummyBase1 := dummyBase{stopCh: stopChBase1}
	registry.RegisterComponent(
		motor.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			if config.Attributes.Bool("rem", false) {
				if config.Name == "motor1" {
					return &dummyRemMotor1, nil
				}
				return &dummyRemMotor2, nil
			}
			return &dummyMotor1, nil
		}})
	registry.RegisterComponent(
		echoSubType,
		streamModelName,
		registry.Component{
			Constructor: func(
				ctx context.Context,
				_ registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				return &dummyRemEcho1, nil
			},
		},
	)
	registry.RegisterComponent(
		base.Subtype,
		modelName,
		registry.Component{
			Constructor: func(
				ctx context.Context,
				_ registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				if config.Attributes.Bool("rem", false) {
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
				"name": "dummy1",
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
	`, modelName, streamModelName)

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
	`, remoteAddr, modelName)

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
	test.That(t, err, test.ShouldBeNil)

	// this kind of method doesn't cause safety monitoring
	pos, err := motor1.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 2.0)

	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	// kind of racy but it's okay
	select {
	case <-stopChRemMotor1:
		panic("unexpected")
	case <-stopChRemMotor2:
		panic("unexpected")
	case <-stopChRemEcho1:
		panic("unexpected")
	case <-stopChRemBase1:
		panic("unexpected")
	case <-stopChMotor1:
		panic("unexpected")
	case <-stopChBase1:
		panic("unexpected")
	default:
	}

	roboClient, err = client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	motor1, err = motor.FromRobot(roboClient, "rem1:motor1")
	test.That(t, err, test.ShouldBeNil)

	// this should cause safety monitoring
	test.That(t, motor1.SetPower(ctx, 50, nil), test.ShouldBeNil)

	startAt := time.Now()
	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	select {
	case <-stopChRemMotor1:
		panic("unexpected; too fast")
	case <-stopChRemMotor2:
		panic("unexpected; too fast")
	case <-stopChRemEcho1:
		panic("unexpected; too fast")
	case <-stopChRemBase1:
		panic("unexpected; too fast")
	case <-stopChMotor1:
		panic("unexpected; too fast")
	case <-stopChBase1:
		panic("unexpected; too fast")
	default:
	}

	select {
	case <-stopChRemMotor1:
		select {
		case <-stopChRemMotor2:
			panic("unexpected; wrong stop")
		case <-stopChRemEcho1:
			panic("unexpected; wrong stop")
		case <-stopChRemBase1:
			panic("unexpected; wrong stop")
		case <-stopChMotor1:
			panic("unexpected; wrong stop")
		case <-stopChBase1:
			panic("unexpected; wrong stop")
		default:
		}
	case <-stopChRemMotor2:
		panic("unexpected; wrong stop")
	case <-stopChRemEcho1:
		panic("unexpected; wrong stop")
	case <-stopChRemBase1:
		panic("unexpected; wrong stop")
	case <-stopChMotor1:
		panic("unexpected; wrong stop")
	case <-stopChBase1:
		panic("unexpected; wrong stop")
	}

	test.That(t,
		time.Since(startAt),
		test.ShouldBeBetweenOrEqual,
		float64(config.DefaultSessionHeartbeatWindow)*.75,
		float64(config.DefaultSessionHeartbeatWindow)*1.5,
	)

	dummyRemMotor1.mu.Lock()
	stopChRemMotor1 = make(chan struct{})
	dummyRemMotor1.stopCh = stopChRemMotor1
	dummyRemMotor1.mu.Unlock()

	logger.Info("close robot instead of client")

	roboClient, err = client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	motor1, err = motor.FromRobot(roboClient, "rem1:motor1")
	test.That(t, err, test.ShouldBeNil)

	// this should cause safety monitoring
	test.That(t, motor1.SetPower(ctx, 50, nil), test.ShouldBeNil)

	startAt = time.Now()
	test.That(t, r.Close(ctx), test.ShouldBeNil)

	select {
	case <-stopChRemMotor1:
		panic("unexpected; too fast")
	case <-stopChRemMotor2:
		panic("unexpected; too fast")
	case <-stopChRemEcho1:
		panic("unexpected; too fast")
	case <-stopChRemBase1:
		panic("unexpected; too fast")
	case <-stopChMotor1:
		panic("unexpected; too fast")
	case <-stopChBase1:
		panic("unexpected; too fast")
	default:
	}

	select {
	case <-stopChRemMotor1:
		select {
		case <-stopChRemMotor2:
			panic("unexpected; wrong stop")
		case <-stopChRemEcho1:
			panic("unexpected; wrong stop")
		case <-stopChRemBase1:
			panic("unexpected; wrong stop")
		case <-stopChMotor1:
			panic("unexpected; wrong stop")
		case <-stopChBase1:
			panic("unexpected; wrong stop")
		default:
		}
	case <-stopChRemMotor2:
		panic("unexpected; wrong stop")
	case <-stopChRemEcho1:
		panic("unexpected; wrong stop")
	case <-stopChRemBase1:
		panic("unexpected; wrong stop")
	case <-stopChMotor1:
		panic("unexpected; wrong stop")
	case <-stopChBase1:
		panic("unexpected; wrong stop")
	}

	test.That(t,
		time.Since(startAt),
		test.ShouldBeBetweenOrEqual,
		float64(config.DefaultSessionHeartbeatWindow)*.75,
		float64(config.DefaultSessionHeartbeatWindow)*1.5,
	)

	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	dummyRemMotor1.mu.Lock()
	stopChRemMotor1 = make(chan struct{})
	dummyRemMotor1.stopCh = stopChRemMotor1
	dummyRemMotor1.mu.Unlock()

	r, err = robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	options, _, addr = robottestutils.CreateBaseOptionsAndListener(t)
	err = r.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	roboClient, err = client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	motor2, err := motor.FromRobot(roboClient, "rem1:motor2")
	test.That(t, err, test.ShouldBeNil)

	// this should cause safety monitoring
	test.That(t, motor2.SetPower(ctx, 50, nil), test.ShouldBeNil)

	dummyName := resource.NameFromSubtype(echoSubType, "dummy1")
	dummy1Client, err := roboClient.ResourceByName(dummyName)
	test.That(t, err, test.ShouldBeNil)
	dummy1Conn := dummy1Client.(*reconfigurableClient).ProxyFor().(echopb.TestEchoServiceClient)

	echoMultiClient, err := dummy1Conn.EchoMultiple(ctx, &echopb.EchoMultipleRequest{Name: "dummy1"})
	test.That(t, err, test.ShouldBeNil)
	_, err = echoMultiClient.Recv() // EOF; okay
	test.That(t, err, test.ShouldBeError, io.EOF)

	startAt = time.Now()
	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	select {
	case <-stopChRemMotor1:
		panic("unexpected; too fast")
	case <-stopChRemMotor2:
		panic("unexpected; too fast")
	case <-stopChRemEcho1:
		panic("unexpected; too fast")
	case <-stopChRemBase1:
		panic("unexpected; too fast")
	case <-stopChMotor1:
		panic("unexpected; too fast")
	case <-stopChBase1:
		panic("unexpected; too fast")
	default:
	}

	for idx, ch := range []chan struct{}{
		stopChRemMotor2, stopChRemBase1, stopChRemEcho1,
	} {
		logger.Info("check stop on ", idx)
		select {
		case <-stopChRemMotor1:
			panic("unexpected; wrong stop")
		case <-stopChMotor1:
			panic("unexpected; wrong stop")
		case <-stopChBase1:
			panic("unexpected; wrong stop")
		case <-ch:
			select {
			case <-stopChRemMotor1:
				panic("unexpected; wrong stop")
			case <-stopChMotor1:
				panic("unexpected; wrong stop")
			case <-stopChBase1:
				panic("unexpected; wrong stop")
			default:
			}
		}
	}

	test.That(t,
		time.Since(startAt),
		test.ShouldBeBetweenOrEqual,
		float64(config.DefaultSessionHeartbeatWindow)*.75,
		float64(config.DefaultSessionHeartbeatWindow)*1.5,
	)

	test.That(t, roboClient.Close(ctx), test.ShouldBeNil)

	test.That(t, r.Close(ctx), test.ShouldBeNil)
	test.That(t, remoteRobot.Close(ctx), test.ShouldBeNil)
}

func TestSessionsMixedClients(t *testing.T) {
	logger := golog.NewTestLogger(t)
	stopChMotor1 := make(chan struct{})

	modelName := utils.RandomAlphaString(8)
	dummyMotor1 := dummyMotor{stopCh: stopChMotor1}
	registry.RegisterComponent(
		motor.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return &dummyMotor1, nil
		}})

	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%s",
				"name": "motor1",
				"type": "motor"
			}
		]
	}
	`, modelName)

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
	logger := golog.NewTestLogger(t)
	stopChMotor1 := make(chan struct{})

	modelName := utils.RandomAlphaString(8)
	dummyMotor1 := dummyMotor{stopCh: stopChMotor1}
	registry.RegisterComponent(
		motor.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return &dummyMotor1, nil
		}})

	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%s",
				"name": "motor1",
				"type": "motor"
			}
		]
	}
	`, modelName)

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
	motor1Client2 := motor.NewClientFromConn(ctx, roboClientConn2, "motor1", logger)

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
	logger := golog.NewTestLogger(t)
	stopChMotor1 := make(chan struct{})

	modelName := utils.RandomAlphaString(8)
	dummyMotor1 := dummyMotor{stopCh: stopChMotor1}
	registry.RegisterComponent(
		motor.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return &dummyMotor1, nil
		}})

	roboConfig := fmt.Sprintf(`{
		"components": [
			{
				"model": "%s",
				"name": "motor1",
				"type": "motor"
			}
		]
	}
	`, modelName)

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
	motor1Client2 := motor.NewClientFromConn(ctx, roboClientConn2, "motor1", logger)

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
	mu sync.Mutex
	motor.LocalMotor
	stopCh chan struct{}
}

func (dm *dummyMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	return nil
}

func (dm *dummyMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
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

type dummyBase struct {
	mu sync.Mutex
	base.LocalBase
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

// NewClientFromConn constructs a new client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) echopb.TestEchoServiceClient {
	return echopb.NewTestEchoServiceClient(conn)
}

func wrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	switch v := r.(type) {
	case echopb.TestEchoServiceClient:
		return &reconfigurableClient{name: name, actual: v}, nil
	case *dummyEcho:
		return &reconfigurableClient{name: name, actual: v}, nil
	default:
		panic(errors.Errorf("bad type %T", r))
	}
}

type reconfigurableClient struct {
	mu     sync.RWMutex
	name   resource.Name
	actual interface{}
}

func (r *reconfigurableClient) Name() resource.Name {
	return r.name
}

func (r *reconfigurableClient) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableClient) Reconfigure(ctx context.Context, newBase resource.Reconfigurable) error {
	panic("unexpected")
}

func (r *reconfigurableClient) Stop(ctx context.Context, extra map[string]interface{}) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	switch v := r.actual.(type) {
	case echopb.TestEchoServiceClient:
		_, err := v.Stop(ctx, &echopb.StopRequest{Name: r.name.Name})
		return err
	case *dummyEcho:
		return v.Stop(ctx, nil)
	default:
		panic(errors.Errorf("bad type %T", r))
	}
}

func (r *reconfigurableClient) EchoMultiple(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	switch v := r.actual.(type) {
	case echopb.TestEchoServiceClient:
		echoClient, err := v.EchoMultiple(ctx, &echopb.EchoMultipleRequest{Name: r.name.Name})
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
	case *dummyEcho:
		return v.EchoMultiple(ctx)
	default:
		panic(errors.Errorf("bad type %T", r))
	}
}

type dummyEcho struct {
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

type echoServer struct {
	echopb.UnimplementedTestEchoServiceServer
	s subtype.Service
}

func (srv *echoServer) EchoMultiple(
	req *echopb.EchoMultipleRequest,
	server echopb.TestEchoService_EchoMultipleServer,
) error {
	if err := srv.s.Resource(req.Name).(*reconfigurableClient).EchoMultiple(server.Context()); err != nil {
		return err
	}
	return nil
}

func (srv *echoServer) Stop(ctx context.Context, req *echopb.StopRequest) (*echopb.StopResponse, error) {
	if err := resource.StopResource(ctx, srv.s.Resource(req.Name), nil); err != nil {
		return nil, err
	}
	return &echopb.StopResponse{}, nil
}
