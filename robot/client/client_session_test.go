package client_test

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	echopb "go.viam.com/utils/proto/rpc/examples/echoresource/v1"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/web"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
)

/*
The client session tests here are fairly complicated because they make heavy use of dependency injection
in order to mimic the server side very deliberately in order to introduce failures that would be hard
with the actual production code. As a result, you'll find the the server analogue to this to be much simpler
to reason about and in fact it ends up covering many similar cases but ones that are not as important to
client behavior.
*/

var (
	someTargetName1 = resource.NewName("rdk", "bar", "baz", "barf")
	someTargetName2 = resource.NewName("rdk", "bar", "baz", "barfy")
)

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
				&echopb.EchoResourceService_ServiceDesc,
				&echoServer{s: subtypeSvc},
				echopb.RegisterEchoResourceServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &echopb.EchoResourceService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	registry.RegisterComponent(
		echoSubType,
		resource.NewDefaultModel("fake"),
		registry.Component{
			Constructor: func(
				ctx context.Context,
				_ registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				panic("never construct")
			},
		},
	)
}

func TestClientSessionOptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, webrtcDisabled := range []bool{false, true} {
		for _, sessionsDisabled := range []bool{false, true} {
			for _, withRemoteName := range []bool{false, true} {
				webrtcDisabledCopy := webrtcDisabled
				withRemoteNameCopy := withRemoteName
				sessionsDisabledCopy := sessionsDisabled

				t.Run(
					fmt.Sprintf(
						"webrtc disabled=%t,with remote name=%t,sessions disabled=%t",
						webrtcDisabledCopy,
						withRemoteNameCopy,
						sessionsDisabledCopy,
					),
					func(t *testing.T) {
						t.Parallel()

						logger := golog.NewTestLogger(t)

						sessMgr := &sessionManager{}
						arbName := resource.NameFromSubtype(echoSubType, "woo")
						injectRobot := &inject.Robot{
							ResourceNamesFunc: func() []resource.Name { return []resource.Name{arbName} },
							ResourceByNameFunc: func(name resource.Name) (interface{}, error) {
								return &dummyEcho{}, nil
							},
							ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
							LoggerFunc:              func() golog.Logger { return logger },
							SessMgr:                 sessMgr,
						}

						svc := web.New(ctx, injectRobot, logger)

						options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
						err := svc.Start(ctx, options)
						test.That(t, err, test.ShouldBeNil)

						var opts []client.RobotClientOption
						if sessionsDisabledCopy {
							opts = append(opts, client.WithDisableSessions())
						}
						if withRemoteNameCopy {
							opts = append(opts, client.WithRemoteName("rem1"))
						}
						if webrtcDisabledCopy {
							opts = append(opts, client.WithDialOptions(rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
								Disable: true,
							})))
						}
						roboClient, err := client.New(ctx, addr, logger, opts...)
						test.That(t, err, test.ShouldBeNil)

						injectRobot.Mu.Lock()
						injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
							session.SafetyMonitorResourceName(ctx, someTargetName1)
							return []robot.Status{}, nil
						}
						injectRobot.Mu.Unlock()

						var capMu sync.Mutex
						var startCalled int
						var findCalled int
						var capOwnerID string
						var capPeerConnInfo *pb.PeerConnectionInfo
						var capID uuid.UUID
						var associateCount int
						var storedID uuid.UUID
						var storedResourceName resource.Name

						sess1 := session.New("ownerID", nil, 5*time.Second, func(id uuid.UUID, resourceName resource.Name) {
							capMu.Lock()
							associateCount++
							storedID = id
							storedResourceName = resourceName
							capMu.Unlock()
						})
						nextCtx := session.ToContext(ctx, sess1)

						sessMgr.mu.Lock()
						sessMgr.StartFunc = func(ownerID string, peerConnInfo *pb.PeerConnectionInfo) (*session.Session, error) {
							capMu.Lock()
							startCalled++
							capOwnerID = ownerID
							capPeerConnInfo = peerConnInfo
							capMu.Unlock()
							return sess1, nil
						}
						sessMgr.FindByIDFunc = func(id uuid.UUID, ownerID string) (*session.Session, error) {
							if id != sess1.ID() {
								return nil, errors.New("session id mismatch")
							}
							capMu.Lock()
							findCalled++
							capID = id
							capOwnerID = ownerID
							capMu.Unlock()
							sess1.Heartbeat() // gotta keep session alive
							return sess1, nil
						}
						sessMgr.mu.Unlock()

						resp, err := roboClient.Status(nextCtx, []resource.Name{})
						test.That(t, err, test.ShouldBeNil)
						test.That(t, len(resp), test.ShouldEqual, 0)

						if sessionsDisabledCopy {
							// wait for any kind of heartbeat
							time.Sleep(2 * time.Second)

							capMu.Lock()
							test.That(t, startCalled, test.ShouldEqual, 0)
							test.That(t, findCalled, test.ShouldEqual, 0)
							capMu.Unlock()
						} else {
							capMu.Lock()
							test.That(t, startCalled, test.ShouldEqual, 1)
							test.That(t, findCalled, test.ShouldEqual, 0)

							if webrtcDisabledCopy {
								test.That(t, capOwnerID, test.ShouldEqual, "")
							} else {
								test.That(t, capOwnerID, test.ShouldNotEqual, "")
							}
							test.That(t, capPeerConnInfo, test.ShouldNotBeNil)
							capMu.Unlock()

							startAt := time.Now()
							testutils.WaitForAssertionWithSleep(t, time.Second, 10, func(tb testing.TB) {
								tb.Helper()

								capMu.Lock()
								defer capMu.Unlock()
								test.That(tb, findCalled, test.ShouldBeGreaterThanOrEqualTo, 5)
								test.That(tb, capID, test.ShouldEqual, sess1.ID())

								if webrtcDisabledCopy {
									test.That(tb, capOwnerID, test.ShouldEqual, "")
								} else {
									test.That(tb, capOwnerID, test.ShouldNotEqual, "")
								}
							})
							// testing against time but fairly generous range
							test.That(t, time.Since(startAt), test.ShouldBeBetween, 4*time.Second, 7*time.Second)
						}

						capMu.Lock()
						if withRemoteNameCopy {
							test.That(t, associateCount, test.ShouldEqual, 1)
							test.That(t, storedID, test.ShouldEqual, sess1.ID())
							test.That(t, storedResourceName, test.ShouldResemble, someTargetName1.PrependRemote("rem1"))
						} else {
							test.That(t, associateCount, test.ShouldEqual, 0)
						}
						capMu.Unlock()

						echoRes, err := roboClient.ResourceByName(arbName)
						test.That(t, err, test.ShouldBeNil)
						echoClient := echoRes.(*reconfigurableClient).actual

						echoMultiClient, err := echoClient.EchoResourceMultiple(nextCtx, &echopb.EchoResourceMultipleRequest{
							Name:    arbName.Name,
							Message: "doesnotmatter",
						})
						test.That(t, err, test.ShouldBeNil)
						_, err = echoMultiClient.Recv() // EOF; okay
						test.That(t, err, test.ShouldBeError, io.EOF)

						err = roboClient.Close(context.Background())
						test.That(t, err, test.ShouldBeNil)

						capMu.Lock()
						if withRemoteNameCopy {
							test.That(t, associateCount, test.ShouldEqual, 2)
							test.That(t, storedID, test.ShouldEqual, sess1.ID())
							test.That(t, storedResourceName, test.ShouldResemble, someTargetName2.PrependRemote("rem1"))
						} else {
							test.That(t, associateCount, test.ShouldEqual, 0)
						}
						capMu.Unlock()

						test.That(t, utils.TryClose(ctx, svc), test.ShouldBeNil)
					})
			}
		}
	}
}

func TestClientSessionExpiration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, webrtcDisabled := range []bool{false, true} {
		webrtcDisabledCopy := webrtcDisabled

		t.Run(
			fmt.Sprintf(
				"webrtc disabled=%t",
				webrtcDisabledCopy,
			),
			func(t *testing.T) {
				t.Parallel()

				logger := golog.NewTestLogger(t)

				sessMgr := &sessionManager{}
				arbName := resource.NameFromSubtype(echoSubType, "woo")

				var dummyEcho1 dummyEcho
				injectRobot := &inject.Robot{
					ResourceNamesFunc: func() []resource.Name { return []resource.Name{arbName} },
					ResourceByNameFunc: func(name resource.Name) (interface{}, error) {
						return &dummyEcho1, nil
					},
					ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
					LoggerFunc:              func() golog.Logger { return logger },
					SessMgr:                 sessMgr,
				}

				svc := web.New(ctx, injectRobot, logger)

				options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
				err := svc.Start(ctx, options)
				test.That(t, err, test.ShouldBeNil)

				var opts []client.RobotClientOption
				if webrtcDisabledCopy {
					opts = append(opts, client.WithDialOptions(rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
						Disable: true,
					})))
				}
				roboClient, err := client.New(ctx, addr, logger, opts...)
				test.That(t, err, test.ShouldBeNil)

				injectRobot.Mu.Lock()
				var capSessID uuid.UUID
				injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
					sess, ok := session.FromContext(ctx)
					if !ok {
						panic("expected session")
					}
					capSessID = sess.ID()
					return []robot.Status{}, nil
				}
				injectRobot.Mu.Unlock()

				var capMu sync.Mutex
				var startCalled int
				var findCalled int

				sess1 := session.New("ownerID", nil, 5*time.Second, nil)
				sess2 := session.New("ownerID", nil, 5*time.Second, nil)
				sess3 := session.New("ownerID", nil, 5*time.Second, nil)
				sessions := []*session.Session{sess1, sess2, sess3}
				nextCtx := session.ToContext(ctx, sess1)

				sessMgr.mu.Lock()
				sessMgr.StartFunc = func(ownerID string, peerConnInfo *pb.PeerConnectionInfo) (*session.Session, error) {
					logger.Debug("start session requested")
					capMu.Lock()
					if startCalled != 0 && findCalled < 5 {
						logger.Debug("premature start session")
						return nil, errors.New("premature restart")
					}
					startCalled++
					findCalled = 0
					sess := sessions[startCalled-1]
					capMu.Unlock()

					// like a restart
					sessMgr.expired = false
					logger.Debug("start session started")
					return sess, nil
				}
				sessMgr.FindByIDFunc = func(id uuid.UUID, ownerID string) (*session.Session, error) {
					capMu.Lock()
					findCalled++
					if startCalled == 1 && findCalled >= 5 { // expired until restart
						capMu.Unlock()
						logger.Debug("enough heartbeats once; expire the session")
						return nil, session.ErrNoSession
					}
					if startCalled == 2 && findCalled >= 5 { // expired until restart
						capMu.Unlock()
						logger.Debug("enough heartbeats twice; expire the session")
						return nil, session.ErrNoSession
					}
					sess := sessions[startCalled-1]
					if id != sess.ID() {
						return nil, errors.New("session id mismatch")
					}
					capMu.Unlock()
					sess.Heartbeat() // gotta keep session alive
					return sess, nil
				}
				sessMgr.mu.Unlock()

				resp, err := roboClient.Status(nextCtx, []resource.Name{})
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(resp), test.ShouldEqual, 0)

				injectRobot.Mu.Lock()
				test.That(t, capSessID, test.ShouldEqual, sess1.ID())
				injectRobot.Mu.Unlock()

				capMu.Lock()
				test.That(t, startCalled, test.ShouldEqual, 1)
				test.That(t, findCalled, test.ShouldEqual, 0)
				capMu.Unlock()

				startAt := time.Now()
				testutils.WaitForAssertionWithSleep(t, time.Second, 10, func(tb testing.TB) {
					tb.Helper()

					capMu.Lock()
					defer capMu.Unlock()
					test.That(tb, findCalled, test.ShouldBeGreaterThanOrEqualTo, 5)
				})
				// testing against time but fairly generous range
				test.That(t, time.Since(startAt), test.ShouldBeBetween, 4*time.Second, 7*time.Second)

				sessMgr.mu.Lock()
				sessMgr.expired = true
				sessMgr.mu.Unlock()

				capMu.Lock()
				test.That(t, startCalled, test.ShouldEqual, 1)
				capMu.Unlock()

				logger.Debug("now call status which should work with a restarted session")
				resp, err = roboClient.Status(nextCtx, []resource.Name{})
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(resp), test.ShouldEqual, 0)

				injectRobot.Mu.Lock()
				test.That(t, capSessID, test.ShouldEqual, sess2.ID())
				injectRobot.Mu.Unlock()

				capMu.Lock()
				test.That(t, startCalled, test.ShouldEqual, 2)
				capMu.Unlock()

				testutils.WaitForAssertionWithSleep(t, time.Second, 10, func(tb testing.TB) {
					tb.Helper()

					capMu.Lock()
					defer capMu.Unlock()
					test.That(tb, findCalled, test.ShouldBeGreaterThanOrEqualTo, 5)
				})
				sessMgr.mu.Lock()
				sessMgr.expired = true
				sessMgr.mu.Unlock()

				echoRes, err := roboClient.ResourceByName(arbName)
				test.That(t, err, test.ShouldBeNil)
				echoClient := echoRes.(*reconfigurableClient).actual

				capMu.Lock()
				test.That(t, startCalled, test.ShouldEqual, 2)
				capMu.Unlock()

				echoMultiClient, err := echoClient.EchoResourceMultiple(nextCtx, &echopb.EchoResourceMultipleRequest{
					Name:    arbName.Name,
					Message: "doesnotmatter",
				})
				test.That(t, err, test.ShouldBeNil)
				_, err = echoMultiClient.Recv() // EOF; okay
				test.That(t, err, test.ShouldBeError, io.EOF)

				dummyEcho1.mu.Lock()
				test.That(t, dummyEcho1.capSessID, test.ShouldEqual, sess3.ID())
				dummyEcho1.mu.Unlock()

				capMu.Lock()
				test.That(t, startCalled, test.ShouldEqual, 3)
				capMu.Unlock()

				err = roboClient.Close(context.Background())
				test.That(t, err, test.ShouldBeNil)

				test.That(t, utils.TryClose(ctx, svc), test.ShouldBeNil)
			})
	}
}

func TestClientSessionResume(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, webrtcDisabled := range []bool{false, true} {
		webrtcDisabledCopy := webrtcDisabled

		t.Run(
			fmt.Sprintf(
				"webrtc disabled=%t",
				webrtcDisabledCopy,
			),
			func(t *testing.T) {
				t.Parallel()

				logger := golog.NewTestLogger(t)

				sessMgr := &sessionManager{}
				injectRobot := &inject.Robot{
					ResourceNamesFunc:       func() []resource.Name { return []resource.Name{} },
					ResourceRPCSubtypesFunc: func() []resource.RPCSubtype { return nil },
					LoggerFunc:              func() golog.Logger { return logger },
					SessMgr:                 sessMgr,
				}

				svc := web.New(ctx, injectRobot, logger)

				options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
				err := svc.Start(ctx, options)
				test.That(t, err, test.ShouldBeNil)

				var opts []client.RobotClientOption
				if webrtcDisabledCopy {
					opts = append(opts, client.WithDialOptions(rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
						Disable: true,
					})))
				}
				roboClient, err := client.New(ctx, addr, logger, opts...)
				test.That(t, err, test.ShouldBeNil)

				var capMu sync.Mutex
				var startCalled int
				var findCalled int

				sess1 := session.New("ownerID", nil, 5*time.Second, nil)
				nextCtx := session.ToContext(ctx, sess1)

				sessMgr.mu.Lock()
				sessMgr.StartFunc = func(ownerID string, peerConnInfo *pb.PeerConnectionInfo) (*session.Session, error) {
					logger.Debug("start session requested")
					capMu.Lock()
					startCalled++
					findCalled = 0
					capMu.Unlock()
					return sess1, nil
				}
				sessMgr.FindByIDFunc = func(id uuid.UUID, ownerID string) (*session.Session, error) {
					if id != sess1.ID() {
						return nil, errors.New("session id mismatch")
					}
					capMu.Lock()
					findCalled++
					capMu.Unlock()
					sess1.Heartbeat() // gotta keep session alive
					return sess1, nil
				}
				sessMgr.mu.Unlock()

				injectRobot.Mu.Lock()
				var capSessID uuid.UUID
				injectRobot.StatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
					sess, ok := session.FromContext(ctx)
					if !ok {
						panic("expected session")
					}
					capSessID = sess.ID()
					return []robot.Status{}, nil
				}
				injectRobot.Mu.Unlock()

				resp, err := roboClient.Status(nextCtx, []resource.Name{})
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(resp), test.ShouldEqual, 0)

				testutils.WaitForAssertionWithSleep(t, time.Second, 10, func(tb testing.TB) {
					tb.Helper()
					capMu.Lock()
					defer capMu.Unlock()
					test.That(tb, findCalled, test.ShouldBeGreaterThanOrEqualTo, 5)
				})

				capMu.Lock()
				test.That(t, startCalled, test.ShouldEqual, 1)
				capMu.Unlock()

				errFindCalled := make(chan struct{})
				sessMgr.mu.Lock()
				sessMgr.FindByIDFunc = func(id uuid.UUID, ownerID string) (*session.Session, error) {
					close(errFindCalled)
					return nil, status.New(codes.Unavailable, "disconnected or something").Err()
				}
				sessMgr.mu.Unlock()

				<-errFindCalled
				time.Sleep(time.Second)

				sessMgr.mu.Lock()
				sessMgr.FindByIDFunc = func(id uuid.UUID, ownerID string) (*session.Session, error) {
					if id != sess1.ID() {
						return nil, errors.New("session id mismatch")
					}
					capMu.Lock()
					findCalled++
					capMu.Unlock()
					sess1.Heartbeat() // gotta keep session alive
					return sess1, nil
				}
				sessMgr.mu.Unlock()

				resp, err = roboClient.Status(nextCtx, []resource.Name{})
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(resp), test.ShouldEqual, 0)

				capMu.Lock()
				test.That(t, startCalled, test.ShouldEqual, 1)
				capMu.Unlock()

				injectRobot.Mu.Lock()
				test.That(t, capSessID, test.ShouldEqual, sess1.ID())
				injectRobot.Mu.Unlock()

				err = roboClient.Close(context.Background())
				test.That(t, err, test.ShouldBeNil)

				test.That(t, utils.TryClose(ctx, svc), test.ShouldBeNil)
			})
	}
}

// we don't want everyone making an inject of this, so let's keep it here for now.
type sessionManager struct {
	mu           sync.Mutex
	StartFunc    func(ownerID string, peerConnInfo *pb.PeerConnectionInfo) (*session.Session, error)
	FindByIDFunc func(id uuid.UUID, ownerID string) (*session.Session, error)
	expired      bool
}

func (mgr *sessionManager) Start(ownerID string, peerConnInfo *pb.PeerConnectionInfo) (*session.Session, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.StartFunc(ownerID, peerConnInfo)
}

func (mgr *sessionManager) All() []*session.Session {
	panic("unimplemented")
}

func (mgr *sessionManager) FindByID(id uuid.UUID, ownerID string) (*session.Session, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.FindByIDFunc(id, ownerID)
}

func (mgr *sessionManager) AssociateResource(id uuid.UUID, resourceName resource.Name) {
	panic("unimplemented")
}

func (mgr *sessionManager) Close() {
}

func (mgr *sessionManager) ServerInterceptors() session.ServerInterceptors {
	return session.ServerInterceptors{
		// this is required for expiration tests which pull session info via interceptor
		UnaryServerInterceptor:  mgr.UnaryServerInterceptor,
		StreamServerInterceptor: mgr.StreamServerInterceptor,
	}
}

func (mgr *sessionManager) sessionFromMetadata(ctx context.Context) (context.Context, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, nil
	}

	values := meta.Get(session.IDMetadataKey)
	switch len(values) {
	case 0:
		return ctx, nil
	case 1:
		mgr.mu.Lock()
		if mgr.expired {
			mgr.mu.Unlock()
			return nil, session.ErrNoSession
		}
		mgr.mu.Unlock()
		sessID, err := uuid.Parse(values[0])
		if err != nil {
			return nil, err
		}
		sess := session.NewWithID(sessID, "", nil, time.Minute, nil)
		return session.ToContext(ctx, sess), nil
	default:
		return nil, errors.New("found more than one session id in metadata")
	}
}

func (mgr *sessionManager) UnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	ctx, err := mgr.sessionFromMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// StreamServerInterceptor associates the current session (if present) in the current context before
// passing it to the stream response handler.
func (mgr *sessionManager) StreamServerInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	ctx, err := mgr.sessionFromMetadata(ss.Context())
	if err != nil {
		return err
	}
	return handler(srv, &ssStreamContextWrapper{ss, ctx})
}

type ssStreamContextWrapper struct {
	grpc.ServerStream
	ctx context.Context
}

func (w ssStreamContextWrapper) Context() context.Context {
	return w.ctx
}

// NewClientFromConn constructs a new client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) echopb.EchoResourceServiceClient {
	return echopb.NewEchoResourceServiceClient(conn)
}

func wrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	return &reconfigurableClient{name: name, actual: r.(echopb.EchoResourceServiceClient)}, nil
}

type reconfigurableClient struct {
	mu     sync.RWMutex
	name   resource.Name
	actual echopb.EchoResourceServiceClient
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

type dummyEcho struct {
	mu        sync.Mutex
	capSessID uuid.UUID
}

type echoServer struct {
	echopb.UnimplementedEchoResourceServiceServer
	s subtype.Service
}

func (srv *echoServer) EchoResourceMultiple(
	req *echopb.EchoResourceMultipleRequest,
	server echopb.EchoResourceService_EchoResourceMultipleServer,
) error {
	sess, ok := session.FromContext(server.Context())
	if ok {
		resource := srv.s.Resource(req.Name).(*dummyEcho)
		resource.mu.Lock()
		resource.capSessID = sess.ID()
		resource.mu.Unlock()
	}

	session.SafetyMonitorResourceName(server.Context(), someTargetName2)
	return nil
}
